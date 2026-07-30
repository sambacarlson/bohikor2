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

var (
	testSubjectID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	testCompanyID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	otherCompany  = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
)

type mockQuerier struct {
	admin      *db.Admin
	adminErr   error
	user       *db.User
	userErr    error
	company    *db.Company
	companyErr error
	platform   *db.PlatformAdmin
	platformE  error
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

func (m *mockQuerier) GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error) {
	if m.companyErr != nil {
		return db.Company{}, m.companyErr
	}
	if m.company == nil {
		return db.Company{ID: id, Status: db.CompanyStatusActive}, nil
	}
	return *m.company, nil
}

func (m *mockQuerier) GetPlatformAdminByID(ctx context.Context, id uuid.UUID) (db.PlatformAdmin, error) {
	if m.platformE != nil {
		return db.PlatformAdmin{}, m.platformE
	}
	if m.platform == nil {
		return db.PlatformAdmin{}, errNotFound
	}
	return *m.platform, nil
}

// claimInjector emulates JWTAuth putting the token claims into the context.
func claimInjector(subjectType, claimCompanyID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("subject_id", testSubjectID.String())
		c.Set("subject_type", subjectType)
		c.Set("claim_company_id", claimCompanyID)
		c.Next()
	}
}

func setupRoleRouter(querier Querier, claimCompanyID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(claimInjector("admin", claimCompanyID))
	r.Use(RequireAdmin(querier))
	r.GET("/admin-only", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func setupUserRouter(querier Querier, claimCompanyID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(claimInjector("user", claimCompanyID))
	r.Use(RequireActiveUser(querier))
	r.GET("/user-only", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func activeAdmin() *db.Admin {
	return &db.Admin{ID: testSubjectID, CompanyID: testCompanyID, Email: "admin@test.com"}
}

func activeUser() *db.User {
	return &db.User{ID: testSubjectID, CompanyID: testCompanyID, Email: "user@test.com", Status: db.UserStatusActive}
}

func doGet(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestRequireAdmin_AdminFound(t *testing.T) {
	q := &mockQuerier{admin: activeAdmin()}
	w := doGet(setupRoleRouter(q, testCompanyID.String()), "/admin-only")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireAdmin_AdminNotFound(t *testing.T) {
	q := &mockQuerier{}
	w := doGet(setupRoleRouter(q, testCompanyID.String()), "/admin-only")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "admin not found" {
		t.Fatalf("expected admin not found error, got %s", resp["error"])
	}
}

// A token minted for company B must not grant access to an admin record in company A.
func TestRequireAdmin_CompanyMismatch(t *testing.T) {
	q := &mockQuerier{admin: activeAdmin()}
	w := doGet(setupRoleRouter(q, otherCompany.String()), "/admin-only")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "company mismatch" {
		t.Fatalf("expected company mismatch, got %s", resp["error"])
	}
}

func TestRequireAdmin_SuspendedCompany(t *testing.T) {
	q := &mockQuerier{
		admin:   activeAdmin(),
		company: &db.Company{ID: testCompanyID, Status: db.CompanyStatusSuspended},
	}
	w := doGet(setupRoleRouter(q, testCompanyID.String()), "/admin-only")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "company suspended" {
		t.Fatalf("expected company suspended, got %s", resp["error"])
	}
}

func TestRequireActiveUser_ActiveUser(t *testing.T) {
	q := &mockQuerier{user: activeUser()}
	w := doGet(setupUserRouter(q, testCompanyID.String()), "/user-only")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequireActiveUser_SuspendedUser(t *testing.T) {
	u := activeUser()
	u.Status = db.UserStatusSuspended
	q := &mockQuerier{user: u}
	w := doGet(setupUserRouter(q, testCompanyID.String()), "/user-only")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "account suspended" {
		t.Fatalf("expected account suspended error, got %s", resp["error"])
	}
}

// A user cannot read another company's scope even with a valid session.
func TestRequireActiveUser_CompanyMismatch(t *testing.T) {
	q := &mockQuerier{user: activeUser()}
	w := doGet(setupUserRouter(q, otherCompany.String()), "/user-only")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "company mismatch" {
		t.Fatalf("expected company mismatch, got %s", resp["error"])
	}
}

func TestRequireActiveUser_UserNotFound(t *testing.T) {
	q := &mockQuerier{}
	w := doGet(setupUserRouter(q, testCompanyID.String()), "/user-only")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRequireAdmin_WrongSubjectType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &mockQuerier{admin: activeAdmin()}
	r := gin.New()
	r.Use(claimInjector("user", testCompanyID.String()))
	r.Use(RequireAdmin(q))
	r.GET("/admin-only", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := doGet(r, "/admin-only")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequirePlatformAdmin_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &mockQuerier{platform: &db.PlatformAdmin{ID: testSubjectID, Email: "root@platform"}}
	r := gin.New()
	r.Use(claimInjector("platform_admin", ""))
	r.Use(RequirePlatformAdmin(q))
	r.GET("/platform-only", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := doGet(r, "/platform-only")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequirePlatformAdmin_RejectsCompanyAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := &mockQuerier{platform: &db.PlatformAdmin{ID: testSubjectID}}
	r := gin.New()
	r.Use(claimInjector("admin", testCompanyID.String()))
	r.Use(RequirePlatformAdmin(q))
	r.GET("/platform-only", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := doGet(r, "/platform-only")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
