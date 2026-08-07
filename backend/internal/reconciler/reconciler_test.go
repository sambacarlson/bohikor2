package reconciler

import (
	"context"
	"errors"
	"os"
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
	"github.com/Iknite-Space/bohikor2/internal/campay"
	"github.com/Iknite-Space/bohikor2/internal/database"
	"github.com/Iknite-Space/bohikor2/internal/dbtypes"
)

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

func seedRequest(t *testing.T, ctx context.Context, queries *db.Queries, status db.RequestStatus, campayRef string) db.AdvanceRequest {
	t.Helper()
	company, err := queries.CreateCompany(ctx, db.CreateCompanyParams{Slug: "co-" + uuid.NewString()[:8], Name: "Test Co"})
	if err != nil {
		t.Fatalf("seed company: %v", err)
	}
	user, err := queries.CreateUser(ctx, db.CreateUserParams{CompanyID: company.ID, Email: "u-" + uuid.NewString()[:8] + "@acme.com", Status: db.UserStatusActive})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var amount dbtypes.NumericString
	if err := amount.Scan("5000"); err != nil {
		t.Fatalf("scan amount: %v", err)
	}
	req, err := queries.CreateAdvanceRequest(ctx, db.CreateAdvanceRequestParams{CompanyID: company.ID, UserID: user.ID, AmountXaf: amount, Status: status})
	if err != nil {
		t.Fatalf("seed request: %v", err)
	}
	ref := pgtype.Text{Valid: false}
	if campayRef != "" {
		ref = pgtype.Text{String: campayRef, Valid: true}
	}
	req, err = queries.UpdateAdvanceRequestStatus(ctx, db.UpdateAdvanceRequestStatusParams{ID: req.ID, Status: status, CampayPayoutRef: ref})
	if err != nil {
		t.Fatalf("set campay ref: %v", err)
	}
	// Force the row into the past so it's picked up by ListReconcilableAdvanceRequests
	// (next_retry_at/created_at default to NOW() on insert).
	if _, err := queries.UpdateAdvanceRequestReconcileAttempt(ctx, db.UpdateAdvanceRequestReconcileAttemptParams{
		ID:               req.ID,
		NextRetryAt:      pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
		NeedsAdminReview: false,
	}); err != nil {
		t.Fatalf("backdate next_retry_at: %v", err)
	}
	req, err = queries.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("reload request: %v", err)
	}
	return req
}

type fakeStatusPoller struct {
	responses map[string]*campay.TransactionStatusResponse
	errs      map[string]error
}

func (f *fakeStatusPoller) GetTransactionStatus(ctx context.Context, reference string) (*campay.TransactionStatusResponse, error) {
	if err, ok := f.errs[reference]; ok {
		return nil, err
	}
	if resp, ok := f.responses[reference]; ok {
		return resp, nil
	}
	return nil, errors.New("unexpected reference in test")
}

func TestTick_ResolvesSuccessfulTransaction(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)
	req := seedRequest(t, ctx, queries, db.RequestStatusPending, "ref-success")

	poller := &fakeStatusPoller{responses: map[string]*campay.TransactionStatusResponse{
		"ref-success": {Reference: "ref-success", Status: "SUCCESSFUL"},
	}}
	r := New(pool, queries, poller)
	r.Tick(ctx)

	updated, err := queries.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetAdvanceRequestByID: %v", err)
	}
	if updated.Status != db.RequestStatusSuccess {
		t.Fatalf("expected success, got %s", updated.Status)
	}
}

func TestTick_BumpsAttemptOnStillPending(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)
	req := seedRequest(t, ctx, queries, db.RequestStatusPending, "ref-still-pending")

	poller := &fakeStatusPoller{responses: map[string]*campay.TransactionStatusResponse{
		"ref-still-pending": {Reference: "ref-still-pending", Status: "PENDING"},
	}}
	r := New(pool, queries, poller)
	r.Tick(ctx)

	updated, err := queries.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetAdvanceRequestByID: %v", err)
	}
	if updated.AttemptCount != req.AttemptCount+1 {
		t.Fatalf("expected attempt_count to increment, was %d now %d", req.AttemptCount, updated.AttemptCount)
	}
	if !updated.NextRetryAt.Valid {
		t.Fatal("expected next_retry_at to be set for the next backoff step")
	}
}

