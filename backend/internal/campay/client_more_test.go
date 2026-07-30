package campay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
)

func TestInitiateTransfer_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer ts.Close()
	client := NewClient("perm-token", ts.URL, "secret")
	if _, err := client.InitiateTransfer(context.Background(), "237600000000", decimal.NewFromInt(1000), "d", "ref"); err == nil {
		t.Fatal("expected an unmarshal error on malformed transfer response")
	}
}

func TestInitiateCollection_EmptyReference(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"reference":"","ussd_code":"*126#"}`))
	}))
	defer ts.Close()
	client := NewClient("perm-token", ts.URL, "secret")
	if _, err := client.InitiateCollection(context.Background(), "237600000000", decimal.NewFromInt(100), "verify", "ref"); err == nil {
		t.Fatal("expected error when collect returns an empty reference")
	}
}

func TestInitiateCollection_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer ts.Close()
	client := NewClient("perm-token", ts.URL, "secret")
	if _, err := client.InitiateCollection(context.Background(), "237600000000", decimal.NewFromInt(100), "verify", "ref"); err == nil {
		t.Fatal("expected an unmarshal error on malformed collect response")
	}
}

// A request against an unreachable base URL exercises the transport error path.
func TestInitiate_TransportError(t *testing.T) {
	client := NewClient("perm-token", "http://127.0.0.1:1", "secret")
	if _, err := client.InitiateTransfer(context.Background(), "237600000000", decimal.NewFromInt(100), "d", "ref"); err == nil {
		t.Fatal("expected a transport error for transfer")
	}
	if _, err := client.InitiateCollection(context.Background(), "237600000000", decimal.NewFromInt(100), "d", "ref"); err == nil {
		t.Fatal("expected a transport error for collect")
	}
}

func TestVerifyWebhook_MalformedToken(t *testing.T) {
	client := NewClient("token-abc", "http://localhost", "secret")
	if client.VerifyWebhook("this-is-not-a-jwt") {
		t.Fatal("a malformed token must not verify")
	}
}
