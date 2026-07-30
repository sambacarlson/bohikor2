package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Iknite-Space/bohikor2/internal/authjwt"
)

func setupTestRouter(svc authjwt.TokenService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(JWTAuth(svc))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"subject_id": c.GetString("subject_id"), "subject_type": c.GetString("subject_type")})
	})
	return r
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	r := setupTestRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "missing authorization header" {
		t.Fatalf("expected missing auth header error, got %s", resp["error"])
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	r := setupTestRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "invalid token" {
		t.Fatalf("expected invalid token error, got %s", resp["error"])
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	svc := authjwt.NewHS256Service("test-secret", -1*time.Second)
	token, _ := svc.GenerateAccessToken("test-user-id", "user", "")
	r := setupTestRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "invalid token" {
		t.Fatalf("expected invalid token error, got %s", resp["error"])
	}
}

func TestJWTAuth_ValidToken(t *testing.T) {
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token, _ := svc.GenerateAccessToken("test-user-id", "user", "")
	r := setupTestRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["subject_id"] != "test-user-id" {
		t.Fatalf("expected subject_id test-user-id, got %s", resp["subject_id"])
	}
	if resp["subject_type"] != "user" {
		t.Fatalf("expected subject_type user, got %s", resp["subject_type"])
	}
}

func TestJWTAdmin_TokenType(t *testing.T) {
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token, _ := svc.GenerateAccessToken("test-admin-id", "admin", "")
	r := setupTestRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["subject_type"] != "admin" {
		t.Fatalf("expected subject_type admin, got %s", resp["subject_type"])
	}
}
