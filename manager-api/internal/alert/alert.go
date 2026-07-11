// Package alert delivers fleet health notifications. The poller emits an Event
// when a server crosses the healthy boundary (down / recovered); a Notifier
// delivers it. The webhook notifier posts JSON and also works for Slack /
// Discord / Mattermost incoming-webhook URLs (roadmap M1: alerting).
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Kind classifies an alert.
const (
	KindDown      = "down"
	KindRecovered = "recovered"
)

// Event is a fleet health notification.
type Event struct {
	Kind             string    `json:"kind"`
	ServerID         string    `json:"server_id"`
	ServerName       string    `json:"server_name"`
	BaseURL          string    `json:"base_url"`
	Status           string    `json:"status"`
	CredentialStatus string    `json:"credential_status"`
	Message          string    `json:"message"`
	At               time.Time `json:"at"`
}

// Notifier delivers an Event. Implementations must not block the caller for long
// and should treat delivery failure as non-fatal to the poll loop.
type Notifier interface {
	Notify(ctx context.Context, ev Event) error
}

const webhookTimeout = 10 * time.Second

// WebhookNotifier POSTs the event as JSON to a configured URL.
type WebhookNotifier struct {
	url    string
	client *http.Client
	log    *slog.Logger
}

// NewWebhook returns a webhook notifier for url. A nil log defaults to slog.
func NewWebhook(url string, log *slog.Logger) *WebhookNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &WebhookNotifier{
		url:    url,
		client: &http.Client{Timeout: webhookTimeout},
		log:    log,
	}
}

// Notify posts the event to the webhook URL. It also sends a Slack/Discord/
// Mattermost-compatible "text" field so incoming-webhook endpoints render it.
func (w *WebhookNotifier) Notify(ctx context.Context, ev Event) error {
	payload := struct {
		Event
		Text string `json:"text"`
	}{Event: ev, Text: humanText(ev)}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("alert marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("alert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("alert deliver: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("alert webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func humanText(ev Event) string {
	if ev.Kind == KindRecovered {
		return fmt.Sprintf("✅ %s recovered (status=%s, credentials=%s)", ev.ServerName, ev.Status, ev.CredentialStatus)
	}
	return fmt.Sprintf("🔴 %s is unhealthy: %s (status=%s, credentials=%s)", ev.ServerName, ev.Message, ev.Status, ev.CredentialStatus)
}