func TestTick_FlagsNeedsAdminReviewAfterMaxAttempts(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)
	req := seedRequest(t, ctx, queries, db.RequestStatusPending, "ref-stuck")

	poller := &fakeStatusPoller{responses: map[string]*campay.TransactionStatusResponse{
		"ref-stuck": {Reference: "ref-stuck", Status: "PENDING"},
	}}
	r := New(pool, queries, poller)

	// Drive attempt_count to the cap (5) by ticking repeatedly, backdating
	// next_retry_at before each tick so it's always due.
	for i := 0; i < len(backoffLadder); i++ {
		r.Tick(ctx)
		if _, err := queries.UpdateAdvanceRequestReconcileAttempt(ctx, db.UpdateAdvanceRequestReconcileAttemptParams{
			ID: req.ID, NextRetryAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true}, NeedsAdminReview: false,
		}); err != nil {
			t.Fatalf("backdate: %v", err)
		}
	}
	r.Tick(ctx)

	updated, err := queries.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetAdvanceRequestByID: %v", err)
	}
	if !updated.NeedsAdminReview {
		t.Fatalf("expected needs_admin_review after %d attempts, attempt_count=%d", len(backoffLadder)+1, updated.AttemptCount)
	}
}

func TestTick_NoRefRowsAgeIntoAdminReviewAfterGracePeriod(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)
	req := seedRequest(t, ctx, queries, db.RequestStatusProcessing, "")

	// Backdate created_at itself past the 10-minute grace period.
	if _, err := pool.Exec(ctx, `UPDATE advance_requests SET created_at = NOW() - INTERVAL '11 minutes' WHERE id = $1`, req.ID); err != nil {
		t.Fatalf("backdate created_at: %v", err)
	}
	if _, err := queries.UpdateAdvanceRequestReconcileAttempt(ctx, db.UpdateAdvanceRequestReconcileAttemptParams{
		ID: req.ID, NextRetryAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true}, NeedsAdminReview: false,
	}); err != nil {
		t.Fatalf("backdate next_retry_at: %v", err)
	}

	r := New(pool, queries, &fakeStatusPoller{})
	r.Tick(ctx)

	updated, err := queries.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetAdvanceRequestByID: %v", err)
	}
	if !updated.NeedsAdminReview {
		t.Fatal("expected no-ref row past grace period to be flagged for admin review")
	}
}

// TestTick_NoRefProcessingRowWithNoNextRetryAt_IsSurfacedByQuery is a
// regression test for the exact row shape CreateRequest produces on a pure
// Campay transport/timeout error: status=processing, campay_payout_ref NULL,
// next_retry_at NULL (nothing ever sets it on that path). Unlike seedRequest,
// which unconditionally backdates next_retry_at via
// UpdateAdvanceRequestReconcileAttempt (masking this exact bug), this test
// deliberately leaves next_retry_at unset so ListReconcilableAdvanceRequests
// must rely on its unconditional "processing AND campay_payout_ref IS NULL"
// branch to surface the row at all.
func TestTick_NoRefProcessingRowWithNoNextRetryAt_IsSurfacedByQuery(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)

	company, err := queries.CreateCompany(ctx, db.CreateCompanyParams{Slug: "co-" + uuid.NewString()[:8], Name: "Test Co"})
	if err != nil {
		t.Fatalf("seed company: %v", err)
	}
	user, err := queries.CreateUser(ctx, db.CreateUserParams{CompanyID: company.ID, Email: "u-" + uuid.NewString()[:8] + "@acme.com", Status: db.UserStatusActive})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var amount dbtypes.NumericString
	if err := amount.Scan("5000"); err != nil {
		t.Fatalf("scan amount: %v", err)
	}
	req, err := queries.CreateAdvanceRequest(ctx, db.CreateAdvanceRequestParams{CompanyID: company.ID, UserID: user.ID, AmountXaf: amount, Status: db.RequestStatusInitiated})
	if err != nil {
		t.Fatalf("seed request: %v", err)
	}

	// Mirror CreateRequest's transport-error path exactly: transition straight
	// to processing with campay_payout_ref left NULL (no Campay response was
	// ever received) and next_retry_at never touched.
	req, err = queries.UpdateAdvanceRequestStatus(ctx, db.UpdateAdvanceRequestStatusParams{
		ID:              req.ID,
		Status:          db.RequestStatusProcessing,
		CampayPayoutRef: pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("transition to processing: %v", err)
	}

	// Backdate created_at past the 10-minute grace period so handleNoRef acts
	// on it once the row is selected.
	if _, err := pool.Exec(ctx, `UPDATE advance_requests SET created_at = NOW() - INTERVAL '11 minutes' WHERE id = $1`, req.ID); err != nil {
		t.Fatalf("backdate created_at: %v", err)
	}

	preTick, err := queries.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("reload request: %v", err)
	}
	if preTick.NextRetryAt.Valid {
		t.Fatal("test setup invalid: next_retry_at must be NULL to reproduce the production bug shape")
	}
	if preTick.CampayPayoutRef.Valid {
		t.Fatal("test setup invalid: campay_payout_ref must be NULL to reproduce the production bug shape")
	}

	r := New(pool, queries, &fakeStatusPoller{})
	r.Tick(ctx)

	updated, err := queries.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetAdvanceRequestByID: %v", err)
	}
	if !updated.NeedsAdminReview {
		t.Fatal("expected no-ref processing row with NULL next_retry_at to be surfaced by ListReconcilableAdvanceRequests and flagged for admin review")
	}
}

