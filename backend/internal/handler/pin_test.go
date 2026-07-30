package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

// configurableHasher lets each test choose Verify/Hash behaviour.
type configurableHasher struct {
	verifyOK bool
	hashErr  error
}

func (h configurableHasher) Hash(plain string) (string, error) {
	if h.hashErr != nil {
		return "", h.hashErr
	}
	return "hashed:" + plain, nil
}
func (h configurableHasher) Verify(hash, plain string) bool { return h.verifyOK }

type mockPinQuerier struct {
	user          db.User
	getErr        error
	updateErr     error
	resetErr      error
	updateCalled  bool
	lastUpdateArg db.UpdateUserPinHashParams
}

func (m *mockPinQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return m.user, m.getErr
}
func (m *mockPinQuerier) UpdateUserPinHash(ctx context.Context, arg db.UpdateUserPinHashParams) (db.User, error) {
	m.updateCalled = true
	m.lastUpdateArg = arg
	return m.user, m.updateErr
}
func (m *mockPinQuerier) ResetLoginAttempts(ctx context.Context, id uuid.UUID) (db.User, error) {
	return m.user, m.resetErr
}

func pinRequest(t *testing.T, r *gin.Engine, method, path, body string, userID *uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// ginWithUser injects a user_id like RequireActiveUser does.
func ginWithUser(userID any) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if userID != nil {
		r.Use(func(c *gin.Context) {
			c.Set("user_id", userID)
			c.Next()
		})
	}
	return r
}

func TestChangePin_Success(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{user: db.User{ID: uid, PinHash: pgtype.Text{String: "oldhash", Valid: true}}}
	h := NewPinHandler(q, configurableHasher{verifyOK: true})
	r := ginWithUser(uid)
	r.POST("/pin", h.ChangePin)

	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"11111","new_pin":"22222"}`, &uid)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !q.updateCalled || q.lastUpdateArg.PinHash.String != "hashed:22222" {
		t.Fatalf("expected new PIN hashed and persisted, got %+v", q.lastUpdateArg)
	}
}

func TestChangePin_Unauthenticated(t *testing.T) {
	q := &mockPinQuerier{}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(nil)
	r.POST("/pin", h.ChangePin)
	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"11111","new_pin":"22222"}`, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestChangePin_BadUserIDType(t *testing.T) {
	q := &mockPinQuerier{}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser("not-a-uuid-type") // wrong type in context
	r.POST("/pin", h.ChangePin)
	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"1","new_pin":"2"}`, nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestChangePin_MissingFields(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(uid)
	r.POST("/pin", h.ChangePin)
	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"11111"}`, &uid)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChangePin_WrongLength(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(uid)
	r.POST("/pin", h.ChangePin)
	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"11111","new_pin":"123"}`, &uid)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short PIN, got %d", w.Code)
	}
}

func TestChangePin_UserNotFound(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{getErr: errors.New("no rows")}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(uid)
	r.POST("/pin", h.ChangePin)
	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"11111","new_pin":"22222"}`, &uid)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestChangePin_NoPinSet(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{user: db.User{ID: uid, PinHash: pgtype.Text{Valid: false}}}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(uid)
	r.POST("/pin", h.ChangePin)
	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"11111","new_pin":"22222"}`, &uid)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 no_pin_set, got %d", w.Code)
	}
}

func TestChangePin_WrongCurrentPin(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{user: db.User{ID: uid, PinHash: pgtype.Text{String: "oldhash", Valid: true}}}
	h := NewPinHandler(q, configurableHasher{verifyOK: false})
	r := ginWithUser(uid)
	r.POST("/pin", h.ChangePin)
	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"00000","new_pin":"22222"}`, &uid)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong current PIN, got %d", w.Code)
	}
}

func TestChangePin_HashError(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{user: db.User{ID: uid, PinHash: pgtype.Text{String: "oldhash", Valid: true}}}
	h := NewPinHandler(q, configurableHasher{verifyOK: true, hashErr: errors.New("boom")})
	r := ginWithUser(uid)
	r.POST("/pin", h.ChangePin)
	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"11111","new_pin":"22222"}`, &uid)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestChangePin_UpdateError(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{
		user:      db.User{ID: uid, PinHash: pgtype.Text{String: "oldhash", Valid: true}},
		updateErr: errors.New("boom"),
	}
	h := NewPinHandler(q, configurableHasher{verifyOK: true})
	r := ginWithUser(uid)
	r.POST("/pin", h.ChangePin)
	w := pinRequest(t, r, "POST", "/pin", `{"current_pin":"11111","new_pin":"22222"}`, &uid)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestResetPin_Success(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{user: db.User{ID: uid, Status: db.UserStatusActive}}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(uid)
	r.POST("/pin/reset", h.ResetPin)
	w := pinRequest(t, r, "POST", "/pin/reset", `{"new_pin":"33333"}`, &uid)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if q.lastUpdateArg.PinHash.String != "hashed:33333" {
		t.Fatalf("expected new PIN persisted, got %s", q.lastUpdateArg.PinHash.String)
	}
}

func TestResetPin_Locked(t *testing.T) {
	uid := uuid.New()
	q := &mockPinQuerier{user: db.User{ID: uid, Status: db.UserStatusLocked}}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(uid)
	r.POST("/pin/reset", h.ResetPin)
	w := pinRequest(t, r, "POST", "/pin/reset", `{"new_pin":"33333"}`, &uid)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for locked account, got %d", w.Code)
	}
	if q.updateCalled {
		t.Fatalf("locked account must not update PIN")
	}
}

func TestResetPin_Validation(t *testing.T) {
	uid := uuid.New()
	cases := []struct {
		name string
		body string
		want int
	}{
		{"missing new_pin", `{}`, http.StatusBadRequest},
		{"short pin", `{"new_pin":"12"}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &mockPinQuerier{user: db.User{ID: uid, Status: db.UserStatusActive}}
			h := NewPinHandler(q, configurableHasher{})
			r := ginWithUser(uid)
			r.POST("/pin/reset", h.ResetPin)
			w := pinRequest(t, r, "POST", "/pin/reset", tc.body, &uid)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, w.Code)
			}
		})
	}
}

func TestResetPin_ResetLoginAttemptsErrorStillSucceeds(t *testing.T) {
	// A failure to clear the login-attempt counter is logged but must not
	// fail the PIN reset itself.
	uid := uuid.New()
	q := &mockPinQuerier{user: db.User{ID: uid, Status: db.UserStatusActive}, resetErr: errors.New("boom")}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(uid)
	r.POST("/pin/reset", h.ResetPin)
	w := pinRequest(t, r, "POST", "/pin/reset", `{"new_pin":"33333"}`, &uid)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 despite reset-attempts error, got %d", w.Code)
	}
}

func TestResetPin_Unauthenticated(t *testing.T) {
	q := &mockPinQuerier{}
	h := NewPinHandler(q, configurableHasher{})
	r := ginWithUser(nil)
	r.POST("/pin/reset", h.ResetPin)
	w := pinRequest(t, r, "POST", "/pin/reset", `{"new_pin":"33333"}`, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
