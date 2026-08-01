package server

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
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/authjwt"
	"github.com/Iknite-Space/bohikor2/internal/handler"
	"github.com/Iknite-Space/bohikor2/internal/middleware"
	"github.com/Iknite-Space/bohikor2/internal/service"
)

var errNotFound = errors.New("not found")

var testCompany = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

type testQuerier struct {
	admin    *db.Admin
	adminErr error
	user     *db.User
	userErr  error
}

func (q *testQuerier) GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error) {
	if q.adminErr != nil {
		return db.Admin{}, q.adminErr
	}
	if q.admin == nil {
		return db.Admin{}, errNotFound
	}
	return *q.admin, nil
}

func (q *testQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	if q.userErr != nil {
		return db.User{}, q.userErr
	}
	if q.user == nil {
		return db.User{}, errNotFound
	}
	return *q.user, nil
}

func (q *testQuerier) GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error) {
	return db.Company{ID: id, Status: db.CompanyStatusActive}, nil
}

func (q *testQuerier) GetPlatformAdminByID(ctx context.Context, id uuid.UUID) (db.PlatformAdmin, error) {
	return db.PlatformAdmin{}, errNotFound
}

func makeToken(svc authjwt.TokenService, subjectID, subjectType, companyID string) string {
	token, err := svc.GenerateAccessToken(subjectID, subjectType, companyID)
	if err != nil {
		panic(err)
	}
	return token
}

func TestHealthHandler_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", healthHandler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", resp["status"])
	}
}

func TestAdminMeEndpoint_NotAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, uuid.New().String(), "user", testCompany.String())
	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireAdmin(&testQuerier{}))
	r.GET("/api/admin/me", handleAdminMe(&testQuerier{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestAdminMeEndpoint_ActiveAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminID := uuid.New()
	q := &testQuerier{
		admin: &db.Admin{
			ID: adminID, CompanyID: testCompany, Email: "admin@test.com",
			PasswordHash: "$2a$10$secretbcryptvaluethatmustneverleak",
		},
	}
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, adminID.String(), "admin", testCompany.String())
	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireAdmin(q))
	r.GET("/api/admin/me", handleAdminMe(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "password_hash") || strings.Contains(w.Body.String(), q.admin.PasswordHash) {
		t.Fatalf("response leaked password hash: %s", w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", resp["data"])
	}
	if data["id"] != adminID.String() {
		t.Fatalf("expected id %s, got %v", adminID.String(), data["id"])
	}
}

func TestUserMeEndpoint_UserNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, uuid.New().String(), "user", testCompany.String())
	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireActiveUser(&testQuerier{}))
	r.GET("/api/users/me", handleUserMe(&testQuerier{}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUserMeEndpoint_SuspendedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	q := &testQuerier{
		user: &db.User{
			ID:        userID,
			CompanyID: testCompany,
			Status:    db.UserStatusSuspended,
		},
	}
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, userID.String(), "user", testCompany.String())
	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireActiveUser(q))
	r.GET("/api/users/me", handleUserMe(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestUserMeEndpoint_ActiveUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New()
	q := &testQuerier{
		user: &db.User{
			ID: userID, CompanyID: testCompany, Email: "user@test.com",
			Status: db.UserStatusActive,
		},
	}
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, userID.String(), "user", testCompany.String())
	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireActiveUser(q))
	r.GET("/api/users/me", handleUserMe(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", resp["data"])
	}
	if data["id"] != userID.String() {
		t.Fatalf("expected id %s, got %v", userID.String(), data["id"])
	}
}

type mockInviteStore struct {
	email        string
	invitation   *db.Invitation
	getErr       error
	createErr    error
	updateErr    error
	createdEmail string
}

func (m *mockInviteStore) GetInvitationByEmail(ctx context.Context, email string) (db.Invitation, error) {
	if m.getErr != nil {
		return db.Invitation{}, m.getErr
	}
	if m.invitation == nil || m.email != email {
		return db.Invitation{}, errNotFound
	}
	return *m.invitation, nil
}

func (m *mockInviteStore) CreateInvitation(ctx context.Context, email string, companyID uuid.UUID, invitedBy pgtype.UUID) (db.Invitation, error) {
	if m.createErr != nil {
		return db.Invitation{}, m.createErr
	}
	m.createdEmail = email
	if m.invitation != nil {
		return *m.invitation, nil
	}
	return db.Invitation{
		ID:        uuid.New(),
		CompanyID: companyID,
		Email:     email,
		Status:    db.InvitationStatusSent,
		SentAt:    time.Now().UTC(),
	}, nil
}

func (m *mockInviteStore) UpdateInvitationStatus(ctx context.Context, status db.InvitationStatus, id pgtype.UUID) (db.Invitation, error) {
	if m.updateErr != nil {
		return db.Invitation{}, m.updateErr
	}
	return db.Invitation{
		ID:     uuid.New(),
		Email:  m.createdEmail,
		Status: status,
		SentAt: time.Now().UTC(),
	}, nil
}

type mockEmailSender struct {
	sendErr error
}

func (m *mockEmailSender) SendInvitation(ctx context.Context, email string) error {
	return m.sendErr
}

func TestInviteEndpoint_AdminInvites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminID := uuid.New()
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, adminID.String(), "admin", testCompany.String())
	adminQuerier := &testQuerier{
		admin: &db.Admin{
			ID:        adminID,
			CompanyID: testCompany,
			Email:     "admin@example.com",
		},
	}
	inviteStore := &mockInviteStore{
		invitation: nil,
	}
	emailSender := &mockEmailSender{}
	inviteSvc := service.NewInviteService(inviteStore, emailSender, adminQuerier)

	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireAdmin(adminQuerier))
	r.POST("/api/admin/invite", handler.HandleInvite(inviteSvc))

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
}

func TestInviteEndpoint_DuplicateInvitation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminID := uuid.New()
	svc := authjwt.NewHS256Service("test-secret", 15*time.Minute)
	token := makeToken(svc, adminID.String(), "admin", testCompany.String())
	adminQuerier := &testQuerier{
		admin: &db.Admin{
			ID:        adminID,
			CompanyID: testCompany,
			Email:     "admin@example.com",
		},
	}
	inviteStore := &mockInviteStore{
		email: "existing@example.com",
		invitation: &db.Invitation{
			Email:  "existing@example.com",
			Status: db.InvitationStatusSent,
		},
	}
	inviteSvc := service.NewInviteService(inviteStore, &mockEmailSender{}, adminQuerier)

	r := gin.New()
	r.Use(middleware.JWTAuth(svc))
	r.Use(middleware.RequireAdmin(adminQuerier))
	r.POST("/api/admin/invite", handler.HandleInvite(inviteSvc))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/invite", strings.NewReader(`{"email":"existing@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}
