package authjwt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestGenerateAndVerifyAccessToken(t *testing.T) {
	svc := NewHS256Service("test-secret-key-that-is-long-enough", 15*time.Minute)

	token, err := svc.GenerateAccessToken("user-uuid-123", "user", "")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := svc.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if claims.SubjectID != "user-uuid-123" {
		t.Errorf("SubjectID = %q, want %q", claims.SubjectID, "user-uuid-123")
	}
	if claims.SubjectType != "user" {
		t.Errorf("SubjectType = %q, want %q", claims.SubjectType, "user")
	}
	if claims.Issuer != "bohikor2" {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, "bohikor2")
	}
}

func TestVerifyAccessToken_Expired(t *testing.T) {
	svc := NewHS256Service("test-secret-key-that-is-long-enough", -1*time.Second)

	token, err := svc.GenerateAccessToken("user-uuid-123", "user", "")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	_, err = svc.VerifyAccessToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestVerifyAccessToken_WrongSecret(t *testing.T) {
	svc1 := NewHS256Service("secret-one", 15*time.Minute)
	svc2 := NewHS256Service("secret-two", 15*time.Minute)

	token, err := svc1.GenerateAccessToken("user-uuid-123", "user", "")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	_, err = svc2.VerifyAccessToken(token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestVerifyAccessToken_InvalidToken(t *testing.T) {
	svc := NewHS256Service("test-secret-key-that-is-long-enough", 15*time.Minute)

	_, err := svc.VerifyAccessToken("not-a-real-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	svc := NewHS256Service("test-secret-key-that-is-long-enough", 15*time.Minute)

	plain, hashed, err := svc.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if plain == "" {
		t.Fatal("expected non-empty plain token")
	}
	if hashed == "" {
		t.Fatal("expected non-empty hashed token")
	}
	if plain == hashed {
		t.Fatal("plain and hashed should differ")
	}

	plain2, _, _ := svc.GenerateRefreshToken()
	if plain == plain2 {
		t.Fatal("two refresh tokens should differ")
	}
}

func TestHashTokenStr_Deterministic(t *testing.T) {
	h1 := HashTokenStr("hello")
	h2 := HashTokenStr("hello")
	if h1 != h2 {
		t.Fatal("same input should produce same hash")
	}

	h3 := HashTokenStr("world")
	if h1 == h3 {
		t.Fatal("different inputs should produce different hashes")
	}
}

// A token signed with the "none" algorithm must be rejected by the HMAC-only
// verifier (defends against alg-confusion).
func TestVerifyAccessToken_UnexpectedSigningMethod(t *testing.T) {
	svc := NewHS256Service("test-secret-key-that-is-long-enough", 15*time.Minute)

	claims := TokenClaims{SubjectID: "u1", SubjectType: "user"}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none-alg token: %v", err)
	}

	if _, err := svc.VerifyAccessToken(signed); err == nil {
		t.Fatal("expected verification to reject a none-alg token")
	}
}
