package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

// ginWithAdminID injects an admin_id like RequireAdmin does, without the full JWT chain.
func ginWithAdminID(adminID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("admin_id", adminID)
		c.Next()
	})
	return r
}

func TestHandleInvite_BadJSON(t *testing.T) {
	r := ginWithAdminID(uuid.New().String())
	r.POST("/invite", HandleInvite(&mockInviteQuerier{}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/invite", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleInvite_InternalError(t *testing.T) {
	r := ginWithAdminID(uuid.New().String())
	r.POST("/invite", HandleInvite(&mockInviteQuerier{err: errors.New("smtp down")}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/invite", strings.NewReader(`{"email":"a@b.com"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// companyIDFromContext: a non-uuid value in the context is an internal error.
func TestCompanyIDFromContext_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("company_id", "not-a-uuid"); c.Next() })
	r.GET("/x", HandleListInvitations(&mockInvitationsQuerier{}))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for invalid company scope, got %d", w.Code)
	}
}

// JSONError includes an optional details payload.
func TestJSONError_WithDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", func(c *gin.Context) {
		JSONError(c, http.StatusBadRequest, "bad", "bad thing", gin.H{"field": "email"})
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/x", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "field") {
		t.Fatalf("expected details in body, got %s", w.Body.String())
	}
}

// ResetPin failure branches (hash + persist).
func TestResetPin_HashError(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{user: db.User{ID: uid, Status: db.UserStatusActive}}
	h := NewPinHandler(q, configurableHasher{hashErr: errors.New("boom")})
	r := ginWithUser(uid)
	r.POST("/pin/reset", h.ResetPin)
	w := pinRequest(t, r, "POST", "/pin/reset", `{"new_pin":"33333"}`, &uid)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 hash_failed, got %d", w.Code)
	}
}

func TestResetPin_UpdateError(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{user: db.User{ID: uid, Status: db.UserStatusActive}, updateErr: errors.New("db down")}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(uid)
	r.POST("/pin/reset", h.ResetPin)
	w := pinRequest(t, r, "POST", "/pin/reset", `{"new_pin":"33333"}`, &uid)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestResetPin_UserNotFound(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{getErr: errors.New("no rows")}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(uid)
	r.POST("/pin/reset", h.ResetPin)
	w := pinRequest(t, r, "POST", "/pin/reset", `{"new_pin":"33333"}`, &uid)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// CreateRequest: an error counting daily/monthly usage must fail the request.
func TestCreateRequest_DailyCountError(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid), countTodayErr: errors.New("db down")}
	settings := &mockAdvanceSettingsQuerier{settings: []db.Setting{{Key: "daily_request_limit", Value: []byte("1")}}}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, settings, time.UTC)
	if w := runCreateRequest(h, uid, `{}`); w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCreateRequest_MonthlyCountError(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid), countMonthErr: errors.New("db down")}
	settings := &mockAdvanceSettingsQuerier{settings: []db.Setting{{Key: "monthly_request_limit", Value: []byte("3")}}}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, settings, time.UTC)
	if w := runCreateRequest(h, uid, `{}`); w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// resetOTPFailures logs but does not fail the verification when the reset errors.
func TestVerifyEmailOTP_ResetFailuresErrorStillSucceeds(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
		resetFailsErr:      errors.New("db down"),
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"a@b.com","code":"123456","purpose":"signup"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 despite reset-failures error, got %d", w.Code)
	}
}