// TestTick_RefdProcessingRowWithNoNextRetryAt_IsSurfacedByQuery is a
// regression test for the sibling of the bug fixed above: CreateRequest's
// post-transfer-DB-update-failure fallback (Campay already confirmed the
// transfer, but our own Transition call to record that outcome failed)
// transitions the row to processing WITH campay_payout_ref set, but never
// touches next_retry_at, leaving it NULL. Unlike seedRequest, which
// unconditionally backdates next_retry_at via
// UpdateAdvanceRequestReconcileAttempt (masking this exact bug), this test
// deliberately leaves next_retry_at unset so ListReconcilableAdvanceRequests
// must treat a NULL next_retry_at on a ref'd row as due immediately to
// surface it at all.
func TestTick_RefdProcessingRowWithNoNextRetryAt_IsSurfacedByQuery(t *testing.T) {
	dsn := getPG(t)
	ctx := context.Background()
	pool := newPool(t, dsn)
	queries := db.New(pool)

	company, err := queries.CreateCompany(ctx, db.CreateCompanyParams{Slug: "co-" + uuid.NewString()[:8], Name: "Test Co"})
	if err != nil {
		t.Fatalf("seed company: %v", err)
	}
	user, err := queries.CreateUser(ctx, db.CreateUserParams{CompanyID: company.ID, Email: "u-" + uuid.NewString()[:8] + "@acme.com", Status: db.UserStatusActive})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var amount dbtypes.NumericString
	if err := amount.Scan("5000"); err != nil {
		t.Fatalf("scan amount: %v", err)
	}
	req, err := queries.CreateAdvanceRequest(ctx, db.CreateAdvanceRequestParams{CompanyID: company.ID, UserID: user.ID, AmountXaf: amount, Status: db.RequestStatusInitiated})
	if err != nil {
		t.Fatalf("seed request: %v", err)
	}

	// Mirror CreateRequest's post-transfer-DB-update-failure fallback exactly:
	// transition straight to processing with campay_payout_ref set (Campay
	// already confirmed the transfer) and next_retry_at never touched.
	req, err = queries.UpdateAdvanceRequestStatus(ctx, db.UpdateAdvanceRequestStatusParams{
		ID:              req.ID,
		Status:          db.RequestStatusProcessing,
		CampayPayoutRef: pgtype.Text{String: "ref-deferred", Valid: true},
	})
	if err != nil {
		t.Fatalf("transition to processing: %v", err)
	}

	preTick, err := queries.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("reload request: %v", err)
	}
	if preTick.NextRetryAt.Valid {
		t.Fatal("test setup invalid: next_retry_at must be NULL to reproduce the production bug shape")
	}
	if !preTick.CampayPayoutRef.Valid {
		t.Fatal("test setup invalid: campay_payout_ref must be set to reproduce the production bug shape")
	}

	poller := &fakeStatusPoller{responses: map[string]*campay.TransactionStatusResponse{
		"ref-deferred": {Reference: "ref-deferred", Status: "SUCCESSFUL"},
	}}
	r := New(pool, queries, poller)
	r.Tick(ctx)

	updated, err := queries.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetAdvanceRequestByID: %v", err)
	}
	if updated.Status != db.RequestStatusSuccess {
		t.Fatalf("expected ref'd processing row with NULL next_retry_at to be surfaced by ListReconcilableAdvanceRequests and resolved to success, got status=%s", updated.Status)
	}
}
