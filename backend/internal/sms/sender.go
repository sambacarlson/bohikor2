package sms

import "context"

type Sender interface {
	SendOTP(ctx context.Context, phoneNumber string, code string) error
}
