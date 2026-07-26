// Package web serves the embedded status page, a JSON API and Prometheus
// metrics for the monitor.
package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/moveeeax/uptime-ping/internal/monitor"
)

//go:embed assets/index.html
var assets embed.FS

// Snapshotter is the read side of the monitor the server renders.
type Snapshotter interface {
	Snapshot() []monitor.Status
}

// Server wires HTTP handlers to a monitor snapshot source.
type Server struct {
	mon Snapshotter
}

// New builds a Server.
func New(mon Snapshotter) *Server {
	return &Server{mon: mon}
}

// Handler returns the fully-wired HTTP handler (exported for testing).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	page, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "ui unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	statuses := s.mon.Snapshot()
	up := 0
	for _, st := range statuses {
		if st.State == monitor.StateUp {
			up++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"services": statuses,
		"summary": map[string]int{
			"total": len(statuses),
			"up":    up,
		},
	})
}

// handleMetrics emits Prometheus text-format metrics without pulling in a
// client library — the exposition format is stable and tiny.
func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	var b strings.Builder
	b.WriteString("# HELP uptime_ping_up Whether the service is up (1) or down (0).\n")
	b.WriteString("# TYPE uptime_ping_up gauge\n")
	for _, st := range s.mon.Snapshot() {
		up := 0
		if st.State == monitor.StateUp {
			up = 1
		}
		lbl := fmt.Sprintf("{service=%q}", st.Name)
		fmt.Fprintf(&b, "uptime_ping_up%s %d\n", lbl, up)
		fmt.Fprintf(&b, "uptime_ping_latency_ms%s %g\n", lbl, st.LastLatMS)
		fmt.Fprintf(&b, "uptime_ping_uptime_ratio%s %g\n", lbl, st.UptimePct/100)
	}
	_, _ = w.Write([]byte(b.String()))
}
