package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

func numeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("scan numeric %q: %v", s, err)
	}
	return n
}

func TestListCompanies_WithBalances(t *testing.T) {
	store := &mockPlatformStore{
		companies: []db.Company{
			{ID: uuid.New(), Slug: "acme", Name: "Acme", Status: db.CompanyStatusActive},
			{ID: uuid.New(), Slug: "globex", Name: "Globex", Status: db.CompanyStatusSuspended},
		},
		balance: numeric(t, "250000"),
	}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/companies", h.ListCompanies)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/companies", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 2 {
		t.Fatalf("expected 2 companies, got %d", len(data))
	}
	first := data[0].(map[string]interface{})
	if first["balance_xaf"] != "250000" {
		t.Fatalf("expected balance_xaf 250000, got %v", first["balance_xaf"])
	}
}

func TestListCompanies_Empty(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/companies", h.ListCompanies)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/companies", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if data := mustUnmarshalDataArray(t, w.Body.Bytes()); len(data) != 0 {
		t.Fatalf("expected empty array, got %d", len(data))
	}
}

func TestListCompanies_ListError(t *testing.T) {
	store := &mockPlatformStore{listErr: errors.New("boom")}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/companies", h.ListCompanies)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/companies", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGetCompany_Success(t *testing.T) {
	id := uuid.New()
	store := &mockPlatformStore{
		company: &db.Company{ID: id, Slug: "acme", Name: "Acme", Status: db.CompanyStatusActive},
		balance: numeric(t, "1000"),
	}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/companies/:id", h.GetCompany)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/companies/"+id.String(), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["slug"] != "acme" || data["balance_xaf"] != "1000" {
		t.Fatalf("unexpected company response: %v", data)
	}
}

func TestGetCompany_InvalidID(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/companies/:id", h.GetCompany)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/companies/not-a-uuid", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetCompany_NotFound(t *testing.T) {
	store := &mockPlatformStore{companyErr: errors.New("no rows")}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/companies/:id", h.GetCompany)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/companies/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetCompany_BalanceError(t *testing.T) {
	id := uuid.New()
	store := &mockPlatformStore{
		company:    &db.Company{ID: id, Slug: "acme"},
		balanceErr: errors.New("boom"),
	}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.GET("/companies/:id", h.GetCompany)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/companies/"+id.String(), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---- additional branches on existing handlers ----

func TestCreateCompany_CreateError(t *testing.T) {
	store := &mockPlatformStore{createCompanyErr: errors.New("dup slug")}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/companies", h.CreateCompany)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/companies",
		strings.NewReader(`{"slug":"acme","name":"Acme"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateCompanyAdmin_InvalidID(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/companies/:id/admins", h.CreateCompanyAdmin)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/companies/bad/admins",
		strings.NewReader(`{"email":"a@b.com","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateCompanyAdmin_CreateError(t *testing.T) {
	store := &mockPlatformStore{createAdminErr: errors.New("dup email")}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/companies/:id/admins", h.CreateCompanyAdmin)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/companies/"+uuid.New().String()+"/admins",
		strings.NewReader(`{"email":"a@b.com","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestTopUpCompany_InvalidAmountString(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/companies/:id/ledger/topup", h.TopUpCompany)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/companies/"+uuid.New().String()+"/ledger/topup",
		strings.NewReader(`{"amount_xaf":"not-a-number"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTopUpCompany_CompanyNotFound(t *testing.T) {
	store := &mockPlatformStore{companyErr: errors.New("no rows")}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/companies/:id/ledger/topup", h.TopUpCompany)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/companies/"+uuid.New().String()+"/ledger/topup",
		strings.NewReader(`{"amount_xaf":"5000"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateCompanyStatus_InvalidStatus(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.PUT("/companies/:id/status", h.UpdateCompanyStatus)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/companies/"+uuid.New().String()+"/status",
		strings.NewReader(`{"status":"deleted"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d", w.Code)
	}
}

func TestUpdateCompanyStatus_NotFound(t *testing.T) {
	store := &mockPlatformStore{companyErr: errors.New("no rows")}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.PUT("/companies/:id/status", h.UpdateCompanyStatus)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/companies/"+uuid.New().String()+"/status",
		strings.NewReader(`{"status":"active"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func platformPUT(store platformStore, h *PlatformHandler, register func(*gin.Engine), method, path, body string) *httptest.ResponseRecorder {
	r := platformGin(store)
	register(r)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestCreateCompanyAdmin_HashError(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, failingHasher{})
	w := platformPUT(store, h, func(r *gin.Engine) { r.POST("/companies/:id/admins", h.CreateCompanyAdmin) },
		"POST", "/companies/"+uuid.New().String()+"/admins", `{"email":"a@b.com","password":"secret123"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 hash_failed, got %d", w.Code)
	}
}

func TestTopUpCompany_LedgerError(t *testing.T) {
	store := &mockPlatformStore{ledgerErr: errors.New("db down")}
	h := NewPlatformHandler(store, stubHasher{})
	w := platformPUT(store, h, func(r *gin.Engine) { r.POST("/companies/:id/ledger/topup", h.TopUpCompany) },
		"POST", "/companies/"+uuid.New().String()+"/ledger/topup", `{"amount_xaf":"5000"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestTopUpCompany_BalanceError(t *testing.T) {
	store := &mockPlatformStore{balanceErr: errors.New("db down")}
	h := NewPlatformHandler(store, stubHasher{})
	w := platformPUT(store, h, func(r *gin.Engine) { r.POST("/companies/:id/ledger/topup", h.TopUpCompany) },
		"POST", "/companies/"+uuid.New().String()+"/ledger/topup", `{"amount_xaf":"5000"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestUpdateCompanyStatus_BalanceError(t *testing.T) {
	store := &mockPlatformStore{balanceErr: errors.New("db down")}
	h := NewPlatformHandler(store, stubHasher{})
	w := platformPUT(store, h, func(r *gin.Engine) { r.PUT("/companies/:id/status", h.UpdateCompanyStatus) },
		"PUT", "/companies/"+uuid.New().String()+"/status", `{"status":"active"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestTopUpCompany_InvalidID(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	w := platformPUT(store, h, func(r *gin.Engine) { r.POST("/companies/:id/ledger/topup", h.TopUpCompany) },
		"POST", "/companies/bad-id/ledger/topup", `{"amount_xaf":"5000"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateCompanyStatus_InvalidID(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	w := platformPUT(store, h, func(r *gin.Engine) { r.PUT("/companies/:id/status", h.UpdateCompanyStatus) },
		"PUT", "/companies/bad-id/status", `{"status":"active"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestIsNonPositive(t *testing.T) {
	var invalid pgtype.Numeric // zero value: !Valid
	if !isNonPositive(invalid) {
		t.Fatalf("invalid numeric must be non-positive")
	}
	pos := numeric(t, "1")
	if isNonPositive(pos) {
		t.Fatalf("1 must be positive")
	}
	neg := numeric(t, "-5")
	if !isNonPositive(neg) {
		t.Fatalf("-5 must be non-positive")
	}
}

func TestNumericToString_InvalidReturnsZero(t *testing.T) {
	var invalid pgtype.Numeric
	if got := numericToString(invalid); got != "0" {
		t.Fatalf("expected 0 for invalid numeric, got %q", got)
	}
}

func TestPlatformHandlers_BadJSONBodies(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	id := uuid.New().String()
	cases := []struct {
		name         string
		method, path string
		register     func(*gin.Engine)
	}{
		{"CreateCompany", "POST", "/companies", func(r *gin.Engine) { r.POST("/companies", h.CreateCompany) }},
		{"CreateCompanyAdmin", "POST", "/companies/" + id + "/admins", func(r *gin.Engine) { r.POST("/companies/:id/admins", h.CreateCompanyAdmin) }},
		{"TopUpCompany", "POST", "/companies/" + id + "/ledger/topup", func(r *gin.Engine) { r.POST("/companies/:id/ledger/topup", h.TopUpCompany) }},
		{"UpdateCompanyStatus", "PUT", "/companies/" + id + "/status", func(r *gin.Engine) { r.PUT("/companies/:id/status", h.UpdateCompanyStatus) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := platformPUT(store, h, tc.register, tc.method, tc.path, `not json`)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestPlatformAdminUUID_InvalidReturnsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("platform_admin_id", "not-a-uuid")
	if id := platformAdminUUID(c); id.Valid {
		t.Fatalf("expected invalid pgtype.UUID for a bad admin id")
	}
}
