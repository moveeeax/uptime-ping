package config

import (
	"testing"
	"time"
)

func TestParseDefaults(t *testing.T) {
	c, err := Parse([]byte(`
targets:
  - name: web
    type: http
    url: https://example.com
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Listen != ":8080" {
		t.Errorf("default listen = %q, want :8080", c.Listen)
	}
	if c.History != 100 {
		t.Errorf("default history = %d, want 100", c.History)
	}
	got := c.Targets[0]
	if got.Interval != 30*time.Second {
		t.Errorf("default interval = %v, want 30s", got.Interval)
	}
	if got.Timeout != 10*time.Second {
		t.Errorf("default timeout = %v, want 10s", got.Timeout)
	}
	if got.ExpectStatus != 200 {
		t.Errorf("default expect_status = %d, want 200", got.ExpectStatus)
	}
	if got.FailureThreshold != 1 || got.SuccessThreshold != 1 {
		t.Errorf("default thresholds = %d/%d, want 1/1", got.FailureThreshold, got.SuccessThreshold)
	}
}

func TestParseDurations(t *testing.T) {
	c, err := Parse([]byte(`
targets:
  - name: db
    type: tcp
    address: localhost:5432
    interval: 15s
    timeout: 2s
    retries: 3
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tg := c.Targets[0]
	if tg.Interval != 15*time.Second {
		t.Errorf("interval = %v, want 15s", tg.Interval)
	}
	if tg.Retries != 3 {
		t.Errorf("retries = %d, want 3", tg.Retries)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := map[string]string{
		"no targets":     `targets: []`,
		"missing name":   "targets:\n  - type: http\n    url: https://x.com\n",
		"http no url":    "targets:\n  - name: a\n    type: http\n",
		"bad url scheme": "targets:\n  - name: a\n    type: http\n    url: ftp://x\n",
		"tcp no address": "targets:\n  - name: a\n    type: tcp\n",
		"tcp no port":    "targets:\n  - name: a\n    type: tcp\n    address: localhost\n",
		"unknown type":   "targets:\n  - name: a\n    type: carrier-pigeon\n",
		"dup name":       "targets:\n  - name: a\n    type: http\n    url: http://x\n  - name: a\n    type: http\n    url: http://y\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(doc)); err == nil {
				t.Errorf("expected error for %s, got nil", name)
			}
		})
	}
}

func TestParseInvalidYAML(t *testing.T) {
	if _, err := Parse([]byte("targets: [::::")); err == nil {
		t.Error("expected error for malformed yaml")
	}
}
