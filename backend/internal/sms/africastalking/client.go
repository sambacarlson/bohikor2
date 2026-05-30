package africastalking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Iknite-Space/bohikor2/internal/sms"
)

type client struct {
	apiKey   string
	username string
	senderID string
	baseURL  string
	httpDoer HTTPDoer
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Option func(*client)

func WithHTTPDoer(doer HTTPDoer) Option {
	return func(c *client) { c.httpDoer = doer }
}

func NewClient(apiKey, username, senderID, baseURL string, opts ...Option) sms.Sender {
	c := &client{
		apiKey:   apiKey,
		username: username,
		senderID: senderID,
		baseURL:  baseURL,
		httpDoer: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type atMessage struct {
	Username string `json:"username"`
	To       string `json:"to"`
	Message  string `json:"message"`
	From     string `json:"from,omitempty"`
}

type atResponse struct {
	SMSMessageData struct {
		Message string `json:"message"`
		Recipients []struct {
			StatusCode int    `json:"statusCode"`
			Number     string `json:"number"`
			Status     string `json:"status"`
		} `json:"recipients"`
	} `json:"SMSMessageData"`
}

func (c *client) SendOTP(ctx context.Context, phoneNumber string, code string) error {
	msg := atMessage{
		Username: c.username,
		To:       phoneNumber,
		Message:  fmt.Sprintf("Your Bohikor verification code is %s. It expires in 15 minutes.", code),
		From:     c.senderID,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messaging", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apiKey", c.apiKey)

	resp, err := c.httpDoer.Do(req)
	if err != nil {
		return fmt.Errorf("send sms: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		slog.Error("africas-talking sms failed", "status", resp.StatusCode, "body", string(respBody))
		return fmt.Errorf("sms send failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var atResp atResponse
	if err := json.Unmarshal(respBody, &atResp); err != nil {
		slog.Warn("could not parse africas-talking response", "error", err)
		return nil
	}

	for _, r := range atResp.SMSMessageData.Recipients {
		if r.StatusCode != 101 && r.Status != "Success" {
			slog.Warn("sms recipient issue", "number", r.Number, "status", r.Status, "code", r.StatusCode)
		}
	}

	return nil
}
