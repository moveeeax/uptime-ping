package monitor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/moveeeax/uptime-ping/internal/config"
)

// Result is the outcome of probing a single target once.
type Result struct {
	OK      bool
	Latency time.Duration
	Status  int    // HTTP status code, 0 for TCP
	Err     string // human-readable failure reason, empty when OK
}

// Prober performs a single network probe against a target. All real network
// I/O lives behind this interface so the scheduling, threshold and reporting
// logic can be tested without touching the network.
type Prober interface {
	Probe(ctx context.Context, t config.Target) Result
}

// NetProber is the production Prober using net/http and net.Dialer.
type NetProber struct {
	client *http.Client
}

// NewNetProber builds a Prober backed by the standard library.
func NewNetProber() *NetProber {
	return &NetProber{
		client: &http.Client{
			// Never follow redirects silently: a 301/302 is a distinct state.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Probe dispatches to the correct transport for the target type.
func (p *NetProber) Probe(ctx context.Context, t config.Target) Result {
	switch t.Type {
	case config.KindTCP:
		return p.probeTCP(ctx, t)
	default:
		return p.probeHTTP(ctx, t)
	}
}

func (p *NetProber) probeHTTP(ctx context.Context, t config.Target) Result {
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return Result{Err: fmt.Sprintf("build request: %v", err)}
	}
	req.Header.Set("User-Agent", "uptime-ping/1.0")

	resp, err := p.client.Do(req)
	lat := time.Since(start)
	if err != nil {
		return Result{Latency: lat, Err: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != t.ExpectStatus {
		return Result{
			Latency: lat,
			Status:  resp.StatusCode,
			Err:     fmt.Sprintf("got status %d, want %d", resp.StatusCode, t.ExpectStatus),
		}
	}
	return Result{OK: true, Latency: lat, Status: resp.StatusCode}
}

func (p *NetProber) probeTCP(ctx context.Context, t config.Target) Result {
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", t.Address)
	lat := time.Since(start)
	if err != nil {
		return Result{Latency: lat, Err: err.Error()}
	}
	_ = conn.Close()
	return Result{OK: true, Latency: lat}
}
