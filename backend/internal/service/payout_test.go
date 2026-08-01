package service

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/database"
)

// These tests exercise TransitionRequest against a real Postgres, since its
// entire purpose is correctness of atomic status/ledger/event writes under a
// DB transaction — a fake store would only prove the fake was called, not
// that the ledger nets to zero. Skips automatically when Docker is unavailable,
// mirroring internal/server/integration_test.go.

var (
	pgOnce      sync.Once
	pgDSN       string
	pgErr       error
	pgContainer *tcpostgres.PostgresContainer
)

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
		// migrations/ lives at the backend root, two levels up from internal/service.
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

// seedAdvanceRequest creates a company, a user, and an advance_requests row in
// the given initial status, returning the row and queries bound to the pool.
func seedAdvanceRequest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, initial db.RequestStatus, amount string) db.AdvanceRequest {
	t.Helper()
	queries := db.New(pool)

	company, err := queries.CreateCompany(ctx, db.CreateCompanyParams{
		Slug: "co-" + uuid.NewString()[:8],
		Name: "Test Co",
	})
	if err != nil {
		t.Fatalf("seed company: %v", err)
	}
	if err := queries.SeedDefaultSettings(ctx, company.ID); err != nil {
		t.Fatalf("seed default settings: %v", err)
	}

	user, err := queries.CreateUser(ctx, db.CreateUserParams{
		CompanyID: company.ID,
		Email:     "user-" + uuid.NewString()[:8] + "@acme.com",
		Status:    db.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var amountXaf pgtype.Numeric
	if err := amountXaf.Scan(amount); err != nil {
		t.Fatalf("scan amount: %v", err)
	}

	req, err := queries.CreateAdvanceRequest(ctx, db.CreateAdvanceRequestParams{
		CompanyID: company.ID,
		UserID:    user.ID,
		AmountXaf: amountXaf,
		Status:    initial,
	})
	if err != nil {
		t.Fatalf("seed advance request: %v", err)
	}
	return req
}

func ledgerEntryTypes(t *testing.T, ctx context.Context, queries *db.Queries, companyID uuid.UUID) []string {
	t.Helper()
	entries, err := queries.ListLedgerByCompany(ctx, db.ListLedgerByCompanyParams{CompanyID: companyID, Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("ListLedgerByCompany: %v", err)
	}
	types := make([]string, len(entries))
	for i, e := range entries {
		types[i] = e.EntryType
	}
	return types
}

func eventTypes(t *testing.T, ctx context.Context, queries *db.Queries, companyID uuid.UUID) []string {
	t.Helper()
	events, err := queries.ListEventsWithUserByCompany(ctx, db.ListEventsWithUserByCompanyParams{CompanyID: pgtype.UUID{Bytes: companyID, Valid: true}, Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("ListEventsWithUserByCompany: %v", err)
	}
	types := make([]string, len(events))
	for i, e := range events {
		types[i] = e.EventType
	}
	return types
}

// numericToStringT renders a pgtype.Numeric to a plain string for assertions.
func numericToStringT(t *testing.T, n pgtype.Numeric) string {
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

func withTx(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx)) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	fn(tx)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
}

func TestTransitionRequest_ToSuccess_EmitsEventNoLedger(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	req := seedAdvanceRequest(t, ctx, pool, db.RequestStatusInitiated, "5000")

	var updated db.AdvanceRequest
	withTx(t, ctx, pool, func(tx pgx.Tx) {
		var err error
		updated, err = TransitionRequest(ctx, tx, req, db.RequestStatusSuccess, TransitionOpts{CampayPayoutRef: "campay-ref-1"})
		if err != nil {
			t.Fatalf("TransitionRequest: %v", err)
		}
	})

	if updated.Status != db.RequestStatusSuccess {
		t.Fatalf("expected status success, got %s", updated.Status)
	}
	if !updated.CampayPayoutRef.Valid || updated.CampayPayoutRef.String != "campay-ref-1" {
		t.Fatalf("expected campay_payout_ref to be set, got %+v", updated.CampayPayoutRef)
	}

	queries := db.New(pool)
	events := eventTypes(t, ctx, queries, req.CompanyID)
	found := false
	for _, e := range events {
		if e == "payout_success" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a payout_success event, got %v", events)
	}

	if ledger := ledgerEntryTypes(t, ctx, queries, req.CompanyID); len(ledger) != 0 {
		t.Fatalf("expected no ledger entries on success, got %v", ledger)
	}
}

func TestTransitionRequest_ToFailed_PostsReversalNettingToZero(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)
	req := seedAdvanceRequest(t, ctx, pool, db.RequestStatusInitiated, "5000")

	// Simulate the §2 debit that would have been posted at request-creation time.
	var debitAmount pgtype.Numeric
	if err := debitAmount.Scan("-5000"); err != nil {
		t.Fatalf("scan debit amount: %v", err)
	}
	if _, err := queries.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		CompanyID:        req.CompanyID,
		EntryType:        "payout_debit",
		AmountXaf:        debitAmount,
		AdvanceRequestID: pgtype.UUID{Bytes: req.ID, Valid: true},
	}); err != nil {
		t.Fatalf("seed payout_debit: %v", err)
	}

	var updated db.AdvanceRequest
	withTx(t, ctx, pool, func(tx pgx.Tx) {
		var err error
		updated, err = TransitionRequest(ctx, tx, req, db.RequestStatusFailed, TransitionOpts{FailureReason: "insufficient funds"})
		if err != nil {
			t.Fatalf("TransitionRequest: %v", err)
		}
	})

	if updated.Status != db.RequestStatusFailed {
		t.Fatalf("expected status failed, got %s", updated.Status)
	}
	if !updated.FailureReason.Valid || updated.FailureReason.String != "insufficient funds" {
		t.Fatalf("expected failure_reason to be set, got %+v", updated.FailureReason)
	}

	if ledger := ledgerEntryTypes(t, ctx, queries, req.CompanyID); len(ledger) != 2 {
		t.Fatalf("expected debit + reversal entries, got %v", ledger)
	}

	bal, err := queries.GetCompanyBalance(ctx, req.CompanyID)
	if err != nil {
		t.Fatalf("GetCompanyBalance: %v", err)
	}
	if s := numericToStringT(t, bal); s != "0" {
		t.Fatalf("expected debit+reversal to net to zero, got %q", s)
	}

	events := eventTypes(t, ctx, queries, req.CompanyID)
	found := false
	for _, e := range events {
		if e == "payout_failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a payout_failed event, got %v", events)
	}
}

