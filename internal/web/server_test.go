package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moveeeax/uptime-ping/internal/monitor"
)

type fakeMon struct{ statuses []monitor.Status }

func (f fakeMon) Snapshot() []monitor.Status { return f.statuses }

func testMon() fakeMon {
	return fakeMon{statuses: []monitor.Status{
		{Name: "web", Type: "http", Target: "http://web", State: monitor.StateUp, LastLatMS: 12.5, UptimePct: 99.5},
		{Name: "db", Type: "tcp", Target: "db:5432", State: monitor.StateDown, LastLatMS: 0, UptimePct: 40},
	}}
}

func do(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	New(testMon()).Handler().ServeHTTP(rr, req)
	return rr
}

func TestIndexServesEmbeddedUI(t *testing.T) {
	rr := do(t, "/")
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "uptime-ping") {
		t.Error("index did not render embedded UI")
	}
}

func TestUnknownPath404(t *testing.T) {
	if rr := do(t, "/nope"); rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rr.Code)
	}
}

func TestStatusAPI(t *testing.T) {
	rr := do(t, "/api/status")
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	var out struct {
		Services []monitor.Status `json:"services"`
		Summary  struct {
			Total int `json:"total"`
			Up    int `json:"up"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if out.Summary.Total != 2 || out.Summary.Up != 1 {
		t.Errorf("summary = %+v", out.Summary)
	}
}

func TestMetricsExposition(t *testing.T) {
	rr := do(t, "/metrics")
	body := rr.Body.String()
	for _, want := range []string{
		"uptime_ping_up{service=\"web\"} 1",
		"uptime_ping_up{service=\"db\"} 0",
		"uptime_ping_uptime_ratio{service=\"web\"} 0.995",
		"# TYPE uptime_ping_up gauge",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q\n---\n%s", want, body)
		}
	}
}

func TestHealthz(t *testing.T) {
	rr := do(t, "/healthz")
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "ok") {
		t.Fatalf("healthz code=%d body=%q", rr.Code, rr.Body.String())
	}
}
