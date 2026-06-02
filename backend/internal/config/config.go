package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Env         string `env:"ENV" envDefault:"development"`
	Port        int    `env:"PORT" envDefault:"8080"`
	DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://localhost:5432/bohikor2?sslmode=disable"`

	// JWT Auth
	JWTSecret       string        `env:"JWT_SECRET" envDefault:"change-me-in-production-at-least-32-chars"`
	JWTAccessExpiry time.Duration `env:"JWT_ACCESS_EXPIRY" envDefault:"15m"`

	// SMS Provider: "discord" or "africastalking"
	SMSProvider string `env:"SMS_PROVIDER" envDefault:"discord"`

	// Discord SMS (OTP delivered via webhook for dev/testing)
	DiscordWebhookURL  string `env:"DISCORD_WEBHOOK_URL" envDefault:""`
	DiscordBotUsername string `env:"DISCORD_BOT_USERNAME" envDefault:"Bohikor OTP"`

	// Africa's Talking SMS
	AfricasTalkingAPIKey   string `env:"AFRICASTALKING_API_KEY" envDefault:""`
	AfricasTalkingUsername string `env:"AFRICASTALKING_USERNAME" envDefault:"sandbox"`
	AfricasTalkingSenderID string `env:"AFRICASTALKING_SENDER_ID" envDefault:""`
	AfricasTalkingBaseURL  string `env:"AFRICASTALKING_BASE_URL" envDefault:"https://api.sandbox.africastalking.com/v1"`

	// Campay
	CampayPermanentAccessToken string `env:"CAMPAY_PERMANENT_ACCESS_TOKEN" envDefault:""`
	CampayWebhookSecret        string `env:"CAMPAY_WEBHOOK_SECRET" envDefault:""`
	CampayBaseURL              string `env:"CAMPAY_BASE_URL" envDefault:"https://demo.campay.net/api"`

	// Resend
	ResendAPIKey string `env:"RESEND_API_KEY" envDefault:""`
	FromEmail    string `env:"FROM_EMAIL" envDefault:"onboarding@resend.dev"`

	// Timezone
	Timezone string `env:"TIMEZONE" envDefault:"Africa/Douala"`

	// Server timeouts
	ReadTimeout  time.Duration `env:"READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"15s"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT" envDefault:"60s"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func (c *Config) LoadLocation() (*time.Location, error) {
	return time.LoadLocation(c.Timezone)
}
