package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ginWithRawUser sets an arbitrary (possibly wrong-typed) user_id value, to
// exercise the "invalid user ID" type-assertion guard in the user handlers.
func ginWithRawUser(val any) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", val); c.Next() })
	return r
}

func expectStatus(t *testing.T, r *gin.Engine, method, path, body string, want int) {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != want {
		t.Fatalf("%s %s: expected %d, got %d (%s)", method, path, want, w.Code, w.Body.String())
	}
}

// The user_id in context is not a uuid.UUID -> 500 across all user handlers.
func TestUserHandlers_InvalidUserIDType(t *testing.T) {
	badType := "not-a-uuid-value"

	t.Run("CreateRequest", func(t *testing.T) {
		h := NewAdvanceHandler(&mockAdvanceQuerier{}, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
		r := ginWithRawUser(badType)
		r.POST("/x", h.CreateRequest)
		expectStatus(t, r, "POST", "/x", `{}`, http.StatusInternalServerError)
	})
	t.Run("GetEligibility", func(t *testing.T) {
		h := NewAdvanceHandler(&mockAdvanceQuerier{}, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
		r := ginWithRawUser(badType)
		r.GET("/x", h.GetEligibility)
		expectStatus(t, r, "GET", "/x", ``, http.StatusInternalServerError)
	})
	t.Run("ListUserRequests", func(t *testing.T) {
		h := NewAdvanceHandler(&mockAdvanceQuerier{}, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
		r := ginWithRawUser(badType)
		r.GET("/x", h.ListUserRequests)
		expectStatus(t, r, "GET", "/x", ``, http.StatusInternalServerError)
	})
	t.Run("AddPhoneNumber", func(t *testing.T) {
		h := newPhoneHandler(&mockPhoneQuerier{}, &mockCampayTransferer{})
		r := ginWithRawUser(badType)
		r.POST("/x", h.AddPhoneNumber)
		expectStatus(t, r, "POST", "/x", `{"phone_number":"237600000000"}`, http.StatusInternalServerError)
	})
	t.Run("GetPhoneVerificationStatus", func(t *testing.T) {
		h := newPhoneHandler(&mockPhoneQuerier{}, &mockCampayTransferer{})
		r := ginWithRawUser(badType)
		r.GET("/x", h.GetPhoneVerificationStatus)
		expectStatus(t, r, "GET", "/x", ``, http.StatusInternalServerError)
	})
	t.Run("ChangePin", func(t *testing.T) {
		h := NewPinHandler(&mockPinQuerier{}, configurableHasher{})
		r := ginWithRawUser(badType)
		r.POST("/x", h.ChangePin)
		expectStatus(t, r, "POST", "/x", `{"current_pin":"11111","new_pin":"22222"}`, http.StatusInternalServerError)
	})
	t.Run("ResetPin", func(t *testing.T) {
		h := NewPinHandler(&mockPinQuerier{}, configurableHasher{})
		r := ginWithRawUser(badType)
		r.POST("/x", h.ResetPin)
		expectStatus(t, r, "POST", "/x", `{"new_pin":"22222"}`, http.StatusInternalServerError)
	})
	t.Run("HandleAcceptTerms", func(t *testing.T) {
		r := ginWithRawUser(badType)
		r.PUT("/x", HandleAcceptTerms(&mockUserTermsQuerier{}))
		expectStatus(t, r, "PUT", "/x", `{"version":"1.0"}`, http.StatusInternalServerError)
	})
}

// Request-body validation (400) branches on the auth endpoints.
func TestAuthEndpoints_BadRequestBodies(t *testing.T) {
	h := newFlexAuthHandler(&flexAuthQuerier{}, &trackingEmailSender{})

	cases := []struct {
		name    string
		method  string
		handler gin.HandlerFunc
		body    string
	}{
		{"ForgotPin bad email", "POST", h.ForgotPin, `{"email":"nope"}`},
		{"SendEmailOTP bad email", "POST", h.SendEmailOTP, `{"email":"nope"}`},
		{"VerifyEmailOTP missing code", "POST", h.VerifyEmailOTP, `{"email":"a@b.com"}`},
		{"AdminLogin missing password", "POST", h.AdminLogin, `{"email":"a@b.com"}`},
		{"PlatformLogin missing password", "POST", h.PlatformLogin, `{"email":"a@b.com"}`},
		{"RefreshToken missing token", "POST", h.RefreshToken, `{}`},
		{"CreatePin missing fields", "POST", h.CreatePin, `{"email":"a@b.com"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Handle(tc.method, "/x", tc.handler)
			expectStatus(t, r, tc.method, "/x", tc.body, http.StatusBadRequest)
		})
	}
}

func TestPlatformLogin_NotFound(t *testing.T) {
	h := newFlexAuthHandler(&flexAuthQuerier{}, &trackingEmailSender{}) // getPlatformAdmin -> not found
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", h.PlatformLogin)
	expectStatus(t, r, "POST", "/x", `{"email":"root@p.io","password":"secret"}`, http.StatusUnauthorized)
}

// AddPhoneNumber: the amount is derived from the configured verification amount;
// an unauthenticated request (no user_id) is a 401 (separate from the bad-type case).
func TestAddPhoneNumber_UnauthWithUUIDHelper(t *testing.T) {
	h := newPhoneHandler(&mockPhoneQuerier{}, &mockCampayTransferer{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", h.AddPhoneNumber)
	expectStatus(t, r, "POST", "/x", `{"phone_number":"237600000000"}`, http.StatusUnauthorized)
}

var _ = uuid.New
