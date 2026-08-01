package campay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/shopspring/decimal"
)

type Client struct {
	permanentToken string
	baseURL        string
	webhookSecret  string
	httpClient     *http.Client
}

type TransferRequest struct {
	Amount            decimal.Decimal `json:"amount"`
	To                string          `json:"to"`
	Description       string          `json:"description"`
	ExternalReference string          `json:"external_reference"`
}

type TransferResponse struct {
	Reference         string  `json:"reference"`
	Status            string  `json:"status"`
	Message           string  `json:"message,omitempty"`
	Amount            float64 `json:"amount,omitempty"`
	Currency          string  `json:"currency,omitempty"`
	Operator          string  `json:"operator,omitempty"`
	Code              string  `json:"code,omitempty"`
	OperatorReference string  `json:"operator_reference,omitempty"`
}

type CollectRequest struct {
	Amount            decimal.Decimal `json:"amount"`
	Currency          string          `json:"currency"`
	From              string          `json:"from"`
	Description       string          `json:"description"`
	ExternalReference string          `json:"external_reference"`
}

type CollectResponse struct {
	Reference string `json:"reference"`
	UssdCode  string `json:"ussd_code"`
	Operator  string `json:"operator"`
}

type WebhookPayload struct {
	Reference         string `json:"reference"`
	Status            string `json:"status"`
	Amount            string `json:"amount"`
	Currency          string `json:"currency"`
	Operator          string `json:"operator"`
	Code              string `json:"code"`
	OperatorReference string `json:"operator_reference"`
	Endpoint          string `json:"endpoint"`
	Signature         string `json:"signature"`
	ExternalReference string `json:"external_reference"`
	PhoneNumber       string `json:"phone_number"`
	Description       string `json:"description"`
	Reason            string `json:"reason"`
}

type TransactionStatusResponse struct {
	Reference         string `json:"reference"`
	Status            string `json:"status"`
	Amount            string `json:"amount,omitempty"`
	Currency          string `json:"currency,omitempty"`
	Operator          string `json:"operator,omitempty"`
	Code              string `json:"code,omitempty"`
	OperatorReference string `json:"operator_reference,omitempty"`
	ExternalReference string `json:"external_reference,omitempty"`
	Reason            string `json:"reason,omitempty"`
}

type campayClaims struct {
	jwt.StandardClaims
	Source string `json:"source"`
}

func NewClient(permanentToken, baseURL, webhookSecret string) *Client {
	return &Client{
		permanentToken: permanentToken,
		baseURL:        baseURL,
		webhookSecret:  webhookSecret,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) InitiateTransfer(ctx context.Context, phoneNumber string, amount decimal.Decimal, description string, externalRef string) (*TransferResponse, error) {
	transferReq := TransferRequest{
		Amount:            amount,
		To:                phoneNumber,
		Description:       description,
		ExternalReference: externalRef,
	}
	body, err := json.Marshal(transferReq)
	if err != nil {
		return nil, fmt.Errorf("marshal transfer request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/withdraw/", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create transfer request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+c.permanentToken)

	slog.Info("campay transfer request",
		"phone", phoneNumber,
		"amount", amount.String(),
		"ref", externalRef,
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transfer request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read transfer response: %w", err)
	}

	slog.Info("campay transfer response",
		"status", resp.StatusCode,
		"body", string(respBody),
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("transfer failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var tr TransferResponse
	if err := json.Unmarshal(respBody, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal transfer response: %w", err)
	}

	if tr.Status == "FAILED" {
		return &tr, fmt.Errorf("transfer failed: %s", tr.Message)
	}

	return &tr, nil
}

func (c *Client) InitiateCollection(ctx context.Context, phoneNumber string, amount decimal.Decimal, description string, externalRef string) (*CollectResponse, error) {
	collectReq := CollectRequest{
		Amount:            amount,
		Currency:          "XAF",
		From:              phoneNumber,
		Description:       description,
		ExternalReference: externalRef,
	}
	body, err := json.Marshal(collectReq)
	if err != nil {
		return nil, fmt.Errorf("marshal collect request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/collect/", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create collect request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+c.permanentToken)

	slog.Info("campay collect request",
		"phone", phoneNumber,
		"amount", amount.String(),
		"ref", externalRef,
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collect request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read collect response: %w", err)
	}

	slog.Info("campay collect response",
		"status", resp.StatusCode,
		"body", string(respBody),
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("collect failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var cr CollectResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return nil, fmt.Errorf("unmarshal collect response: %w", err)
	}

	if cr.Reference == "" {
		return nil, fmt.Errorf("collect returned empty reference: body=%s", string(respBody))
	}

	return &cr, nil
}

// GetTransactionStatus polls GET /transaction/{reference}/. reference must be
// Campay's own reference (the value returned in TransferResponse/CollectResponse),
// never our external_reference — confirmed against Campay's docs and official SDK.
func (c *Client) GetTransactionStatus(ctx context.Context, reference string) (*TransactionStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/transaction/"+reference+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("create status request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.permanentToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("status request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read status response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status request failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var sr TransactionStatusResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return nil, fmt.Errorf("unmarshal status response: %w", err)
	}
	return &sr, nil
}

func (c *Client) VerifyWebhook(token string) bool {
	claims := &campayClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(c.webhookSecret), nil
	})
	if err != nil {
		slog.Warn("webhook JWT verification failed", "error", err)
		return false
	}
	if claims.Source != "CamPay" {
		slog.Warn("webhook invalid source claim", "source", claims.Source)
		return false
	}
	return true
}
