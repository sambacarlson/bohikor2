package handler

import (
	"context"
	"encoding/json"
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
	"github.com/Iknite-Space/bohikor2/internal/database"
	"github.com/Iknite-Space/bohikor2/internal/dbtypes"
	"github.com/Iknite-Space/bohikor2/internal/service"
)

var (
	handlerPgOnce      sync.Once
	handlerPgDSN       string
	handlerPgErr       error
	handlerPgContainer *tcpostgres.PostgresContainer
)

func getHandlerPG(t *testing.T) string {
	t.Helper()
	handlerPgOnce.Do(func() {
		ctx := context.Background()
		handlerPgContainer, handlerPgErr = tcpostgres.Run(ctx, "postgres:18-alpine",
			tcpostgres.WithDatabase("bohikor2"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForListeningPort("5432/tcp").WithStartupTimeout(90*time.Second),
			),
		)
		if handlerPgErr != nil {
			return
		}
		handlerPgDSN, handlerPgErr = handlerPgContainer.ConnectionString(ctx, "sslmode=disable")
		if handlerPgErr != nil {
			return
		}
		handlerPgErr = database.RunMigrations(handlerPgDSN, "../../migrations")
	})
	if handlerPgErr != nil {
		t.Skipf("skipping integration test: Docker/Postgres unavailable: %v", handlerPgErr)
	}
	return handlerPgDSN
}

func handlerTestMain(m *testing.M) int {
	code := m.Run()
	if handlerPgContainer != nil {
		_ = handlerPgContainer.Terminate(context.Background())
	}
	return code
}

func TestMain(m *testing.M) {
	os.Exit(handlerTestMain(m))
}

func newHandlerPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedCompanyAndUser(t *testing.T, ctx context.Context, queries *db.Queries) (db.Company, db.User) {
	t.Helper()
	company, err := queries.CreateCompany(ctx, db.CreateCompanyParams{
		Slug: "co-" + uuid.NewString()[:8],
		Name: "Test Co",
	})
	if err != nil {
		t.Fatalf("seed company: %v", err)
	}
	user, err := queries.CreateUser(ctx, db.CreateUserParams{
		CompanyID: company.ID,
		Email:     "user-" + uuid.NewString()[:8] + "@acme.com",
		Status:    db.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return company, user
}

func TestRealAdvanceStore_CreateAdvanceRequestWithDebit_ReservesFloat(t *testing.T) {
	dsn := getHandlerPG(t)
	ctx := context.Background()
	pool := newHandlerPool(t, dsn)
	queries := db.New(pool)
	company, user := seedCompanyAndUser(t, ctx, queries)

	var topUp dbtypes.NumericString
	if err := topUp.Scan("5000"); err != nil {
		t.Fatalf("scan top-up: %v", err)
	}
	if _, err := queries.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		CompanyID: company.ID,
		EntryType: "topup",
		AmountXaf: topUp,
	}); err != nil {
		t.Fatalf("seed top-up: %v", err)
	}

	store := NewRealAdvanceStore(queries, pool)

	var amount dbtypes.NumericString
	if err := amount.Scan("5000"); err != nil {
		t.Fatalf("scan amount: %v", err)
	}

	req, err := store.CreateAdvanceRequestWithDebit(ctx, db.CreateAdvanceRequestParams{
		CompanyID: company.ID,
		UserID:    user.ID,
		AmountXaf: amount,
		Status:    db.RequestStatusInitiated,
	})
	if err != nil {
		t.Fatalf("CreateAdvanceRequestWithDebit: %v", err)
	}
	if req.Status != db.RequestStatusInitiated {
		t.Fatalf("expected initiated, got %s", req.Status)
	}

	balance, err := queries.GetCompanyBalance(ctx, company.ID)
	if err != nil {
		t.Fatalf("GetCompanyBalance: %v", err)
	}
	bd, err := numericToDecimal(balance)
	if err != nil {
		t.Fatalf("numericToDecimal: %v", err)
	}
	if !bd.Equal(bd.Neg().Neg()) || bd.String() != "0" {
		t.Fatalf("expected balance 0 after a 5000 top-up and a 5000 debit, got %s", bd.String())
	}
}

