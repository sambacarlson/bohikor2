package discord

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendOTP_Success(t *testing.T) {
	var gotPayload discordWebhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q, want application/json", r.Header.Get("Content-Type"))
		}

		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotPayload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := NewClient(server.URL, "Bohikor OTP", WithHTTPDoer(server.Client()))

	err := c.SendOTP(context.Background(), "+237612345678", "123456")
	if err != nil {
		t.Fatalf("SendOTP: %v", err)
	}

	if gotPayload.Username != "Bohikor OTP" {
		t.Errorf("username = %q, want %q", gotPayload.Username, "Bohikor OTP")
	}
	if len(gotPayload.Embeds) != 1 {
		t.Fatalf("embeds count = %d, want 1", len(gotPayload.Embeds))
	}
	embed := gotPayload.Embeds[0]
	foundPhone := false
	foundCode := false
	for _, f := range embed.Fields {
		if f.Name == "Phone Number" && f.Value == "+237612345678" {
			foundPhone = true
		}
		if f.Name == "OTP Code" && f.Value == "`123456`" {
			foundCode = true
		}
	}
	if !foundPhone {
		t.Error("expected Phone Number field with +237612345678")
	}
	if !foundCode {
		t.Error("expected OTP Code field with `123456`")
	}
}

func TestSendOTP_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "Internal Server Error"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "Bohikor OTP", WithHTTPDoer(server.Client()))

	err := c.SendOTP(context.Background(), "+237612345678", "123456")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}