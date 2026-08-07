package handler

import (
	"context"
	"encoding/json"
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
)

// mockAuthQuerier implements authQuerier. Only the fields the tested flow reads
// are configured; everything else returns zero values.
type mockAuthQuerier struct {
	user       *db.User
	userErr    error
	company    *db.Company
	companyErr error
}

func (m *mockAuthQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	if m.user == nil {
		return db.User{}, errTestNotFound
	}
	return *m.user, nil
}
func (m *mockAuthQuerier) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	if m.userErr != nil {
		return db.User{}, m.userErr
	}
	if m.user == nil {
		return db.User{}, errTestNotFound
	}
	return *m.user, nil
}
func (m *mockAuthQuerier) GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error) {
	return db.Admin{}, errTestNotFound
}
func (m *mockAuthQuerier) GetAdminByEmail(ctx context.Context, email string) (db.Admin, error) {
	return db.Admin{}, errTestNotFound
}
func (m *mockAuthQuerier) GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error) {
	if m.companyErr != nil {
		return db.Company{}, m.companyErr
	}
	if m.company == nil {
		return db.Company{ID: id, Slug: "acme", Status: db.CompanyStatusActive}, nil
	}
	return *m.company, nil
}
func (m *mockAuthQuerier) GetPlatformAdminByEmail(ctx context.Context, email string) (db.PlatformAdmin, error) {
	return db.PlatformAdmin{}, errTestNotFound
}
func (m *mockAuthQuerier) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	return db.User{}, nil
}
func (m *mockAuthQuerier) CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error) {
	return db.RefreshToken{}, nil
}
func (m *mockAuthQuerier) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error) {
	return db.RefreshToken{}, errTestNotFound
}
func (m *mockAuthQuerier) RevokeRefreshToken(ctx context.Context, tokenHash string) error { return nil }
func (m *mockAuthQuerier) RevokeAllRefreshTokensForSubject(ctx context.Context, arg db.RevokeAllRefreshTokensForSubjectParams) error {
	return nil
}
func (m *mockAuthQuerier) GetActiveInvitationByEmail(ctx context.Context, email string) (db.Invitation, error) {
	return db.Invitation{}, errTestNotFound
}
func (m *mockAuthQuerier) CreateEmailOTP(ctx context.Context, arg db.CreateEmailOTPParams) (db.EmailOtp, error) {
	return db.EmailOtp{}, nil
}
func (m *mockAuthQuerier) GetEmailOTPByEmail(ctx context.Context, email string) (db.EmailOtp, error) {
	return db.EmailOtp{}, errTestNotFound
}
func (m *mockAuthQuerier) DeleteEmailOTP(ctx context.Context, email string) error { return nil }
func (m *mockAuthQuerier) AcceptInvitation(ctx context.Context, email string) (db.Invitation, error) {
	return db.Invitation{}, nil
}
func (m *mockAuthQuerier) IncrementFailedLoginAttempts(ctx context.Context, id uuid.UUID) (db.User, error) {
	if m.user == nil {
		return db.User{}, errTestNotFound
	}
	m.user.FailedLoginAttempts++
	return *m.user, nil
}
func (m *mockAuthQuerier) ResetLoginAttempts(ctx context.Context, id uuid.UUID) (db.User, error) {
	if m.user == nil {
		return db.User{}, errTestNotFound
	}
	return *m.user, nil
}
func (m *mockAuthQuerier) LockUserUntil(ctx context.Context, arg db.LockUserUntilParams) (db.User, error) {
	return db.User{}, nil
}
func (m *mockAuthQuerier) LockUser(ctx context.Context, id uuid.UUID) (db.User, error) {
	return db.User{}, nil
}
func (m *mockAuthQuerier) UpdateUserPinHash(ctx context.Context, arg db.UpdateUserPinHashParams) (db.User, error) {
	return db.User{}, nil
}
func (m *mockAuthQuerier) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	return db.Event{}, nil
}
func (m *mockAuthQuerier) GetEmailOTPFailure(ctx context.Context, email string) (db.EmailOtpFailure, error) {
	return db.EmailOtpFailure{}, errTestNotFound
}
func (m *mockAuthQuerier) UpsertEmailOTPFailure(ctx context.Context, arg db.UpsertEmailOTPFailureParams) (db.EmailOtpFailure, error) {
	return db.EmailOtpFailure{}, nil
}
func (m *mockAuthQuerier) ResetEmailOTPFailures(ctx context.Context, email string) error { return nil }

