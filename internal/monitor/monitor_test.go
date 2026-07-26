package monitor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/moveeeax/uptime-ping/internal/config"
)

// scriptProber returns queued results per target name, cycling the last one.
type scriptProber struct {
	mu      sync.Mutex
	scripts map[string][]Result
	calls   map[string]int
}

func newScriptProber() *scriptProber {
	return &scriptProber{scripts: map[string][]Result{}, calls: map[string]int{}}
}

func (p *scriptProber) Probe(_ context.Context, t config.Target) Result {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls[t.Name]++
	seq := p.scripts[t.Name]
	if len(seq) == 0 {
		return Result{OK: true, Latency: time.Millisecond}
	}
	i := p.calls[t.Name] - 1
	if i >= len(seq) {
		i = len(seq) - 1
	}
	return seq[i]
}

type recordingNotifier struct {
	mu     sync.Mutex
	events []string
}

func (n *recordingNotifier) Notify(_ context.Context, name string, from, to State) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, name+":"+string(from)+"->"+string(to))
}

func cfgWith(targets ...config.Target) *config.Config {
	return &config.Config{History: 10, Targets: targets}
}

func TestRunOnceAllUp(t *testing.T) {
	p := newScriptProber()
	cfg := cfgWith(
		config.Target{Name: "a", Type: config.KindHTTP, URL: "http://a", FailureThreshold: 1, SuccessThreshold: 1},
		config.Target{Name: "b", Type: config.KindTCP, Address: "b:1", FailureThreshold: 1, SuccessThreshold: 1},
	)
	m := New(cfg, p, nil)
	if !m.RunOnce(context.Background()) {
		t.Fatal("RunOnce should report all up")
	}
	snap := m.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(snap))
	}
	for _, s := range snap {
		if s.State != StateUp {
			t.Errorf("%s state = %s, want up", s.Name, s.State)
		}
		if s.UptimePct != 100 {
			t.Errorf("%s uptime = %g, want 100", s.Name, s.UptimePct)
		}
	}
}

func TestRunOnceReportsDown(t *testing.T) {
	p := newScriptProber()
	p.scripts["db"] = []Result{{OK: false, Err: "refused"}}
	cfg := cfgWith(config.Target{Name: "db", Type: config.KindTCP, Address: "x:1", FailureThreshold: 1, SuccessThreshold: 1})
	m := New(cfg, p, nil)
	if m.RunOnce(context.Background()) {
		t.Fatal("RunOnce should report not-all-up")
	}
	if s := m.Snapshot()[0]; s.State != StateDown || s.LastError != "refused" {
		t.Fatalf("got state=%s err=%q", s.State, s.LastError)
	}
}

func TestRetriesConsumeBudgetBeforeFailing(t *testing.T) {
	p := newScriptProber()
	// First two probes fail, third succeeds; with retries=2 the cycle passes.
	p.scripts["flaky"] = []Result{{OK: false, Err: "e"}, {OK: false, Err: "e"}, {OK: true, Latency: 2 * time.Millisecond}}
	cfg := cfgWith(config.Target{Name: "flaky", Type: config.KindHTTP, URL: "http://f", Retries: 2, FailureThreshold: 1, SuccessThreshold: 1})
	m := New(cfg, p, nil)
	if !m.RunOnce(context.Background()) {
		t.Fatal("retry budget should have recovered the check")
	}
	if p.calls["flaky"] != 3 {
		t.Fatalf("prober called %d times, want 3", p.calls["flaky"])
	}
}

func TestNotifierFiresOnTransitionOnly(t *testing.T) {
	p := newScriptProber()
	n := &recordingNotifier{}
	tgt := config.Target{Name: "svc", Type: config.KindHTTP, URL: "http://s", FailureThreshold: 1, SuccessThreshold: 1}
	cfg := cfgWith(tgt)
	m := New(cfg, p, n)
	c := m.byName["svc"]

	seq := []bool{true, true, false, false, true}
	for _, ok := range seq {
		p.mu.Lock()
		p.scripts["svc"] = []Result{{OK: ok}}
		p.calls["svc"] = 0
		p.mu.Unlock()
		m.runCheck(context.Background(), c)
	}

	want := []string{"svc:up->down", "svc:down->up"}
	if len(n.events) != len(want) {
		t.Fatalf("events = %v, want %v", n.events, want)
	}
	for i := range want {
		if n.events[i] != want[i] {
			t.Errorf("event[%d] = %s, want %s", i, n.events[i], want[i])
		}
	}
}

func TestHistoryWindowTrims(t *testing.T) {
	p := newScriptProber()
	cfg := &config.Config{History: 3, Targets: []config.Target{{Name: "a", Type: config.KindHTTP, URL: "http://a", FailureThreshold: 1, SuccessThreshold: 1}}}
	m := New(cfg, p, nil)
	c := m.byName["a"]
	for i := 0; i < 10; i++ {
		m.runCheck(context.Background(), c)
	}
	if got := len(m.Snapshot()[0].History); got != 3 {
		t.Fatalf("history len = %d, want 3 (trimmed)", got)
	}
}

func TestUptimePercentage(t *testing.T) {
	p := newScriptProber()
	cfg := cfgWith(config.Target{Name: "a", Type: config.KindHTTP, URL: "http://a", FailureThreshold: 1, SuccessThreshold: 1})
	m := New(cfg, p, nil)
	c := m.byName["a"]
	results := []bool{true, true, true, false} // 3/4 = 75%
	for _, ok := range results {
		p.mu.Lock()
		p.scripts["a"] = []Result{{OK: ok}}
		p.calls["a"] = 0
		p.mu.Unlock()
		m.runCheck(context.Background(), c)
	}
	if got := m.Snapshot()[0].UptimePct; got != 75 {
		t.Fatalf("uptime = %g, want 75", got)
	}
}
