package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

var errNotFound = errors.New("not found")

type mockQuerier struct {
	admin    *db.Admin
	adminErr error
	user     *db.User
	userErr  error
}

func (m *mockQuerier) GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error) {
	if m.adminErr != nil {
		return db.Admin{}, m.adminErr
	}
	if m.admin == nil {
		return db.Admin{}, errNotFound
	}
	return *m.admin, nil
}

func (m *mockQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	if m.userErr != nil {
		return db.User{}, m.userErr
	}
	if m.user == nil {
		return db.User{}, errNotFound
	}
	return *m.user, nil
}

func setupRoleRouter(querier Querier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("subject_id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
		c.Set("subject_type", "admin")
		c.Next()
	})
	r.Use(RequireAdmin(querier))
	r.GET("/admin-only", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func setupUserRouter(querier Querier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("subject_id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
		c.Set("subject_type", "user")
		c.Next()
	})
	r.Use(RequireActiveUser(querier))
	r.GET("/user-only", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestRequireAdmin_AdminFound(t *testing.T) {
	q := &mockQuerier{
		admin: &db.Admin{ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), Email: "admin@test.com", PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"},
	}
	r := setupRoleRouter(q)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/admin-only", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireAdmin_AdminNotFound(t *testing.T) {
	q := &mockQuerier{}
	r := setupRoleRouter(q)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/admin-only", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "admin not found" {
		t.Fatalf("expected admin not found error, got %s", resp["error"])
	}
}

func TestRequireActiveUser_ActiveUser(t *testing.T) {
	q := &mockQuerier{
		user: &db.User{
			ID:     uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
			Email:  "user@test.com",
			Status: db.UserStatusActive,
		},
	}
	r := setupUserRouter(q)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/user-only", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireActiveUser_SuspendedUser(t *testing.T) {
	q := &mockQuerier{
		user: &db.User{
			ID:     uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
			Email:  "user@test.com",
			Status: db.UserStatusSuspended,
		},
	}
	r := setupUserRouter(q)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/user-only", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["error"] != "account suspended" {
		t.Fatalf("expected account suspended error, got %s", resp["error"])
	}
}

func TestRequireActiveUser_UserNotFound(t *testing.T) {
	q := &mockQuerier{}
	r := setupUserRouter(q)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/user-only", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRequireAdmin_WrongSubjectType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &mockQuerier{
		admin: &db.Admin{ID: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), Email: "admin@test.com", PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"},
	}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("subject_id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
		c.Set("subject_type", "user")
		c.Next()
	})
	r.Use(RequireAdmin(q))
	r.GET("/admin-only", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/admin-only", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
