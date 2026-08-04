package email

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/resend/resend-go/v2"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

type Client struct {
	apiKey    string
	fromEmail string
	sdk       *resend.Client
}

func NewClient(apiKey, fromEmail string) *Client {
	return &Client{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		sdk:       resend.NewClient(apiKey),
	}
}

func (c *Client) SendInvitation(ctx context.Context, email, signupURL string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format: %s", email)
	}
	if signupURL == "" {
		return fmt.Errorf("signup URL is required")
	}

	params := &resend.SendEmailRequest{
		From:    c.fromEmail,
		To:      []string{email},
		Subject: "You've been invited to Bohikor",
		Text:    fmt.Sprintf("You have been invited to join Bohikor. Sign up using your work email to get started:\n\n%s", signupURL),
	}

	_, err := c.sdk.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("send invitation email via Resend: %w", err)
	}

	return nil
}

func (c *Client) SendOTP(ctx context.Context, email, code string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format: %s", email)
	}
	if code == "" {
		return fmt.Errorf("OTP code is required")
	}
	if len(code) != 6 {
		return fmt.Errorf("OTP code must be 6 digits, got %d", len(code))
	}
	if _, err := strconv.Atoi(code); err != nil {
		return fmt.Errorf("OTP code must be numeric: %s", code)
	}

	params := &resend.SendEmailRequest{
		From:    c.fromEmail,
		To:      []string{email},
		Subject: "Your Bohikor2 verification code",
		Text:    fmt.Sprintf("Your verification code is: %s\n\nThis code expires in 10 minutes.", code),
	}

	_, err := c.sdk.Emails.SendWithContext(ctx, params)
	if err != nil {
		return fmt.Errorf("send email via Resend: %w", err)
	}

	return nil
}
