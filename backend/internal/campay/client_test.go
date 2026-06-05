package campay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/shopspring/decimal"
)

func signJWT(secret string, claims jwt.Claims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}

func TestVerifyWebhook_Valid(t *testing.T) {
	secret := "test-secret"
	claims := jwt.MapClaims{
		"source": "CamPay",
		"exp":    time.Now().Add(1 * time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"nbf":    time.Now().Unix(),
	}
	token := signJWT(secret, claims)

	client := NewClient("token-abc", "http://localhost", secret)
	if !client.VerifyWebhook(token) {
		t.Fatal("expected valid JWT to verify")
	}
}

func TestVerifyWebhook_Invalid(t *testing.T) {
	client := NewClient("token-abc", "http://localhost", "secret")
	if client.VerifyWebhook("not-a-valid-jwt") {
		t.Fatal("expected invalid JWT to fail")
	}
}

func TestVerifyWebhook_WrongSecret(t *testing.T) {
	claims := jwt.MapClaims{
		"source": "CamPay",
		"exp":    time.Now().Add(1 * time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"nbf":    time.Now().Unix(),
	}
	token := signJWT("correct-secret", claims)

	client := NewClient("token-abc", "http://localhost", "wrong-secret")
	if client.VerifyWebhook(token) {
		t.Fatal("expected JWT signed with different secret to fail")
	}
}

func TestVerifyWebhook_Expired(t *testing.T) {
	secret := "test-secret"
	claims := jwt.MapClaims{
		"source": "CamPay",
		"exp":    time.Now().Add(-1 * time.Hour).Unix(),
		"iat":    time.Now().Add(-2 * time.Hour).Unix(),
		"nbf":    time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := signJWT(secret, claims)

	client := NewClient("token-abc", "http://localhost", secret)
	if client.VerifyWebhook(token) {
		t.Fatal("expected expired JWT to fail")
	}
}

func TestVerifyWebhook_WrongSource(t *testing.T) {
	secret := "test-secret"
	claims := jwt.MapClaims{
		"source": "Other",
		"exp":    time.Now().Add(1 * time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"nbf":    time.Now().Unix(),
	}
	token := signJWT(secret, claims)

	client := NewClient("token-abc", "http://localhost", secret)
	if client.VerifyWebhook(token) {
		t.Fatal("expected JWT with wrong source to fail")
	}
}

func TestInitiateTransfer_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/withdraw/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Token perm-token-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TransferResponse{
			Reference: "campay-ref-123",
			Status:    "SUCCESSFUL",
			Amount:    10000,
			Currency:  "XAF",
			Operator:  "MTN",
		})
	}))
	defer ts.Close()

	client := NewClient("perm-token-123", ts.URL, "secret")
	resp, err := client.InitiateTransfer(t.Context(), "237600000000", decimal.NewFromInt(10000), "test transfer", "ext-ref-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Reference != "campay-ref-123" {
		t.Fatalf("expected reference campay-ref-123, got %s", resp.Reference)
	}
	if resp.Status != "SUCCESSFUL" {
		t.Fatalf("expected status SUCCESSFUL, got %s", resp.Status)
	}
}

func TestInitiateTransfer_Pending(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/withdraw/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TransferResponse{
			Reference: "campay-ref-pending",
			Status:    "PENDING",
		})
	}))
	defer ts.Close()

	client := NewClient("perm-token", ts.URL, "secret")
	resp, err := client.InitiateTransfer(t.Context(), "237600000000", decimal.NewFromInt(10000), "test", "ext-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Status != "PENDING" {
		t.Fatalf("expected status PENDING, got %s", resp.Status)
	}
}

func TestInitiateTransfer_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	}))
	defer ts.Close()

	client := NewClient("perm-token", ts.URL, "secret")
	_, err := client.InitiateTransfer(t.Context(), "237600000000", decimal.NewFromInt(10000), "test", "ext-1")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestInitiateTransfer_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Invalid token"})
	}))
	defer ts.Close()

	client := NewClient("bad-token", ts.URL, "secret")
	_, err := client.InitiateTransfer(t.Context(), "237600000000", decimal.NewFromInt(10000), "test", "ext-1")
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("perm-token", "https://demo.campay.net/api", "secret")
	if client.permanentToken != "perm-token" {
		t.Fatalf("expected permanentToken perm-token, got %s", client.permanentToken)
	}
	if client.baseURL != "https://demo.campay.net/api" {
		t.Fatalf("expected baseURL, got %s", client.baseURL)
	}
	if client.httpClient == nil {
		t.Fatal("expected httpClient to be initialized")
	}
}

func TestInitiateTransfer_CampayFailureStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/withdraw/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(TransferResponse{
			Reference: "ref-fail",
			Status:    "FAILED",
			Message:   "insufficient balance",
		})
	}))
	defer ts.Close()

	client := NewClient("perm-token", ts.URL, "secret")
	_, err := client.InitiateTransfer(t.Context(), "237600000000", decimal.NewFromInt(10000), "test", "ext-1")
	if err == nil {
		t.Fatal("expected error for failed transfer status")
	}
}

func TestInitiateCollection_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collect/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Token perm-token-123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CollectResponse{
			Reference: "campay-collect-ref-123",
			UssdCode:  "*126#",
			Operator:  "MTN",
		})
	}))
	defer ts.Close()

	client := NewClient("perm-token-123", ts.URL, "secret")
	resp, err := client.InitiateCollection(t.Context(), "237600000000", decimal.NewFromInt(5), "verification", "ext-ref-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Reference != "campay-collect-ref-123" {
		t.Fatalf("expected reference campay-collect-ref-123, got %s", resp.Reference)
	}
	if resp.UssdCode != "*126#" {
		t.Fatalf("expected ussd code *126#, got %s", resp.UssdCode)
	}
	if resp.Operator != "MTN" {
		t.Fatalf("expected operator MTN, got %s", resp.Operator)
	}
}

func TestInitiateCollection_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
	}))
	defer ts.Close()

	client := NewClient("perm-token", ts.URL, "secret")
	_, err := client.InitiateCollection(t.Context(), "237600000000", decimal.NewFromInt(5), "test", "ext-1")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestInitiateCollection_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "Invalid token"})
	}))
	defer ts.Close()

	client := NewClient("bad-token", ts.URL, "secret")
	_, err := client.InitiateCollection(t.Context(), "237600000000", decimal.NewFromInt(5), "test", "ext-1")
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestInitiateCollection_WrongEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collect/" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(CollectResponse{
				Reference: "campay-collect-ref",
				UssdCode:  "*126#",
				Operator:  "ORANGE",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient("perm-token", ts.URL, "secret")
	_, err := client.InitiateTransfer(t.Context(), "237600000000", decimal.NewFromInt(5), "test", "ext-1")
	if err == nil {
		t.Fatal("expected error because withdraw endpoint not mocked")
	}
}
