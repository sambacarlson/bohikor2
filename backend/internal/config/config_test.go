package config

import "testing"

func TestLoad_FrontendBaseURL_DefaultsToLocalhostOutsideProduction(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("FRONTEND_BASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.FrontendBaseURL != "http://localhost:3000" {
		t.Fatalf("FrontendBaseURL = %q, want http://localhost:3000", cfg.FrontendBaseURL)
	}
}

func TestLoad_FrontendBaseURL_RequiredInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("FRONTEND_BASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected an error when FRONTEND_BASE_URL is unset in production, got nil")
	}
}

func TestLoad_FrontendBaseURL_HonorsExplicitValueInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("FRONTEND_BASE_URL", "https://app.bohikor.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.FrontendBaseURL != "https://app.bohikor.com" {
		t.Fatalf("FrontendBaseURL = %q, want https://app.bohikor.com", cfg.FrontendBaseURL)
	}
}