func TestRealAdvanceStore_CreateAdvanceRequestWithDebit_ConcurrentRequestsDoNotOverdraw(t *testing.T) {
	dsn := getHandlerPG(t)
	ctx := context.Background()
	pool := newHandlerPool(t, dsn)
	queries := db.New(pool)
	company, user := seedCompanyAndUser(t, ctx, queries)

	var topUp dbtypes.NumericString
	if err := topUp.Scan("6000"); err != nil {
		t.Fatalf("scan top-up: %v", err)
	}
	if _, err := queries.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		CompanyID: company.ID,
		EntryType: "topup",
		AmountXaf: topUp,
	}); err != nil {
		t.Fatalf("seed top-up: %v", err)
	}

	store := NewRealAdvanceStore(queries, pool)

	var amount dbtypes.NumericString
	if err := amount.Scan("5000"); err != nil {
		t.Fatalf("scan amount: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := store.CreateAdvanceRequestWithDebit(ctx, db.CreateAdvanceRequestParams{
				CompanyID: company.ID,
				UserID:    user.ID,
				AmountXaf: amount,
				Status:    db.RequestStatusInitiated,
			})
			results[i] = err
		}(i)
	}
	close(start)
	wg.Wait()

	successCount, insufficientCount := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, ErrInsufficientFloat):
			insufficientCount++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successCount != 1 || insufficientCount != 1 {
		t.Fatalf("expected exactly one success and one insufficient-float rejection out of two concurrent 5000 debits against a 6000 balance, got %d successes, %d rejections", successCount, insufficientCount)
	}

	balance, err := queries.GetCompanyBalance(ctx, company.ID)
	if err != nil {
		t.Fatalf("GetCompanyBalance: %v", err)
	}
	bd, err := numericToDecimal(balance)
	if err != nil {
		t.Fatalf("numericToDecimal: %v", err)
	}
	if bd.String() != "1000" {
		t.Fatalf("expected final balance 1000 (6000 top-up - one 5000 debit), got %s — the row lock did not serialize the two debits", bd.String())
	}
}

func TestRealAdvanceStore_Transition_CommitsThroughSharedHelper(t *testing.T) {
	dsn := getHandlerPG(t)
	ctx := context.Background()
	pool := newHandlerPool(t, dsn)
	queries := db.New(pool)
	company, user := seedCompanyAndUser(t, ctx, queries)

	var amount dbtypes.NumericString
	if err := amount.Scan("5000"); err != nil {
		t.Fatalf("scan amount: %v", err)
	}
	req, err := queries.CreateAdvanceRequest(ctx, db.CreateAdvanceRequestParams{
		CompanyID: company.ID,
		UserID:    user.ID,
		AmountXaf: amount,
		Status:    db.RequestStatusInitiated,
	})
	if err != nil {
		t.Fatalf("seed request: %v", err)
	}

	store := NewRealAdvanceStore(queries, pool)
	updated, err := store.Transition(ctx, req, db.RequestStatusSuccess, service.TransitionOpts{CampayPayoutRef: "ref-1"})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if updated.Status != db.RequestStatusSuccess {
		t.Fatalf("expected success, got %s", updated.Status)
	}
}

