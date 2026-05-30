package africastalking

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendOTP_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("apiKey") != "test-api-key" {
			t.Errorf("apiKey header = %q, want %q", r.Header.Get("apiKey"), "test-api-key")
		}

		body, _ := io.ReadAll(r.Body)
		var msg atMessage
		_ = json.Unmarshal(body, &msg)
		if msg.Username != "sandbox" {
			t.Errorf("username = %q, want %q", msg.Username, "sandbox")
		}
		if msg.To != "+237612345678" {
			t.Errorf("to = %q, want %q", msg.To, "+237612345678")
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"SMSMessageData":{"message":"Sent","recipients":[{"statusCode":101,"number":"+237612345678","status":"Success"}]}}`))
	}))
	defer server.Close()

	c := NewClient("test-api-key", "sandbox", "Bohikor", server.URL, WithHTTPDoer(server.Client()))

	err := c.SendOTP(context.Background(), "+237612345678", "123456")
	if err != nil {
		t.Fatalf("SendOTP: %v", err)
	}
}

func TestSendOTP_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	c := NewClient("test-api-key", "sandbox", "Bohikor", server.URL, WithHTTPDoer(server.Client()))

	err := c.SendOTP(context.Background(), "+237612345678", "123456")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
