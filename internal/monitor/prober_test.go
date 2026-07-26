package monitor

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moveeeax/uptime-ping/internal/config"
)

// These exercise the real NetProber but only ever hit a local httptest.Server
// or a local listener — never an external host.

func TestNetProberHTTPUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := NewNetProber()
	res := p.Probe(context.Background(), config.Target{
		Type: config.KindHTTP, URL: srv.URL, Timeout: 2 * time.Second, ExpectStatus: 200,
	})
	if !res.OK {
		t.Fatalf("expected OK, got err=%q", res.Err)
	}
	if res.Status != 200 {
		t.Errorf("status = %d, want 200", res.Status)
	}
}

func TestNetProberHTTPWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewNetProber()
	res := p.Probe(context.Background(), config.Target{
		Type: config.KindHTTP, URL: srv.URL, Timeout: 2 * time.Second, ExpectStatus: 200,
	})
	if res.OK {
		t.Fatal("expected failure on 500")
	}
	if res.Status != 500 {
		t.Errorf("status = %d, want 500", res.Status)
	}
}

func TestNetProberHTTPConnError(t *testing.T) {
	p := NewNetProber()
	// Reserve then release a port so nothing is listening.
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	_ = l.Close()

	res := p.Probe(context.Background(), config.Target{
		Type: config.KindHTTP, URL: "http://" + addr, Timeout: time.Second, ExpectStatus: 200,
	})
	if res.OK {
		t.Fatal("expected connection failure")
	}
}

func TestNetProberTCP(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	p := NewNetProber()
	res := p.Probe(context.Background(), config.Target{
		Type: config.KindTCP, Address: l.Addr().String(), Timeout: 2 * time.Second,
	})
	if !res.OK {
		t.Fatalf("expected TCP OK, got err=%q", res.Err)
	}
}

func TestNetProberTCPRefused(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	_ = l.Close()

	p := NewNetProber()
	res := p.Probe(context.Background(), config.Target{
		Type: config.KindTCP, Address: addr, Timeout: time.Second,
	})
	if res.OK {
		t.Fatal("expected TCP failure to closed port")
	}
}
