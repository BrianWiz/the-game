// Package mail sends transactional email (verification and password reset) via Resend.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// Resend sends through https://resend.com's REST API.
type Resend struct {
	APIKey   string
	From     string // e.g. "The Game <noreply@chrisbox.dev>"; the domain must be verified in Resend
	Endpoint string // defaults to https://api.resend.com/emails
	HTTP     *http.Client
}

func (r *Resend) Send(ctx context.Context, m Message) error {
	endpoint := r.Endpoint
	if endpoint == "" {
		endpoint = "https://api.resend.com/emails"
	}
	client := r.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	body, err := json.Marshal(map[string]any{
		"from":    r.From,
		"to":      []string{m.To},
		"subject": m.Subject,
		"text":    m.Text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("resend: %s: %s", resp.Status, bytes.TrimSpace(detail))
	}
	return nil
}

// Log writes messages (including their links) to the log instead of sending them.
// For local development only: never use it where logs are shared.
type Log struct {
	Logger *slog.Logger
}

func (l *Log) Send(_ context.Context, m Message) error {
	l.Logger.Warn("dev mail (not sent)", "to", m.To, "subject", m.Subject, "text", m.Text)
	return nil
}
