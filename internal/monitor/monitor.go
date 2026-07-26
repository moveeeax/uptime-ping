package monitor

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/moveeeax/uptime-ping/internal/config"
)

// Sample is a single recorded probe outcome.
type Sample struct {
	Time    time.Time     `json:"time"`
	OK      bool          `json:"ok"`
	Latency time.Duration `json:"-"`
	Err     string        `json:"err,omitempty"`
}

// MarshalJSON emits latency as milliseconds so the wire field matches its name.
func (s Sample) MarshalJSON() ([]byte, error) {
	type alias struct {
		Time      time.Time `json:"time"`
		OK        bool      `json:"ok"`
		LatencyMS float64   `json:"latency_ms"`
		Err       string    `json:"err,omitempty"`
	}
	return json.Marshal(alias{
		Time:      s.Time,
		OK:        s.OK,
		LatencyMS: float64(s.Latency) / float64(time.Millisecond),
		Err:       s.Err,
	})
}

// Status is the public snapshot of one monitored service.
type Status struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Target     string   `json:"target"`
	State      State    `json:"state"`
	LastError  string   `json:"last_error,omitempty"`
	LastLatMS  float64  `json:"last_latency_ms"`
	UptimePct  float64  `json:"uptime_pct"`
	LastCheckT string   `json:"last_check,omitempty"`
	History    []Sample `json:"history"`
}

// check holds the runtime state for one target.
type check struct {
	target config.Target
	sm     *stateMachine

	mu      sync.Mutex
	history []Sample
	maxHist int
	last    Sample
	checked bool
}

func newCheck(t config.Target, maxHist int) *check {
	return &check{
		target:  t,
		sm:      newStateMachine(t.FailureThreshold, t.SuccessThreshold),
		maxHist: maxHist,
	}
}

// apply records a sample and folds it into the state machine atomically,
// returning the previous state and whether the reported state changed.
func (c *check) apply(s Sample) (prev State, changed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = append(c.history, s)
	if len(c.history) > c.maxHist {
		c.history = c.history[len(c.history)-c.maxHist:]
	}
	c.last = s
	c.checked = true
	prev = c.sm.state
	changed = c.sm.observe(s.OK)
	return prev, changed
}

// currentState returns the reported state under lock.
func (c *check) currentState() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sm.state
}

func (c *check) snapshot() Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	tgt := c.target.URL
	if c.target.Type == config.KindTCP {
		tgt = c.target.Address
	}
	st := Status{
		Name:      c.target.Name,
		Type:      string(c.target.Type),
		Target:    tgt,
		State:     c.sm.state,
		History:   append([]Sample(nil), c.history...),
		LastLatMS: float64(c.last.Latency) / float64(time.Millisecond),
	}
	if c.checked {
		st.LastError = c.last.Err
		st.LastCheckT = c.last.Time.UTC().Format(time.RFC3339)
	}
	if n := len(c.history); n > 0 {
		ok := 0
		for _, s := range c.history {
			if s.OK {
				ok++
			}
		}
		st.UptimePct = 100 * float64(ok) / float64(n)
	}
	return st
}

// Notifier receives state-change events.
type Notifier interface {
	Notify(ctx context.Context, name string, from, to State)
}

// Monitor schedules probes for every configured target and keeps their
// rolling history and state.
type Monitor struct {
	prober   Prober
	notifier Notifier
	checks   []*check
	byName   map[string]*check
}

// New builds a Monitor from configuration.
func New(cfg *config.Config, prober Prober, notifier Notifier) *Monitor {
	m := &Monitor{
		prober:   prober,
		notifier: notifier,
		byName:   map[string]*check{},
	}
	for _, t := range cfg.Targets {
		c := newCheck(t, cfg.History)
		m.checks = append(m.checks, c)
		m.byName[t.Name] = c
	}
	return m
}

// runCheck probes a single target once, honouring the per-cycle retry budget,
// updates state and fires the notifier on a transition. It performs exactly
// one cycle and is the seam the tests drive directly.
func (m *Monitor) runCheck(ctx context.Context, c *check) Sample {
	var res Result
	attempts := c.target.Retries + 1
	for i := 0; i < attempts; i++ {
		res = m.prober.Probe(ctx, c.target)
		if res.OK {
			break
		}
	}

	s := Sample{Time: time.Now(), OK: res.OK, Latency: res.Latency, Err: res.Err}
	prev, changed := c.apply(s)
	if changed && m.notifier != nil {
		m.notifier.Notify(ctx, c.target.Name, prev, c.currentState())
	}
	return s
}

// RunOnce probes every target exactly once (concurrently) and returns true if
// every service is up. Used by the one-shot CLI mode and by tests.
func (m *Monitor) RunOnce(ctx context.Context) bool {
	var wg sync.WaitGroup
	for _, c := range m.checks {
		wg.Add(1)
		go func(c *check) {
			defer wg.Done()
			m.runCheck(ctx, c)
		}(c)
	}
	wg.Wait()
	for _, c := range m.checks {
		if c.currentState() == StateDown {
			return false
		}
	}
	return true
}

// Run starts a scheduling goroutine per target and blocks until ctx is done.
func (m *Monitor) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, c := range m.checks {
		wg.Add(1)
		go func(c *check) {
			defer wg.Done()
			m.runCheck(ctx, c) // probe immediately on start
			t := time.NewTicker(c.target.Interval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					m.runCheck(ctx, c)
				}
			}
		}(c)
	}
	wg.Wait()
}

// Snapshot returns the current status of every service, sorted by name.
func (m *Monitor) Snapshot() []Status {
	out := make([]Status, 0, len(m.checks))
	for _, c := range m.checks {
		out = append(out, c.snapshot())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
