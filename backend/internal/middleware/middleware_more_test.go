package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

func TestLogger_PassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger())
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusTeapot, gin.H{"ok": true}) })
	w := doGet(r, "/x")
	if w.Code != http.StatusTeapot {
		t.Fatalf("Logger must not alter the response, got %d", w.Code)
	}
}

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := doGet(r, "/x")
	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Fatalf("expected a generated X-Request-ID header")
	}
}

func TestRequestID_PreservesProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/x", nil)
	req.Header.Set("X-Request-ID", "caller-supplied-id")
	r.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-ID"); got != "caller-supplied-id" {
		t.Fatalf("expected provided request id to be preserved, got %q", got)
	}
}

func TestGenerateID_NonEmpty(t *testing.T) {
	if generateID() == "" {
		t.Fatal("generateID must return a non-empty id")
	}
}

func TestGetLockedMessage(t *testing.T) {
	cases := map[db.UserStatus]string{
		db.UserStatusSuspended: "account suspended",
		db.UserStatusLocked:    "account locked",
		db.UserStatusActive:    "account restricted", // default branch
	}
	for status, want := range cases {
		if got := getLockedMessage(status); got != want {
			t.Errorf("getLockedMessage(%v) = %q, want %q", status, got, want)
		}
	}
}

// ---- role guard branches not yet covered ----

func setupPlatformRouter(q Querier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(claimInjector("platform_admin", ""))
	r.Use(RequirePlatformAdmin(q))
	r.GET("/p", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

// injector that sets an explicit (possibly empty/invalid) subject id.
func rawInjector(subjectID, subjectType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("subject_id", subjectID)
		c.Set("subject_type", subjectType)
		c.Next()
	}
}

func TestRequireAdmin_MissingSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rawInjector("", "admin"))
	r.Use(RequireAdmin(&mockQuerier{}))
	r.GET("/a", func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := doGet(r, "/a"); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireAdmin_InvalidSubjectID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rawInjector("not-a-uuid", "admin"))
	r.Use(RequireAdmin(&mockQuerier{admin: activeAdmin()}))
	r.GET("/a", func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := doGet(r, "/a"); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireAdmin_CompanyNotFound(t *testing.T) {
	q := &mockQuerier{admin: activeAdmin(), companyErr: errNotFound}
	if w := doGet(setupRoleRouter(q, testCompanyID.String()), "/admin-only"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequireActiveUser_MissingSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rawInjector("", "user"))
	r.Use(RequireActiveUser(&mockQuerier{}))
	r.GET("/u", func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := doGet(r, "/u"); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireActiveUser_WrongSubjectType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rawInjector(testSubjectID.String(), "admin"))
	r.Use(RequireActiveUser(&mockQuerier{user: activeUser()}))
	r.GET("/u", func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := doGet(r, "/u"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequireActiveUser_InvalidSubjectID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rawInjector("not-a-uuid", "user"))
	r.Use(RequireActiveUser(&mockQuerier{user: activeUser()}))
	r.GET("/u", func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := doGet(r, "/u"); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireActiveUser_SuspendedCompany(t *testing.T) {
	q := &mockQuerier{user: activeUser(), company: &db.Company{ID: testCompanyID, Status: db.CompanyStatusSuspended}}
	if w := doGet(setupUserRouter(q, testCompanyID.String()), "/user-only"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRequirePlatformAdmin_MissingSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rawInjector("", "platform_admin"))
	r.Use(RequirePlatformAdmin(&mockQuerier{}))
	r.GET("/p", func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := doGet(r, "/p"); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequirePlatformAdmin_InvalidSubjectID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(rawInjector("not-a-uuid", "platform_admin"))
	r.Use(RequirePlatformAdmin(&mockQuerier{}))
	r.GET("/p", func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := doGet(r, "/p"); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequirePlatformAdmin_NotFound(t *testing.T) {
	q := &mockQuerier{platformE: errNotFound}
	if w := doGet(setupPlatformRouter(q), "/p"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
