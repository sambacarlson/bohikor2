package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Iknite-Space/bohikor2/internal/sms"
)

type client struct {
	webhookURL string
	botName    string
	httpDoer   HTTPDoer
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Option func(*client)

func WithHTTPDoer(doer HTTPDoer) Option {
	return func(c *client) { c.httpDoer = doer }
}

func NewClient(webhookURL, botName string, opts ...Option) sms.Sender {
	c := &client{
		webhookURL: webhookURL,
		botName:    botName,
		httpDoer:   http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type discordWebhookPayload struct {
	Username string         `json:"username"`
	Embeds   []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields"`
	Timestamp   string         `json:"timestamp"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

func (c *client) SendOTP(ctx context.Context, phoneNumber string, code string) error {
	payload := discordWebhookPayload{
		Username: c.botName,
		Embeds: []discordEmbed{
			{
				Title:       "Phone OTP Verification",
				Description: fmt.Sprintf("OTP code for phone number **%s**", phoneNumber),
				Color:       0x4C4A6E,
				Fields: []discordField{
					{Name: "Phone Number", Value: phoneNumber, Inline: true},
					{Name: "OTP Code", Value: fmt.Sprintf("`%s`", code), Inline: true},
					{Name: "Expires", Value: "15 minutes", Inline: true},
				},
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpDoer.Do(req)
	if err != nil {
		return fmt.Errorf("send discord webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		slog.Error("discord webhook failed", "status", resp.StatusCode, "body", string(respBody))
		return fmt.Errorf("discord webhook failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Info("otp sent via discord", "phone", phoneNumber)
	return nil
}
