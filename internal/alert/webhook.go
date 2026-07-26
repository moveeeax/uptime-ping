// Package alert delivers state-change notifications to a webhook.
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/moveeeax/uptime-ping/internal/monitor"
)

// Webhook posts a JSON payload to a URL whenever a service changes state.
// The payload shape ({"text": ...}) is compatible with Slack and Discord
// incoming webhooks as well as any generic JSON receiver.
type Webhook struct {
	url    string
	client *http.Client
	log    *log.Logger
}

// NewWebhook builds a notifier. A blank url yields a no-op notifier so the
// server runs happily without alerting configured.
func NewWebhook(url string, logger *log.Logger) *Webhook {
	if logger == nil {
		logger = log.Default()
	}
	return &Webhook{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
		log:    logger,
	}
}

type payload struct {
	Text    string        `json:"text"`
	Service string        `json:"service"`
	From    monitor.State `json:"from"`
	To      monitor.State `json:"to"`
	Time    string        `json:"time"`
}

// Notify implements monitor.Notifier.
func (w *Webhook) Notify(ctx context.Context, name string, from, to monitor.State) {
	if w.url == "" {
		return
	}
	p := payload{
		Text:    fmt.Sprintf("%s: %s -> %s", name, from, to),
		Service: name,
		From:    from,
		To:      to,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := w.post(ctx, p); err != nil {
		w.log.Printf("webhook: notify %s failed: %v", name, err)
	}
}

func (w *Webhook) post(ctx context.Context, p payload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook responded %d", resp.StatusCode)
	}
	return nil
}
