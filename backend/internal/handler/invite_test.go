package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/authjwt"
	"github.com/Iknite-Space/bohikor2/internal/middleware"
	"github.com/Iknite-Space/bohikor2/internal/service"
)

type mockInviteQuerier struct {
	result *service.InviteResult
	err    error
}

func (m *mockInviteQuerier) Invite(ctx context.Context, email string, invitedBy string) (*service.InviteResult, error) {
	return m.result, m.err
}

type mockAdminQuerier struct {
	admin    *db.Admin
	adminErr error
}

func (m *mockAdminQuerier) GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error) {
	if m.adminErr != nil {
		return db.Admin{}, m.adminErr
	}
	if m.admin == nil {
		return db.Admin{}, errors.New("not found")
	}
	return *m.admin, nil
}

func (m *mockAdminQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return db.User{}, errors.New("not found")
}

func (m *mockAdminQuerier) GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error) {
	return db.Company{ID: id, Status: db.CompanyStatusActive}, nil
}

func (m *mockAdminQuerier) GetPlatformAdminByID(ctx context.Context, id uuid.UUID) (db.PlatformAdmin, error) {
	return db.PlatformAdmin{}, errors.New("not found")
}

func makeToken(svc authjwt.TokenService, subjectID, subjectType, companyID string) string {
	token, err := svc.GenerateAccessToken(subjectID, subjectType, companyID)
	if err != nil {
		panic(err)
	}
	return token
}

func TestHandleInvite_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.POST("/api/admin/invite", HandleInvite(&mockInviteQuerier{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/invite", strings.NewReader(`{"email":"test@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleInvite_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, uuid.New().String(), "user", testCompanyID.String())
	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireAdmin(&mockAdminQuerier{}))
	r.POST("/api/admin/invite", HandleInvite(&mockInviteQuerier{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/invite", strings.NewReader(`{"email":"test@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestHandleInvite_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminID := uuid.New()
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, adminID.String(), "admin", testCompanyID.String())
	adminQuerier := &mockAdminQuerier{
		admin: &db.Admin{
			ID:           adminID,
			CompanyID:    testCompanyID,
			Email:        "admin@example.com",
			PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		},
	}
	inviteQuerier := &mockInviteQuerier{
		result: &service.InviteResult{
			Invitation: db.Invitation{
				ID:     uuid.New(),
				Email:  "newadmin@example.com",
				Status: db.InvitationStatusSent,
				SentAt: time.Now().UTC(),
			},
		},
	}

	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireAdmin(adminQuerier))
	r.POST("/api/admin/invite", HandleInvite(inviteQuerier))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/invite", strings.NewReader(`{"email":"newadmin@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", resp)
	}
	if data["email"] != "newadmin@example.com" {
		t.Fatalf("expected email newadmin@example.com, got %v", data["email"])
	}
	if data["status"] != "sent" {
		t.Fatalf("expected status sent, got %v", data["status"])
	}
}

func TestHandleInvite_DuplicateInvitation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminID := uuid.New()
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, adminID.String(), "admin", testCompanyID.String())
	adminQuerier := &mockAdminQuerier{
		admin: &db.Admin{
			ID:           adminID,
			CompanyID:    testCompanyID,
			Email:        "admin@example.com",
			PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		},
	}
	inviteQuerier := &mockInviteQuerier{
		err: service.ErrActiveInvitationExists,
	}

	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireAdmin(adminQuerier))
	r.POST("/api/admin/invite", HandleInvite(inviteQuerier))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/invite", strings.NewReader(`{"email":"existing@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestHandleInvite_BadRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminID := uuid.New()
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, adminID.String(), "admin", testCompanyID.String())
	adminQuerier := &mockAdminQuerier{
		admin: &db.Admin{
			ID:           adminID,
			CompanyID:    testCompanyID,
			Email:        "admin@example.com",
			PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		},
	}

	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireAdmin(adminQuerier))
	r.POST("/api/admin/invite", HandleInvite(&mockInviteQuerier{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/invite", strings.NewReader(`{"email":"not-an-email"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleInvite_MissingEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminID := uuid.New()
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, adminID.String(), "admin", testCompanyID.String())
	adminQuerier := &mockAdminQuerier{
		admin: &db.Admin{
			ID:           adminID,
			CompanyID:    testCompanyID,
			Email:        "admin@example.com",
			PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		},
	}

	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireAdmin(adminQuerier))
	r.POST("/api/admin/invite", HandleInvite(&mockInviteQuerier{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/invite", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
