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

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/dbtypes"
)

// mockPlatformStore records provisioning calls and lets tests assert behaviour.
type mockPlatformStore struct {
	createdCompany   *db.CreateCompanyParams
	seededCompany    bool
	companies        []db.Company
	company          *db.Company
	companyErr       error
	createAdminArg   *db.CreateAdminParams
	createAdminErr   error
	ledgerArg        *db.CreateLedgerEntryParams
	balance          dbtypes.NumericString
	createCompanyErr error
	listErr          error
	balanceErr       error
	ledgerErr        error
	needsReview      []db.ListRequestsNeedingReviewAcrossCompaniesRow
	needsReviewErr   error
	health           []db.ListCompanyRequestHealthRow
	healthErr        error
}

func (m *mockPlatformStore) CreateCompanyWithSettings(ctx context.Context, arg db.CreateCompanyParams) (db.Company, error) {
	if m.createCompanyErr != nil {
		return db.Company{}, m.createCompanyErr
	}
	m.createdCompany = &arg
	m.seededCompany = true
	return db.Company{ID: uuid.New(), Slug: arg.Slug, Name: arg.Name, Status: db.CompanyStatusActive}, nil
}

func (m *mockPlatformStore) GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error) {
	if m.companyErr != nil {
		return db.Company{}, m.companyErr
	}
	if m.company == nil {
		return db.Company{ID: id, Slug: "acme", Name: "Acme", Status: db.CompanyStatusActive}, nil
	}
	return *m.company, nil
}

func (m *mockPlatformStore) ListCompaniesWithBalance(ctx context.Context) ([]db.ListCompaniesWithBalanceRow, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	rows := make([]db.ListCompaniesWithBalanceRow, len(m.companies))
	for i, company := range m.companies {
		rows[i] = db.ListCompaniesWithBalanceRow{
			ID:        company.ID,
			Slug:      company.Slug,
			Name:      company.Name,
			Status:    company.Status,
			CreatedBy: company.CreatedBy,
			CreatedAt: company.CreatedAt,
			UpdatedAt: company.UpdatedAt,
			Balance:   m.balance,
		}
	}
	return rows, nil
}

func (m *mockPlatformStore) UpdateCompanyStatus(ctx context.Context, arg db.UpdateCompanyStatusParams) (db.Company, error) {
	if m.companyErr != nil {
		return db.Company{}, m.companyErr
	}
	return db.Company{ID: arg.ID, Slug: "acme", Name: "Acme", Status: arg.Status}, nil
}

func (m *mockPlatformStore) CreateAdmin(ctx context.Context, arg db.CreateAdminParams) (db.Admin, error) {
	if m.createAdminErr != nil {
		return db.Admin{}, m.createAdminErr
	}
	m.createAdminArg = &arg
	return db.Admin{ID: uuid.New(), CompanyID: arg.CompanyID, Email: arg.Email}, nil
}

func (m *mockPlatformStore) CreateLedgerEntry(ctx context.Context, arg db.CreateLedgerEntryParams) (db.CompanyLedger, error) {
	if m.ledgerErr != nil {
		return db.CompanyLedger{}, m.ledgerErr
	}
	m.ledgerArg = &arg
	return db.CompanyLedger{ID: uuid.New(), CompanyID: arg.CompanyID, EntryType: arg.EntryType, AmountXaf: arg.AmountXaf}, nil
}

func (m *mockPlatformStore) GetCompanyBalance(ctx context.Context, companyID uuid.UUID) (dbtypes.NumericString, error) {
	return m.balance, m.balanceErr
}

func (m *mockPlatformStore) ListRequestsNeedingReview(ctx context.Context) ([]db.ListRequestsNeedingReviewAcrossCompaniesRow, error) {
	return m.needsReview, m.needsReviewErr
}

func (m *mockPlatformStore) ListCompanyRequestHealth(ctx context.Context) ([]db.ListCompanyRequestHealthRow, error) {
	return m.health, m.healthErr
}

// stubHasher avoids bcrypt cost in unit tests.
type stubHasher struct{}

func (stubHasher) Hash(plain string) (string, error) { return "hashed:" + plain, nil }
func (stubHasher) Verify(hash, plain string) bool    { return hash == "hashed:"+plain }

func platformGin(store platformStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("platform_admin_id", uuid.New().String())
		c.Next()
	})
	return r
}