type stubEmailSender struct{}

func (stubEmailSender) SendOTP(ctx context.Context, to, code string) error { return nil }

func newTestAuthHandler(q authQuerier) *AuthHandler {
	svc := authjwt.NewHS256Service("test-secret-key-that-is-long-enough", 15*time.Minute)
	return NewAuthHandler(q, svc, stubHasher{}, stubEmailSender{}, 30*24*time.Hour)
}

func postLogin(h *AuthHandler, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/login", h.Login)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// Login resolves the user's company from their (globally-unique) email and
// returns company_slug so the web app can redirect to /{slug}.
func TestLogin_ResolvesCompanySlug(t *testing.T) {
	companyID := uuid.New()
	q := &mockAuthQuerier{
		user: &db.User{
			ID:        uuid.New(),
			CompanyID: companyID,
			Email:     "worker@acme.com",
			Status:    db.UserStatusActive,
			PinHash:   pgtype.Text{String: "hashed:12345", Valid: true},
		},
		company: &db.Company{ID: companyID, Slug: "acme-corp", Status: db.CompanyStatusActive},
	}
	h := newTestAuthHandler(q)

	w := postLogin(h, `{"email":"worker@acme.com","pin":"12345","company_slug":"acme-corp"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["data"]["company_slug"] != "acme-corp" {
		t.Fatalf("expected company_slug acme-corp, got %v", resp["data"]["company_slug"])
	}
	if resp["data"]["access_token"] == "" || resp["data"]["access_token"] == nil {
		t.Fatal("expected an access token in the response")
	}
}

// A user whose company is suspended cannot log in.
func TestLogin_SuspendedCompanyBlocks(t *testing.T) {
	companyID := uuid.New()
	q := &mockAuthQuerier{
		user: &db.User{
			ID:        uuid.New(),
			CompanyID: companyID,
			Email:     "worker@acme.com",
			Status:    db.UserStatusActive,
			PinHash:   pgtype.Text{String: "hashed:12345", Valid: true},
		},
		company: &db.Company{ID: companyID, Slug: "acme-corp", Status: db.CompanyStatusSuspended},
	}
	h := newTestAuthHandler(q)

	w := postLogin(h, `{"email":"worker@acme.com","pin":"12345","company_slug":"acme-corp"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"] != "company_suspended" {
		t.Fatalf("expected code company_suspended, got %v", resp["code"])
	}
}

func TestLogin_WrongPin(t *testing.T) {
	q := &mockAuthQuerier{
		user: &db.User{
			ID:      uuid.New(),
			Email:   "worker@acme.com",
			Status:  db.UserStatusActive,
			PinHash: pgtype.Text{String: "hashed:12345", Valid: true},
		},
	}
	h := newTestAuthHandler(q)

	w := postLogin(h, `{"email":"worker@acme.com","pin":"00000","company_slug":"acme"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// A correct email+PIN for the wrong company slug must be rejected exactly
// like a wrong PIN — the {company} slug in the login URL was previously
// never checked server-side, so any slug would resolve the real account.
func TestLogin_WrongCompanySlug(t *testing.T) {
	companyID := uuid.New()
	q := &mockAuthQuerier{
		user: &db.User{
			ID:        uuid.New(),
			CompanyID: companyID,
			Email:     "worker@acme.com",
			Status:    db.UserStatusActive,
			PinHash:   pgtype.Text{String: "hashed:12345", Valid: true},
		},
		company: &db.Company{ID: companyID, Slug: "acme-corp", Status: db.CompanyStatusActive},
	}
	h := newTestAuthHandler(q)

	w := postLogin(h, `{"email":"worker@acme.com","pin":"12345","company_slug":"someone-elses-company"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	if codeOf(t, w.Body.Bytes()) != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials code, got %s", w.Body.String())
	}
}