// TestRealAdvanceStore_PassThroughMethods exercises every trivial pass-through
// method on RealAdvanceStore against real Postgres (GetUserByID,
// GetActiveRequestByUserID, GetAdvanceRequestByID, CreateEvent,
// ListAdvanceRequestsByUserID, CountAdvanceRequestsByUserToday,
// CountSuccessfulAdvanceRequestsByUserThisMonth, GetCompanyBalance) — these
// were previously untested even though the equivalent RealPlatformStore
// pass-throughs are all covered by TestIntegration_RealPlatformStore.
func TestRealAdvanceStore_PassThroughMethods(t *testing.T) {
	dsn := getHandlerPG(t)
	ctx := context.Background()
	pool := newHandlerPool(t, dsn)
	queries := db.New(pool)
	company, user := seedCompanyAndUser(t, ctx, queries)
	store := NewRealAdvanceStore(queries, pool)

	// GetUserByID
	gotUser, err := store.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if gotUser.ID != user.ID {
		t.Fatalf("GetUserByID: expected id %s, got %s", user.ID, gotUser.ID)
	}

	// GetCompanyBalance (fresh company, no ledger entries yet).
	balance, err := store.GetCompanyBalance(ctx, company.ID)
	if err != nil {
		t.Fatalf("GetCompanyBalance: %v", err)
	}
	bd, err := numericToDecimal(balance)
	if err != nil {
		t.Fatalf("numericToDecimal: %v", err)
	}
	if !bd.IsZero() {
		t.Fatalf("expected zero starting balance, got %s", bd.String())
	}

	// Create a request (still `initiated`, so it is "active").
	var topUp dbtypes.NumericString
	if err := topUp.Scan("2500"); err != nil {
		t.Fatalf("scan top-up: %v", err)
	}
	if _, err := queries.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		CompanyID: company.ID,
		EntryType: "topup",
		AmountXaf: topUp,
	}); err != nil {
		t.Fatalf("seed top-up: %v", err)
	}

	var amount dbtypes.NumericString
	if err := amount.Scan("2500"); err != nil {
		t.Fatalf("scan amount: %v", err)
	}
	req, err := store.CreateAdvanceRequestWithDebit(ctx, db.CreateAdvanceRequestParams{
		CompanyID: company.ID,
		UserID:    user.ID,
		AmountXaf: amount,
		Status:    db.RequestStatusInitiated,
	})
	if err != nil {
		t.Fatalf("CreateAdvanceRequestWithDebit: %v", err)
	}

	// GetActiveRequestByUserID
	active, err := store.GetActiveRequestByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetActiveRequestByUserID: %v", err)
	}
	if active.ID != req.ID {
		t.Fatalf("GetActiveRequestByUserID: expected %s, got %s", req.ID, active.ID)
	}

	// GetAdvanceRequestByID
	byID, err := store.GetAdvanceRequestByID(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetAdvanceRequestByID: %v", err)
	}
	if byID.ID != req.ID {
		t.Fatalf("GetAdvanceRequestByID: expected %s, got %s", req.ID, byID.ID)
	}

	// CreateEvent
	metadata, _ := json.Marshal(map[string]interface{}{"request_id": req.ID})
	event, err := store.CreateEvent(ctx, db.CreateEventParams{
		CompanyID: pgtype.UUID{Bytes: company.ID, Valid: true},
		UserID:    pgtype.UUID{Bytes: user.ID, Valid: true},
		EventType: "request_initiated",
		Metadata:  metadata,
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if event.ID == uuid.Nil {
		t.Fatal("CreateEvent: expected a non-nil event ID")
	}

	// ListAdvanceRequestsByUserID
	list, err := store.ListAdvanceRequestsByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListAdvanceRequestsByUserID: %v", err)
	}
	found := false
	for _, r := range list {
		if r.ID == req.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListAdvanceRequestsByUserID: expected to find request %s in %+v", req.ID, list)
	}

	// CountAdvanceRequestsByUserToday
	todayCount, err := store.CountAdvanceRequestsByUserToday(ctx, user.ID)
	if err != nil {
		t.Fatalf("CountAdvanceRequestsByUserToday: %v", err)
	}
	if todayCount < 1 {
		t.Fatalf("CountAdvanceRequestsByUserToday: expected at least 1, got %d", todayCount)
	}

	// CountSuccessfulAdvanceRequestsByUserThisMonth: 0 until the request
	// transitions to success.
	monthCount, err := store.CountSuccessfulAdvanceRequestsByUserThisMonth(ctx, user.ID)
	if err != nil {
		t.Fatalf("CountSuccessfulAdvanceRequestsByUserThisMonth: %v", err)
	}
	if monthCount != 0 {
		t.Fatalf("expected 0 successful requests before transition, got %d", monthCount)
	}
	if _, err := store.Transition(ctx, req, db.RequestStatusSuccess, service.TransitionOpts{CampayPayoutRef: "ref-pass-through"}); err != nil {
		t.Fatalf("Transition to success: %v", err)
	}
	monthCount, err = store.CountSuccessfulAdvanceRequestsByUserThisMonth(ctx, user.ID)
	if err != nil {
		t.Fatalf("CountSuccessfulAdvanceRequestsByUserThisMonth after success: %v", err)
	}
	if monthCount != 1 {
		t.Fatalf("expected 1 successful request after transition, got %d", monthCount)
	}
}

func TestRealAdvanceStore_ReissueAdvanceRequest_DoubleReissueReturnsUniqueViolation(t *testing.T) {
	dsn := getHandlerPG(t)
	ctx := context.Background()
	pool := newHandlerPool(t, dsn)
	queries := db.New(pool)
	company, user := seedCompanyAndUser(t, ctx, queries)

	var topUp dbtypes.NumericString
	if err := topUp.Scan("20000"); err != nil {
		t.Fatalf("scan top-up: %v", err)
	}
	if _, err := queries.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		CompanyID: company.ID,
		EntryType: "topup",
		AmountXaf: topUp,
	}); err != nil {
		t.Fatalf("seed top-up: %v", err)
	}

	var amount dbtypes.NumericString
	if err := amount.Scan("5000"); err != nil {
		t.Fatalf("scan amount: %v", err)
	}
	original, err := queries.CreateAdvanceRequest(ctx, db.CreateAdvanceRequestParams{
		CompanyID: company.ID,
		UserID:    user.ID,
		AmountXaf: amount,
		Status:    db.RequestStatusFailed,
	})
	if err != nil {
		t.Fatalf("seed original request: %v", err)
	}

	store := NewRealAdvanceStore(queries, pool)
	if _, err := store.ReissueAdvanceRequest(ctx, original); err != nil {
		t.Fatalf("first reissue: %v", err)
	}

	_, err = store.ReissueAdvanceRequest(ctx, original)
	if err == nil {
		t.Fatal("expected second reissue of the same original to fail")
	}
	if !isUniqueViolation(err) {
		t.Fatalf("expected a unique-violation error, got: %v", err)
	}
}
