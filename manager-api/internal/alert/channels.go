// Channel senders for the alerting system (SND-20): ntfy, SMTP email, and
// PagerDuty, alongside the existing generic webhook. Each is a Notifier so the
// dispatcher can fan an Event out to every configured destination.
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// Channel types.
const (
	TypeWebhook   = "webhook"
	TypeNtfy      = "ntfy"
	TypeSMTP      = "smtp"
	TypePagerDuty = "pagerduty"
)

// BuildNotifier constructs a Notifier for a channel type from its config map.
// Config keys are documented per constructor below. Unknown types error.
func BuildNotifier(typ string, cfg map[string]string, log *slog.Logger) (Notifier, error) {
	if log == nil {
		log = slog.Default()
	}
	switch typ {
	case TypeWebhook:
		if cfg["url"] == "" {
			return nil, fmt.Errorf("webhook channel needs url")
		}
		return NewWebhook(cfg["url"], log), nil
	case TypeNtfy:
		return NewNtfy(cfg, log)
	case TypeSMTP:
		return NewSMTP(cfg, log)
	case TypePagerDuty:
		return NewPagerDuty(cfg, log)
	default:
		return nil, fmt.Errorf("unknown channel type %q", typ)
	}
}

// Dispatch delivers ev to every notifier, best-effort: a failure on one channel
// is logged and does not stop the others. Returns the number delivered.
func Dispatch(ctx context.Context, log *slog.Logger, notifiers []Notifier, ev Event) int {
	if log == nil {
		log = slog.Default()
	}
	sent := 0
	for _, n := range notifiers {
		if err := n.Notify(ctx, ev); err != nil {
			log.Warn("alert channel delivery failed", "kind", ev.Kind, "error", err)
			continue
		}
		sent++
	}
	return sent
}

// --- ntfy -------------------------------------------------------------------

// NtfyNotifier posts to an ntfy topic. Config: url (server, default
// https://ntfy.sh), topic (required), token (optional bearer).
type NtfyNotifier struct {
	server string
	topic  string
	token  string
	client *http.Client
	log    *slog.Logger
}

// NewNtfy builds an ntfy notifier from config.
func NewNtfy(cfg map[string]string, log *slog.Logger) (*NtfyNotifier, error) {
	topic := strings.TrimSpace(cfg["topic"])
	if topic == "" {
		return nil, fmt.Errorf("ntfy channel needs topic")
	}
	server := strings.TrimRight(strings.TrimSpace(cfg["url"]), "/")
	if server == "" {
		server = "https://ntfy.sh"
	}
	return &NtfyNotifier{
		server: server, topic: topic, token: cfg["token"],
		client: &http.Client{Timeout: webhookTimeout}, log: log,
	}, nil
}

// Notify sends the event as an ntfy message with a title and priority derived
// from the event kind.
func (n *NtfyNotifier) Notify(ctx context.Context, ev Event) error {
	url := n.server + "/" + n.topic
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(humanText(ev)))
	if err != nil {
		return fmt.Errorf("ntfy request: %w", err)
	}
	req.Header.Set("Title", "Jabali Sounder: "+ev.ServerName)
	req.Header.Set("Tags", ntfyTag(ev.Kind))
	if ev.Kind == KindDown {
		req.Header.Set("Priority", "high")
	}
	if n.token != "" {
		req.Header.Set("Authorization", "Bearer "+n.token)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy deliver: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func ntfyTag(kind string) string {
	switch kind {
	case KindRecovered:
		return "white_check_mark"
	case KindCertExpiring:
		return "warning"
	default:
		return "rotating_light"
	}
}

// --- SMTP email -------------------------------------------------------------

// SMTPNotifier sends a plaintext email. Config: host, port, username, password,
// from, to (comma-separated). Auth is used only when username is set.
type SMTPNotifier struct {
	host, port         string
	username, password string
	from               string
	to                 []string
	log                *slog.Logger
}

// NewSMTP builds an SMTP notifier from config.
func NewSMTP(cfg map[string]string, log *slog.Logger) (*SMTPNotifier, error) {
	host := strings.TrimSpace(cfg["host"])
	from := strings.TrimSpace(cfg["from"])
	to := splitList(cfg["to"])
	if host == "" || from == "" || len(to) == 0 {
		return nil, fmt.Errorf("smtp channel needs host, from, and to")
	}
	port := strings.TrimSpace(cfg["port"])
	if port == "" {
		port = "587"
	}
	return &SMTPNotifier{
		host: host, port: port, username: cfg["username"], password: cfg["password"],
		from: from, to: to, log: log,
	}, nil
}

// Notify sends the event as an email via smtp.SendMail (STARTTLS on 587).
func (s *SMTPNotifier) Notify(_ context.Context, ev Event) error {
	subject := fmt.Sprintf("[Jabali Sounder] %s — %s", strings.ToUpper(ev.Kind), ev.ServerName)
	msg := "From: " + s.from + "\r\n" +
		"To: " + strings.Join(s.to, ", ") + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Date: " + ev.At.Format(time.RFC1123Z) + "\r\n" +
		"\r\n" + humanText(ev) + "\r\n"
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}
	addr := s.host + ":" + s.port
	if err := smtp.SendMail(addr, auth, s.from, s.to, []byte(msg)); err != nil {
		return fmt.Errorf("smtp deliver: %w", err)
	}
	return nil
}

// --- PagerDuty --------------------------------------------------------------

// PagerDutyNotifier triggers a PagerDuty Events API v2 alert. Config:
// routing_key (required).
type PagerDutyNotifier struct {
	routingKey string
	client     *http.Client
	log        *slog.Logger
}

// NewPagerDuty builds a PagerDuty notifier from config.
func NewPagerDuty(cfg map[string]string, log *slog.Logger) (*PagerDutyNotifier, error) {
	key := strings.TrimSpace(cfg["routing_key"])
	if key == "" {
		return nil, fmt.Errorf("pagerduty channel needs routing_key")
	}
	return &PagerDutyNotifier{routingKey: key, client: &http.Client{Timeout: webhookTimeout}, log: log}, nil
}

const pagerDutyURL = "https://events.pagerduty.com/v2/enqueue"

// Notify enqueues a trigger (or resolve, on recovery) keyed by server so
// PagerDuty de-duplicates and auto-resolves.
func (p *PagerDutyNotifier) Notify(ctx context.Context, ev Event) error {
	action := "trigger"
	if ev.Kind == KindRecovered {
		action = "resolve"
	}
	body := map[string]any{
		"routing_key":  p.routingKey,
		"event_action": action,
		"dedup_key":    "jabali-sounder:" + ev.ServerID,
		"payload": map[string]any{
			"summary":   humanText(ev),
			"source":    ev.ServerName,
			"severity":  pagerDutySeverity(ev.Kind),
			"timestamp": ev.At.Format(time.RFC3339),
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("pagerduty marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pagerDutyURL, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("pagerduty request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("pagerduty deliver: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("pagerduty returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func pagerDutySeverity(kind string) string {
	switch kind {
	case KindRecovered:
		return "info"
	case KindCertExpiring:
		return "warning"
	default:
		return "critical"
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
