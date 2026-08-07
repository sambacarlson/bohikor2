package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/config"
	"github.com/Iknite-Space/bohikor2/internal/database"
	"github.com/Iknite-Space/bohikor2/internal/dbtypes"
	"github.com/Iknite-Space/bohikor2/internal/handler"
	"github.com/Iknite-Space/bohikor2/internal/service"
)

// These integration tests exercise the DB-backed code that unit tests mock out:
// the RealPlatformStore/RealInviteStore query wrappers and the full server.New
// wiring (pool + migrations + route graph). They run against a throwaway
// Postgres started via testcontainers and SKIP automatically when Docker is
// unavailable, so the default suite still passes without Docker.

var (
	pgOnce      sync.Once
	pgDSN       string
	pgErr       error
	pgContainer *tcpostgres.PostgresContainer
)

// getPG lazily starts one shared Postgres container, applies the schema
// migrations, and returns its DSN. Skips the calling test if Docker isn't up.
func getPG(t *testing.T) string {
	t.Helper()
	pgOnce.Do(func() {
		ctx := context.Background()
		pgContainer, pgErr = tcpostgres.Run(ctx, "postgres:18-alpine",
			tcpostgres.WithDatabase("bohikor2"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForListeningPort("5432/tcp").WithStartupTimeout(90*time.Second),
			),
		)
		if pgErr != nil {
			return
		}
		pgDSN, pgErr = pgContainer.ConnectionString(ctx, "sslmode=disable")
		if pgErr != nil {
			return
		}
		// migrations/ lives at the backend root, two levels up from internal/server.
		pgErr = database.RunMigrations(pgDSN, "../../migrations")
	})
	if pgErr != nil {
		t.Skipf("skipping integration test: Docker/Postgres unavailable: %v", pgErr)
	}
	return pgDSN
}

func TestMain(m *testing.M) {
	code := m.Run()
	if pgContainer != nil {
		_ = pgContainer.Terminate(context.Background())
	}
	os.Exit(code)
}

func newPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestIntegration_RealPlatformStore drives the transactional company-provisioning
// path and the ledger/balance queries against a real database.
func TestIntegration_RealPlatformStore(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	store := handler.NewRealPlatformStore(db.New(pool), pool)

	slug := "acme-" + uuid.NewString()[:8]

	// Create company + seeded settings in one transaction.
	company, err := store.CreateCompanyWithSettings(ctx, db.CreateCompanyParams{
		Slug: slug,
		Name: "Acme Corp",
	})
	if err != nil {
		t.Fatalf("CreateCompanyWithSettings: %v", err)
	}
	if company.Slug != slug {
		t.Fatalf("expected slug %q, got %q", slug, company.Slug)
	}

	// The transaction must have seeded default settings for the new company.
	settings, err := db.New(pool).ListSettingsByCompany(ctx, company.ID)
	if err != nil {
		t.Fatalf("ListSettingsByCompany: %v", err)
	}
	if len(settings) == 0 {
		t.Fatal("expected default settings to be seeded with the company")
	}

	// Lookups.
	got, err := store.GetCompanyByID(ctx, company.ID)
	if err != nil || got.ID != company.ID {
		t.Fatalf("GetCompanyByID: got %+v err %v", got, err)
	}
	all, err := store.ListCompaniesWithBalance(ctx)
	if err != nil || len(all) == 0 {
		t.Fatalf("ListCompaniesWithBalance: got %d err %v", len(all), err)
	}

	// A fresh company has a zero balance.
	bal, err := store.GetCompanyBalance(ctx, company.ID)
	if err != nil {
		t.Fatalf("GetCompanyBalance: %v", err)
	}
	if s := numericToStringT(t, bal); s != "0" {
		t.Fatalf("expected zero starting balance, got %q", s)
	}

	// A top-up ledger entry increases the balance.
	var amount dbtypes.NumericString
	if err := amount.Scan("500000"); err != nil {
		t.Fatalf("scan amount: %v", err)
	}
	if _, err := store.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		CompanyID: company.ID,
		EntryType: "topup",
		AmountXaf: amount,
	}); err != nil {
		t.Fatalf("CreateLedgerEntry: %v", err)
	}
	bal, err = store.GetCompanyBalance(ctx, company.ID)
	if err != nil {
		t.Fatalf("GetCompanyBalance after topup: %v", err)
	}
	// The ledger SUM is a scale-2 numeric, so 500000 renders as "500000.00".
	if s := numericToStringT(t, bal); s != "500000.00" {
		t.Fatalf("expected balance 500000.00 after topup, got %q", s)
	}

	// Create an admin scoped to the company.
	admin, err := store.CreateAdmin(ctx, db.CreateAdminParams{
		CompanyID:    company.ID,
		Email:        "boss-" + uuid.NewString()[:8] + "@acme.com",
		PasswordHash: "hashed",
	})
	if err != nil || admin.CompanyID != company.ID {
		t.Fatalf("CreateAdmin: got %+v err %v", admin, err)
	}

	// Suspend the company.
	suspended, err := store.UpdateCompanyStatus(ctx, db.UpdateCompanyStatusParams{
		ID:     company.ID,
		Status: db.CompanyStatusSuspended,
	})
	if err != nil || suspended.Status != db.CompanyStatusSuspended {
		t.Fatalf("UpdateCompanyStatus: got %+v err %v", suspended, err)
	}
}