func TestTransitionRequest_NoopWhenAlreadyTerminalAndUnchanged(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)
	req := seedAdvanceRequest(t, ctx, pool, db.RequestStatusInitiated, "5000")

	var first db.AdvanceRequest
	withTx(t, ctx, pool, func(tx pgx.Tx) {
		var err error
		first, err = TransitionRequest(ctx, tx, req, db.RequestStatusSuccess, TransitionOpts{CampayPayoutRef: "campay-ref-2"})
		if err != nil {
			t.Fatalf("TransitionRequest (first): %v", err)
		}
	})

	var second db.AdvanceRequest
	withTx(t, ctx, pool, func(tx pgx.Tx) {
		var err error
		second, err = TransitionRequest(ctx, tx, first, db.RequestStatusSuccess, TransitionOpts{CampayPayoutRef: "campay-ref-2"})
		if err != nil {
			t.Fatalf("TransitionRequest (second): %v", err)
		}
	})

	if second.UpdatedAt != first.UpdatedAt {
		t.Fatalf("expected no-op to leave row unchanged, first updated_at=%v second=%v", first.UpdatedAt, second.UpdatedAt)
	}

	events := eventTypes(t, ctx, queries, req.CompanyID)
	count := 0
	for _, e := range events {
		if e == "payout_success" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one payout_success event after duplicate transition, got %d in %v", count, events)
	}
}

