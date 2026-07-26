package alert

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/moveeeax/uptime-ping/internal/monitor"
)

func TestWebhookNotifyPostsPayload(t *testing.T) {
	var (
		mu   sync.Mutex
		body []byte
		hits int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		body = b
		hits++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := NewWebhook(srv.URL, log.New(io.Discard, "", 0))
	wh.Notify(context.Background(), "api", monitor.StateUp, monitor.StateDown)

	mu.Lock()
	defer mu.Unlock()
	if hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if p.Service != "api" || p.From != monitor.StateUp || p.To != monitor.StateDown {
		t.Errorf("payload = %+v", p)
	}
	if p.Text != "api: up -> down" {
		t.Errorf("text = %q", p.Text)
	}
}

func TestWebhookEmptyURLIsNoop(t *testing.T) {
	wh := NewWebhook("", log.New(io.Discard, "", 0))
	// Must not panic or dial anything.
	wh.Notify(context.Background(), "x", monitor.StateUp, monitor.StateDown)
}
