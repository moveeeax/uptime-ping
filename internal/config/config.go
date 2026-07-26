// Package config loads and validates the uptime-ping YAML configuration.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Kind is the probe type for a target.
type Kind string

const (
	// KindHTTP performs an HTTP(S) GET and checks the status code.
	KindHTTP Kind = "http"
	// KindTCP dials a TCP address and checks that the connection succeeds.
	KindTCP Kind = "tcp"
)

// Target describes a single service to monitor.
type Target struct {
	Name     string        `yaml:"name"`
	Type     Kind          `yaml:"type"`
	URL      string        `yaml:"url"`      // for http
	Address  string        `yaml:"address"`  // for tcp (host:port)
	Interval time.Duration `yaml:"interval"` // poll cadence
	Timeout  time.Duration `yaml:"timeout"`  // per-probe deadline
	Retries  int           `yaml:"retries"`  // immediate retries within one cycle

	// ExpectStatus is the HTTP status code considered healthy (default 200).
	ExpectStatus int `yaml:"expect_status"`

	// FailureThreshold is the number of consecutive failing cycles required
	// to flip a service to DOWN. SuccessThreshold flips it back to UP.
	FailureThreshold int `yaml:"failure_threshold"`
	SuccessThreshold int `yaml:"success_threshold"`
}

// Webhook configures state-change notifications.
type Webhook struct {
	URL string `yaml:"url"`
}

// Config is the top-level configuration document.
type Config struct {
	Listen  string   `yaml:"listen"`
	History int      `yaml:"history"` // samples kept per target
	Targets []Target `yaml:"targets"`
	Webhook Webhook  `yaml:"webhook"`
}

// Load reads, parses and validates a configuration file.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Parse(raw)
}

// Parse decodes and validates configuration from YAML bytes.
func Parse(raw []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.History <= 0 {
		c.History = 100
	}
	for i := range c.Targets {
		t := &c.Targets[i]
		if t.Type == "" {
			t.Type = KindHTTP
		}
		t.Type = Kind(strings.ToLower(string(t.Type)))
		if t.Interval <= 0 {
			t.Interval = 30 * time.Second
		}
		if t.Timeout <= 0 {
			t.Timeout = 10 * time.Second
		}
		if t.Retries < 0 {
			t.Retries = 0
		}
		if t.ExpectStatus == 0 {
			t.ExpectStatus = 200
		}
		if t.FailureThreshold <= 0 {
			t.FailureThreshold = 1
		}
		if t.SuccessThreshold <= 0 {
			t.SuccessThreshold = 1
		}
	}
}

// Validate returns an error if the configuration is not usable.
func (c *Config) Validate() error {
	if len(c.Targets) == 0 {
		return fmt.Errorf("config: at least one target is required")
	}
	seen := map[string]bool{}
	for i, t := range c.Targets {
		if t.Name == "" {
			return fmt.Errorf("config: target #%d is missing a name", i+1)
		}
		if seen[t.Name] {
			return fmt.Errorf("config: duplicate target name %q", t.Name)
		}
		seen[t.Name] = true
		switch t.Type {
		case KindHTTP:
			if t.URL == "" {
				return fmt.Errorf("config: target %q (http) requires url", t.Name)
			}
			if !strings.HasPrefix(t.URL, "http://") && !strings.HasPrefix(t.URL, "https://") {
				return fmt.Errorf("config: target %q url must start with http:// or https://", t.Name)
			}
		case KindTCP:
			if t.Address == "" {
				return fmt.Errorf("config: target %q (tcp) requires address", t.Name)
			}
			if !strings.Contains(t.Address, ":") {
				return fmt.Errorf("config: target %q address must be host:port", t.Name)
			}
		default:
			return fmt.Errorf("config: target %q has unknown type %q", t.Name, t.Type)
		}
	}
	return nil
}