// TestIntegration_RealInviteStore covers the invitation query wrappers.
func TestIntegration_RealInviteStore(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)

	// Need a company to attach the invitation to.
	platform := handler.NewRealPlatformStore(queries, pool)
	company, err := platform.CreateCompanyWithSettings(ctx, db.CreateCompanyParams{
		Slug: "inv-" + uuid.NewString()[:8],
		Name: "Invite Co",
	})
	if err != nil {
		t.Fatalf("seed company: %v", err)
	}

	store := service.NewRealInviteStore(queries)
	email := "invitee-" + uuid.NewString()[:8] + "@acme.com"

	inv, err := store.CreateInvitation(ctx, email, company.ID, pgtype.UUID{})
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}
	if inv.Email != email {
		t.Fatalf("expected email %q, got %q", email, inv.Email)
	}

	fetched, err := store.GetInvitationByEmail(ctx, email)
	if err != nil || fetched.ID != inv.ID {
		t.Fatalf("GetInvitationByEmail: got %+v err %v", fetched, err)
	}

	updated, err := store.UpdateInvitationStatus(ctx, db.InvitationStatusSent, pgtype.UUID{Bytes: inv.ID, Valid: true})
	if err != nil || updated.Status != db.InvitationStatusSent {
		t.Fatalf("UpdateInvitationStatus: got %+v err %v", updated, err)
	}
}

// TestIntegration_ServerNew boots the full application (pool + migrations +
// route graph) and hits a route through the assembled router.
func TestIntegration_ServerNew(t *testing.T) {
	dsn := getPG(t)

	// server.New runs migrations from a relative "migrations" dir, so run from
	// the backend root for this test.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("chdir to backend root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	cfg := &config.Config{
		Env:             "development",
		Port:            0,
		DatabaseURL:     dsn,
		JWTSecret:       strings.Repeat("x", 40),
		JWTAccessExpiry: 15 * time.Minute,
		CampayBaseURL:   "http://localhost",
		Timezone:        "UTC",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		IdleTimeout:     10 * time.Second,
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	t.Cleanup(func() {
		if srv.pool != nil {
			srv.pool.Close()
		}
	})

	// /health is wired without auth; drive it through the real router.
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/health", nil)
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected /health 200, got %d", w.Code)
	}

	// A protected route with no token must be rejected by the auth middleware,
	// proving the middleware chain is actually wired.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/users", nil)
	srv.router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected /api/admin/users 401 without token, got %d", w2.Code)
	}
}

// numericToStringT renders a dbtypes.NumericString to a plain string for assertions.
func numericToStringT(t *testing.T, n dbtypes.NumericString) string {
	t.Helper()
	v, err := n.Value()
	if err != nil || v == nil {
		return "0"
	}
	if s, ok := v.(string); ok {
		return s
	}
	t.Fatalf("unexpected numeric value type %T", v)
	return ""
}