func TestCreateCompany_SeedsSettings(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/api/platform/companies", h.CreateCompany)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/platform/companies",
		strings.NewReader(`{"slug":"acme","name":"Acme Corp"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !store.seededCompany {
		t.Fatal("expected default settings to be seeded on company create")
	}
	if store.createdCompany == nil || store.createdCompany.Slug != "acme" {
		t.Fatalf("expected company slug acme, got %+v", store.createdCompany)
	}
}

func TestCreateCompany_InvalidSlug(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/api/platform/companies", h.CreateCompany)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/platform/companies",
		strings.NewReader(`{"slug":"Acme Corp!","name":"Acme"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if store.seededCompany {
		t.Fatal("invalid slug must not create a company")
	}
}

func TestCreateCompanyAdmin_ScopedToCompany(t *testing.T) {
	companyID := uuid.New()
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/api/platform/companies/:id/admins", h.CreateCompanyAdmin)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/platform/companies/"+companyID.String()+"/admins",
		strings.NewReader(`{"email":"boss@acme.com","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if store.createAdminArg == nil || store.createAdminArg.CompanyID != companyID {
		t.Fatalf("expected admin scoped to company %s, got %+v", companyID, store.createAdminArg)
	}
	if store.createAdminArg.PasswordHash != "hashed:secret123" {
		t.Fatalf("expected password to be hashed, got %q", store.createAdminArg.PasswordHash)
	}
}

func TestTopUpCompany_PostsPositiveLedgerEntry(t *testing.T) {
	companyID := uuid.New()
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/api/platform/companies/:id/ledger/topup", h.TopUpCompany)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/platform/companies/"+companyID.String()+"/ledger/topup",
		strings.NewReader(`{"amount_xaf":"500000","note":"initial float"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if store.ledgerArg == nil || store.ledgerArg.EntryType != "topup" {
		t.Fatalf("expected a topup ledger entry, got %+v", store.ledgerArg)
	}
	if store.ledgerArg.CompanyID != companyID {
		t.Fatalf("expected ledger entry scoped to company %s", companyID)
	}
}

func TestTopUpCompany_RejectsNonPositive(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/api/platform/companies/:id/ledger/topup", h.TopUpCompany)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/platform/companies/"+uuid.New().String()+"/ledger/topup",
		strings.NewReader(`{"amount_xaf":"0"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if store.ledgerArg != nil {
		t.Fatal("non-positive top-up must not post a ledger entry")
	}
}

func TestAdjustCompanyLedger_PostsSignedEntry(t *testing.T) {
	companyID := uuid.New()
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/api/platform/companies/:id/ledger/adjustment", h.AdjustCompanyLedger)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/platform/companies/"+companyID.String()+"/ledger/adjustment",
		strings.NewReader(`{"amount_xaf":"-1500","note":"correcting duplicate topup"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if store.ledgerArg == nil || store.ledgerArg.EntryType != "adjustment" {
		t.Fatalf("expected an adjustment ledger entry, got %+v", store.ledgerArg)
	}
	if store.ledgerArg.CompanyID != companyID {
		t.Fatalf("expected ledger entry scoped to company %s", companyID)
	}
}

func TestAdjustCompanyLedger_RejectsZero(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/api/platform/companies/:id/ledger/adjustment", h.AdjustCompanyLedger)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/platform/companies/"+uuid.New().String()+"/ledger/adjustment",
		strings.NewReader(`{"amount_xaf":"0","note":"noop"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a zero adjustment, got %d", w.Code)
	}
	if store.ledgerArg != nil {
		t.Fatal("a zero adjustment must not post a ledger entry")
	}
}

func TestUpdateCompanyStatus_Suspend(t *testing.T) {
	store := &mockPlatformStore{}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.PUT("/api/platform/companies/:id/status", h.UpdateCompanyStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT",
		"/api/platform/companies/"+uuid.New().String()+"/status",
		strings.NewReader(`{"status":"suspended"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["status"] != string(db.CompanyStatusSuspended) {
		t.Fatalf("expected status suspended, got %v", data["status"])
	}
}

func TestCreateCompanyAdmin_CompanyNotFound(t *testing.T) {
	store := &mockPlatformStore{companyErr: errors.New("no rows")}
	h := NewPlatformHandler(store, stubHasher{})
	r := platformGin(store)
	r.POST("/api/platform/companies/:id/admins", h.CreateCompanyAdmin)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST",
		"/api/platform/companies/"+uuid.New().String()+"/admins",
		strings.NewReader(`{"email":"boss@acme.com","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