func TestTransitionRequest_NoopWhenAlreadyTerminalButDifferentStatus(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)
	req := seedAdvanceRequest(t, ctx, pool, db.RequestStatusInitiated, "5000")

	var succeeded db.AdvanceRequest
	withTx(t, ctx, pool, func(tx pgx.Tx) {
		var err error
		succeeded, err = TransitionRequest(ctx, tx, req, db.RequestStatusSuccess, TransitionOpts{CampayPayoutRef: "campay-ref-terminal-flip"})
		if err != nil {
			t.Fatalf("TransitionRequest (to success): %v", err)
		}
	})

	var afterFlip db.AdvanceRequest
	withTx(t, ctx, pool, func(tx pgx.Tx) {
		var err error
		afterFlip, err = TransitionRequest(ctx, tx, succeeded, db.RequestStatusFailed, TransitionOpts{FailureReason: "contradicting webhook"})
		if err != nil {
			t.Fatalf("TransitionRequest (attempted flip to failed): %v", err)
		}
	})

	if afterFlip.Status != db.RequestStatusSuccess {
		t.Fatalf("expected terminal status to be immutable once success, got %s", afterFlip.Status)
	}
	if afterFlip.UpdatedAt != succeeded.UpdatedAt {
		t.Fatalf("expected no-op to leave row unchanged, before=%v after=%v", succeeded.UpdatedAt, afterFlip.UpdatedAt)
	}

	types := ledgerEntryTypes(t, ctx, queries, req.CompanyID)
	for _, lt := range types {
		if lt == "reversal" {
			t.Fatalf("expected no reversal to be posted for a blocked terminal flip, got ledger entries %v", types)
		}
	}
}

func TestTransitionRequest_ToPending_NoEventNoLedger(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)
	req := seedAdvanceRequest(t, ctx, pool, db.RequestStatusInitiated, "5000")

	withTx(t, ctx, pool, func(tx pgx.Tx) {
		if _, err := TransitionRequest(ctx, tx, req, db.RequestStatusPending, TransitionOpts{CampayPayoutRef: "campay-ref-3"}); err != nil {
			t.Fatalf("TransitionRequest: %v", err)
		}
	})

	if ledger := ledgerEntryTypes(t, ctx, queries, req.CompanyID); len(ledger) != 0 {
		t.Fatalf("expected no ledger entries on pending, got %v", ledger)
	}
	if events := eventTypes(t, ctx, queries, req.CompanyID); len(events) != 0 {
		t.Fatalf("expected no events on pending, got %v", events)
	}
}

func TestTransitionRequest_PreservesExistingCampayRefWhenNotOverridden(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	req := seedAdvanceRequest(t, ctx, pool, db.RequestStatusPending, "5000")

	// Simulate a row that already resolved a campay_payout_ref before this transition.
	queries := db.New(pool)
	var err error
	req, err = queries.UpdateAdvanceRequestStatus(ctx, db.UpdateAdvanceRequestStatusParams{
		ID:              req.ID,
		Status:          db.RequestStatusPending,
		CampayPayoutRef: pgtype.Text{String: "existing-ref", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed existing campay ref: %v", err)
	}

	var updated db.AdvanceRequest
	withTx(t, ctx, pool, func(tx pgx.Tx) {
		updated, err = TransitionRequest(ctx, tx, req, db.RequestStatusSuccess, TransitionOpts{})
		if err != nil {
			t.Fatalf("TransitionRequest: %v", err)
		}
	})

	if !updated.CampayPayoutRef.Valid || updated.CampayPayoutRef.String != "existing-ref" {
		t.Fatalf("expected existing campay_payout_ref to be preserved, got %+v", updated.CampayPayoutRef)
	}
}
