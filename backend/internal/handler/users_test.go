package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type mockUsersQuerier struct {
	listUsers    []db.User
	listErr      error
	lastListArg  db.ListUsersByCompanyParams
	unlockUser   db.User
	unlockErr    error
	resetOTPErr  error
	resetOTPMail string
	statusUser   db.User
	statusErr    error
	lastStatus   db.UpdateUserStatusParams
}

func (m *mockUsersQuerier) ListUsersByCompany(ctx context.Context, arg db.ListUsersByCompanyParams) ([]db.User, error) {
	m.lastListArg = arg
	return m.listUsers, m.listErr
}

func (m *mockUsersQuerier) UnlockUser(ctx context.Context, arg db.UnlockUserParams) (db.User, error) {
	return m.unlockUser, m.unlockErr
}

func (m *mockUsersQuerier) UpdateUserStatus(ctx context.Context, arg db.UpdateUserStatusParams) (db.User, error) {
	m.lastStatus = arg
	return m.statusUser, m.statusErr
}

func (m *mockUsersQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return db.User{}, nil
}

func (m *mockUsersQuerier) ResetEmailOTPFailures(ctx context.Context, email string) error {
	m.resetOTPMail = email
	return m.resetOTPErr
}

// makeTestGinNoScope builds an engine WITHOUT the company-scope middleware,
// to exercise the "missing company scope" guard paths.
func makeTestGinNoScope() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestHandleListUsers_Empty(t *testing.T) {
	q := &mockUsersQuerier{listUsers: nil}
	r := makeTestGin()
	r.GET("/api/admin/users", HandleListUsers(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/users", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 0 {
		t.Fatalf("expected empty array, got %d", len(data))
	}
}

func TestHandleListUsers_DefaultPagination(t *testing.T) {
	q := &mockUsersQuerier{listUsers: []db.User{{ID: uuid.New(), Email: "a@x.com"}}}
	r := makeTestGin()
	r.GET("/api/admin/users", HandleListUsers(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/users", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Defaults: page 1, per_page 20 -> limit 20, offset 0.
	if q.lastListArg.Limit != 20 || q.lastListArg.Offset != 0 {
		t.Fatalf("expected limit 20 offset 0, got limit %d offset %d", q.lastListArg.Limit, q.lastListArg.Offset)
	}
	if q.lastListArg.CompanyID != testCompanyID {
		t.Fatalf("expected company scope %v, got %v", testCompanyID, q.lastListArg.CompanyID)
	}
}

func TestHandleListUsers_Pagination(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantLimit  int32
		wantOffset int32
	}{
		{"page 3 per_page 10", "?page=3&per_page=10", 10, 20},
		{"invalid page falls back to 1", "?page=abc", 20, 0},
		{"zero page falls back to 1", "?page=0", 20, 0},
		{"per_page over 100 falls back to 20", "?per_page=500", 20, 0},
		{"negative per_page falls back to 20", "?per_page=-5", 20, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &mockUsersQuerier{}
			r := makeTestGin()
			r.GET("/api/admin/users", HandleListUsers(q))
			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/users"+tc.query, nil)
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if q.lastListArg.Limit != tc.wantLimit || q.lastListArg.Offset != tc.wantOffset {
				t.Fatalf("limit/offset = %d/%d, want %d/%d", q.lastListArg.Limit, q.lastListArg.Offset, tc.wantLimit, tc.wantOffset)
			}
		})
	}
}

func TestHandleListUsers_DBError(t *testing.T) {
	q := &mockUsersQuerier{listErr: errors.New("boom")}
	r := makeTestGin()
	r.GET("/api/admin/users", HandleListUsers(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/users", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleListUsers_MissingScope(t *testing.T) {
	q := &mockUsersQuerier{}
	r := makeTestGinNoScope()
	r.GET("/api/admin/users", HandleListUsers(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/users", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for missing scope, got %d", w.Code)
	}
}

func TestHandleUnlockUser_Success(t *testing.T) {
	target := db.User{ID: uuid.New(), Email: "locked@x.com"}
	q := &mockUsersQuerier{unlockUser: target}
	r := makeTestGin()
	r.POST("/api/admin/users/:id/unlock", HandleUnlockUser(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/users/"+target.ID.String()+"/unlock", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Unlock must also clear OTP failures for the unlocked user's email.
	if q.resetOTPMail != target.Email {
		t.Fatalf("expected OTP reset for %q, got %q", target.Email, q.resetOTPMail)
	}
}

func TestHandleUnlockUser_InvalidID(t *testing.T) {
	q := &mockUsersQuerier{}
	r := makeTestGin()
	r.POST("/api/admin/users/:id/unlock", HandleUnlockUser(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/users/not-a-uuid/unlock", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUnlockUser_NotFound(t *testing.T) {
	q := &mockUsersQuerier{unlockErr: errors.New("no rows")}
	r := makeTestGin()
	r.POST("/api/admin/users/:id/unlock", HandleUnlockUser(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/users/"+uuid.New().String()+"/unlock", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleUnlockUser_ResetOTPError(t *testing.T) {
	q := &mockUsersQuerier{unlockUser: db.User{ID: uuid.New(), Email: "x@x.com"}, resetOTPErr: errors.New("boom")}
	r := makeTestGin()
	r.POST("/api/admin/users/:id/unlock", HandleUnlockUser(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/users/"+uuid.New().String()+"/unlock", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleSuspendAndActivateUser(t *testing.T) {
	cases := []struct {
		name       string
		handler    func(usersQuerier) gin.HandlerFunc
		wantStatus db.UserStatus
	}{
		{"suspend", HandleSuspendUser, db.UserStatusSuspended},
		{"activate", HandleActivateUser, db.UserStatusActive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			q := &mockUsersQuerier{statusUser: db.User{ID: id}}
			r := makeTestGin()
			r.POST("/u/:id", tc.handler(q))
			w := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), "POST", "/u/"+id.String(), nil)
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if q.lastStatus.Status != tc.wantStatus {
				t.Fatalf("expected status %v, got %v", tc.wantStatus, q.lastStatus.Status)
			}
			if q.lastStatus.CompanyID != testCompanyID {
				t.Fatalf("expected company scope on status update")
			}
		})
	}
}

func TestUpdateUserStatus_InvalidIDAndNotFound(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		q := &mockUsersQuerier{}
		r := makeTestGin()
		r.POST("/u/:id", HandleSuspendUser(q))
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/u/bad", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
	t.Run("not found", func(t *testing.T) {
		q := &mockUsersQuerier{statusErr: errors.New("no rows")}
		r := makeTestGin()
		r.POST("/u/:id", HandleActivateUser(q))
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/u/"+uuid.New().String(), nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}
