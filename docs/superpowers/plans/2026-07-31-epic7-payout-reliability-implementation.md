# Epic 7 — Payout Reliability & Float: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** No advance-request transaction can get permanently stuck in an ambiguous state; company float is enforced and auditable; failures are recoverable by the right actor (auto-reconciler, employee retry, or admin action).

**Architecture:** Everything routes through the `TransitionRequest` helper (`backend/internal/service/payout.go`, already implemented and merged) for status/ledger/event consistency. `CreateRequest` becomes ledger-gated (float check + transactional debit). A new background reconciler polls Campay for stuck rows. New admin/user endpoints cover retry, reconcile, resolve, and reissue.

**Tech Stack:** Go 1.26, Gin, sqlc (pgx/v5), Postgres, testcontainers-go for integration tests.

## Global Constraints

- Design source of truth: `docs/superpowers/specs/2026-07-30-epic7-payout-reliability-design.md` (approved).
- Migration convention: this repo has exactly one migration pair, `backend/migrations/000001_schema.up/down.sql`. Extend it in place — do **not** create a new numbered migration.
- `GET /transaction/{reference}/` accepts only Campay's own `reference`, never our `external_reference` (confirmed — see design doc background section).
- Regenerate sqlc after any `db/queries/*.sql` or schema change: `cd backend && make generate`.
- Every task must pass `make lint` and `make test` (from `backend/`) before being considered done. `make test-cover` must not drop the `internal/{handler,server,middleware,campay,service,authjwt,email,authpassword}` aggregate below ~92%.
- TDD per task: write the failing test, watch it fail for the right reason, write minimal code, watch it pass.
- Money fields (`amount_xaf`) are `pgtype.Numeric`; convert via `shopspring/decimal` (already imported in `advance.go` and `client.go`) rather than hand-rolling big.Int math.
- All new admin/platform routes reuse the existing route-group + middleware pattern in `routes.go`/`server.go` — no new gating logic inside handlers.

---

### Task 1: Schema — `reissued_from_id` column + Epic 7 sqlc queries

**Files:**
- Modify: `backend/migrations/000001_schema.up.sql` (advance_requests table, ~line 126-144)
- Modify: `backend/db/queries/advance_requests.sql`
- Modify: `docs/schema.md` (~line 149-176)
- Generated (do not hand-edit): `backend/db/sqlc/advance_requests.sql.go`, `backend/db/sqlc/models.go`

**Interfaces:**
- Produces: `db.CreateAdvanceRequestReissueParams` / `db.Queries.CreateAdvanceRequestReissue`, `db.Queries.ListReconcilableAdvanceRequests`, `db.UpdateAdvanceRequestReconcileAttemptParams` / `db.Queries.UpdateAdvanceRequestReconcileAttempt`, `db.Queries.ResolveAdvanceRequest`, and `db.AdvanceRequest.ReissuedFromID pgtype.UUID` — all consumed by Tasks 3, 5, 6, 8.

- [ ] **Step 1: Add the column to the schema**

In `backend/migrations/000001_schema.up.sql`, inside the `advance_requests` table definition, add one line after `needs_admin_review`:

```sql
CREATE TABLE advance_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    user_id UUID NOT NULL REFERENCES users(id),
    amount_xaf NUMERIC(10, 2) NOT NULL DEFAULT 10000.00
        CHECK (amount_xaf >= 100 AND amount_xaf <= 25000),
    status request_status NOT NULL DEFAULT 'initiated',
    campay_payout_ref TEXT UNIQUE,
    failure_reason TEXT,
    payout_duration_seconds INTEGER,
    -- Resilience columns (logic lands in Epic 7)
    attempt_count INT NOT NULL DEFAULT 0,
    last_reconciled_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    needs_admin_review BOOLEAN NOT NULL DEFAULT FALSE,
    reissued_from_id UUID UNIQUE REFERENCES advance_requests(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

(No change needed to `000001_schema.down.sql` — it already does a bare `DROP TABLE advance_requests`, which covers any column added to the table.)

- [ ] **Step 2: Add the new queries**

Append to `backend/db/queries/advance_requests.sql`:

```sql
-- name: CreateAdvanceRequestReissue :one
INSERT INTO advance_requests (company_id, user_id, amount_xaf, status, reissued_from_id)
VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: ListReconcilableAdvanceRequests :many
-- Rows the reconciler must act on: processing/pending past their next_retry_at,
-- or initiated rows old enough to be a process-crash artifact (debit committed,
-- server died before the Campay call returned). Excludes rows already flagged
-- for a human.
SELECT * FROM advance_requests
WHERE needs_admin_review = FALSE
  AND (
    (status IN ('processing', 'pending') AND next_retry_at IS NOT NULL AND next_retry_at <= NOW())
    OR (status = 'initiated' AND created_at <= NOW() - INTERVAL '60 seconds')
  )
ORDER BY created_at ASC;

-- name: UpdateAdvanceRequestReconcileAttempt :one
UPDATE advance_requests SET
    attempt_count = attempt_count + 1,
    last_reconciled_at = NOW(),
    next_retry_at = $2,
    needs_admin_review = $3,
    updated_at = NOW()
WHERE id = $1 RETURNING *;

-- name: ResolveAdvanceRequest :one
UPDATE advance_requests SET needs_admin_review = FALSE, updated_at = NOW()
WHERE id = $1 AND needs_admin_review = TRUE RETURNING *;
```

- [ ] **Step 3: Regenerate sqlc and verify it compiles**

Run: `cd backend && make generate && go build ./...`
Expected: no output from `make generate`, clean build. `db/sqlc/models.go`'s `AdvanceRequest` struct now has a `ReissuedFromID pgtype.UUID` field; `db/sqlc/advance_requests.sql.go` has the four new query methods/param structs.

- [ ] **Step 4: Update schema docs**

In `docs/schema.md`, replace the `advance_requests` code block (~line 152-169) to include the new column in the same position as the migration, and update the prose below it:

```sql
CREATE TABLE advance_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    user_id UUID NOT NULL REFERENCES users(id),
    amount_xaf NUMERIC(10, 2) NOT NULL DEFAULT 10000.00
        CHECK (amount_xaf >= 100 AND amount_xaf <= 25000),
    status request_status NOT NULL DEFAULT 'initiated',
    campay_payout_ref TEXT UNIQUE,
    failure_reason TEXT,
    payout_duration_seconds INTEGER,
    -- Resilience columns (Epic 7)
    attempt_count INT NOT NULL DEFAULT 0,
    last_reconciled_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    needs_admin_review BOOLEAN NOT NULL DEFAULT FALSE,
    reissued_from_id UUID UNIQUE REFERENCES advance_requests(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_advance_requests_company_id ON advance_requests (company_id);
CREATE INDEX idx_advance_requests_user_id ON advance_requests (user_id);
CREATE INDEX idx_advance_requests_status ON advance_requests (status);
```

Replace the line `- The resilience columns are created now (greenfield) but exercised in Epic 7's reconciler.` with:

```
- The resilience columns are exercised by Epic 7's reconciler (`internal/reconciler`).
- `reissued_from_id` points at the original request an admin reissue replaced. The UNIQUE
  constraint blocks a double reissue of the same original at the DB level.
```

- [ ] **Step 5: Commit**

```bash
cd backend && git add migrations/000001_schema.up.sql db/queries/advance_requests.sql db/sqlc/ ../docs/schema.md
git commit -m "feat(schema): add reissued_from_id + Epic 7 reconciler/admin queries"
```

---

### Task 2: Campay client — `GetTransactionStatus`

**Files:**
- Modify: `backend/internal/campay/client.go`
- Test: `backend/internal/campay/client_more_test.go`

**Interfaces:**
- Produces: `campay.TransactionStatusResponse{Reference, Status, Amount, Currency, Operator, Code, OperatorReference, ExternalReference, Reason string}` and `func (c *Client) GetTransactionStatus(ctx context.Context, reference string) (*TransactionStatusResponse, error)`, consumed by Task 6 (reconciler) and Task 8 (admin reconcile endpoint).

- [ ] **Step 1: Write the failing tests**

Append to `backend/internal/campay/client_more_test.go`:

```go
func TestGetTransactionStatus_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/transaction/campay-ref-123/" {
			t.Fatalf("expected path /transaction/campay-ref-123/, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"reference":"campay-ref-123","status":"SUCCESSFUL","amount":"5000","operator":"MTN"}`))
	}))
	defer ts.Close()
	client := NewClient("perm-token", ts.URL, "secret")
	resp, err := client.GetTransactionStatus(context.Background(), "campay-ref-123")
	if err != nil {
		t.Fatalf("GetTransactionStatus: %v", err)
	}
	if resp.Status != "SUCCESSFUL" {
		t.Fatalf("expected status SUCCESSFUL, got %s", resp.Status)
	}
}

func TestGetTransactionStatus_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"error"}`))
	}))
	defer ts.Close()
	client := NewClient("perm-token", ts.URL, "secret")
	if _, err := client.GetTransactionStatus(context.Background(), "ref"); err == nil {
		t.Fatal("expected an error on 500 response")
	}
}

func TestGetTransactionStatus_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer ts.Close()
	client := NewClient("perm-token", ts.URL, "secret")
	if _, err := client.GetTransactionStatus(context.Background(), "ref"); err == nil {
		t.Fatal("expected an unmarshal error")
	}
}

func TestGetTransactionStatus_TransportError(t *testing.T) {
	client := NewClient("perm-token", "http://127.0.0.1:1", "secret")
	if _, err := client.GetTransactionStatus(context.Background(), "ref"); err == nil {
		t.Fatal("expected a transport error")
	}
}
```

- [ ] **Step 2: Run and verify it fails**

Run: `cd backend && go test ./internal/campay/... -run TestGetTransactionStatus -v`
Expected: FAIL with `undefined: TransactionStatusResponse` / `client.GetTransactionStatus undefined`.

- [ ] **Step 3: Implement**

In `backend/internal/campay/client.go`, add after `type WebhookPayload struct { ... }` (~line 70):

```go
type TransactionStatusResponse struct {
	Reference         string `json:"reference"`
	Status            string `json:"status"`
	Amount            string `json:"amount,omitempty"`
	Currency          string `json:"currency,omitempty"`
	Operator          string `json:"operator,omitempty"`
	Code              string `json:"code,omitempty"`
	OperatorReference string `json:"operator_reference,omitempty"`
	ExternalReference string `json:"external_reference,omitempty"`
	Reason            string `json:"reason,omitempty"`
}
```

Add after `InitiateCollection` (before `VerifyWebhook`, ~line 200):

```go
// GetTransactionStatus polls GET /transaction/{reference}/. reference must be
// Campay's own reference (the value returned in TransferResponse/CollectResponse),
// never our external_reference — confirmed against Campay's docs and official SDK.
func (c *Client) GetTransactionStatus(ctx context.Context, reference string) (*TransactionStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/transaction/"+reference+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("create status request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.permanentToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("status request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read status response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status request failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var sr TransactionStatusResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return nil, fmt.Errorf("unmarshal status response: %w", err)
	}
	return &sr, nil
}
```

- [ ] **Step 4: Run and verify it passes**

Run: `cd backend && go test ./internal/campay/... -v`
Expected: all PASS, including the 4 new tests.

- [ ] **Step 5: Commit**

```bash
cd backend && git add internal/campay/client.go internal/campay/client_more_test.go
git commit -m "feat(campay): add GetTransactionStatus client method"
```

---

### Task 3: Ledger-gated payout in `CreateRequest`

This is the biggest task: introduces `RealAdvanceStore` (tx-wrapped debit + `Transition`), the float check, and the `processing` outcome for transport errors. It rewrites part of `advance.go` and touches many existing tests in `advance_test.go`/`advance_more_test.go`.

**Files:**
- Create: `backend/internal/handler/advance_store.go`
- Modify: `backend/internal/handler/advance.go`
- Modify: `backend/internal/handler/advance_test.go`
- Modify: `backend/internal/handler/advance_more_test.go`
- Modify: `backend/internal/server/server.go` (~line 154)
- Test (new, real Postgres): `backend/internal/handler/advance_store_test.go`

**Interfaces:**
- Consumes: `service.TransitionRequest(ctx, tx, req, newStatus, opts)` / `service.TransitionOpts{FailureReason, CampayPayoutRef string}` (Task 1, already implemented).
- Produces: `handler.NewRealAdvanceStore(queries *db.Queries, pool *pgxpool.Pool) *RealAdvanceStore` implementing the enlarged `advanceQuerier` interface (`GetCompanyBalance`, `CreateAdvanceRequestWithDebit`, `Transition`, plus the existing methods minus the now-unused direct `CreateAdvanceRequest`/`UpdateAdvanceRequestStatus` calls from `CreateRequest`). Consumed by Task 7 (retry) and `server.go`.

- [ ] **Step 1: Add numeric helpers**

In `backend/internal/handler/advance.go`, add near the bottom (after `itoa`):

```go
// numericToDecimal converts a pgtype.Numeric to decimal.Decimal for arithmetic
// and comparisons that pgtype.Numeric doesn't support directly.
func numericToDecimal(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid {
		return decimal.Zero, nil
	}
	v, err := n.Value()
	if err != nil {
		return decimal.Decimal{}, err
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(s)
}

// negateNumeric flips the sign of a positive advance amount into a ledger debit.
func negateNumeric(n pgtype.Numeric) (pgtype.Numeric, error) {
	d, err := numericToDecimal(n)
	if err != nil {
		return pgtype.Numeric{}, err
	}
	var negated pgtype.Numeric
	if err := negated.Scan(d.Neg().String()); err != nil {
		return pgtype.Numeric{}, err
	}
	return negated, nil
}
```

- [ ] **Step 2: Create `RealAdvanceStore`**

Write `backend/internal/handler/advance_store.go`:

```go
package handler

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/service"
)

// RealAdvanceStore wires the advance handler to sqlc + a pool for transactions,
// mirroring RealPlatformStore's shape in platform.go.
type RealAdvanceStore struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewRealAdvanceStore(queries *db.Queries, pool *pgxpool.Pool) *RealAdvanceStore {
	return &RealAdvanceStore{queries: queries, pool: pool}
}

func (s *RealAdvanceStore) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return s.queries.GetUserByID(ctx, id)
}

func (s *RealAdvanceStore) GetActiveRequestByUserID(ctx context.Context, userID uuid.UUID) (db.AdvanceRequest, error) {
	return s.queries.GetActiveRequestByUserID(ctx, userID)
}

func (s *RealAdvanceStore) GetAdvanceRequestByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	return s.queries.GetAdvanceRequestByID(ctx, id)
}

func (s *RealAdvanceStore) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	return s.queries.CreateEvent(ctx, arg)
}

func (s *RealAdvanceStore) ListAdvanceRequestsByUserID(ctx context.Context, userID uuid.UUID) ([]db.AdvanceRequest, error) {
	return s.queries.ListAdvanceRequestsByUserID(ctx, userID)
}

func (s *RealAdvanceStore) CountAdvanceRequestsByUserToday(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.queries.CountAdvanceRequestsByUserToday(ctx, userID)
}

func (s *RealAdvanceStore) CountSuccessfulAdvanceRequestsByUserThisMonth(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.queries.CountSuccessfulAdvanceRequestsByUserThisMonth(ctx, userID)
}

func (s *RealAdvanceStore) GetCompanyBalance(ctx context.Context, companyID uuid.UUID) (pgtype.Numeric, error) {
	return s.queries.GetCompanyBalance(ctx, companyID)
}

// CreateAdvanceRequestWithDebit creates the advance_requests row and reserves
// float with a payout_debit ledger entry in one transaction (design §2).
func (s *RealAdvanceStore) CreateAdvanceRequestWithDebit(ctx context.Context, arg db.CreateAdvanceRequestParams) (db.AdvanceRequest, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)
	req, err := qtx.CreateAdvanceRequest(ctx, arg)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("create advance request: %w", err)
	}

	debitAmount, err := negateNumeric(arg.AmountXaf)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("negate amount: %w", err)
	}
	if _, err := qtx.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		CompanyID:        arg.CompanyID,
		EntryType:        "payout_debit",
		AmountXaf:        debitAmount,
		AdvanceRequestID: pgtype.UUID{Bytes: req.ID, Valid: true},
	}); err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("post payout debit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("commit: %w", err)
	}
	return req, nil
}

// Transition wraps service.TransitionRequest in its own transaction so callers
// (CreateRequest, and later the webhook/reconciler) don't manage pgx.Tx directly.
func (s *RealAdvanceStore) Transition(ctx context.Context, req db.AdvanceRequest, newStatus db.RequestStatus, opts service.TransitionOpts) (db.AdvanceRequest, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	updated, err := service.TransitionRequest(ctx, tx, req, newStatus, opts)
	if err != nil {
		return db.AdvanceRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("commit: %w", err)
	}
	return updated, nil
}
```

- [ ] **Step 3: Write the failing integration test for `RealAdvanceStore`**

Write `backend/internal/handler/advance_store_test.go` (real Postgres, same testcontainer pattern as `internal/service/payout_test.go` — duplicated per-package since Go test helpers aren't exported across packages):

```go
package handler

import (
	"context"
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

	store := NewRealAdvanceStore(queries, pool)

	var amount pgtype.Numeric
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
	if !bd.Equal(bd.Neg().Neg()) || bd.String() != "-5000" {
		t.Fatalf("expected balance -5000 after debit, got %s", bd.String())
	}
}

func TestRealAdvanceStore_Transition_CommitsThroughSharedHelper(t *testing.T) {
	dsn := getHandlerPG(t)
	ctx := context.Background()
	pool := newHandlerPool(t, dsn)
	queries := db.New(pool)
	company, user := seedCompanyAndUser(t, ctx, queries)

	var amount pgtype.Numeric
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
```

- [ ] **Step 4: Run and verify it fails**

Run: `cd backend && go test ./internal/handler/... -run TestRealAdvanceStore -v`
Expected: FAIL — `undefined: NewRealAdvanceStore` (production code not written yet; Step 2 above already wrote it, so if you're following steps in order this actually compiles and should FAIL only if Step 2 was skipped. Since Step 2 precedes this step in this plan, run this as the confirmation step: it should now build; if any assertion fails, that's the real RED signal — e.g. before Step 2 existed, this would fail with `undefined: NewRealAdvanceStore`).

- [ ] **Step 5: Run and verify it passes**

Run: `cd backend && go test ./internal/handler/... -run TestRealAdvanceStore -v`
Expected: both PASS.

- [ ] **Step 6: Rewrite `CreateRequest` for the float check + processing outcome**

In `backend/internal/handler/advance.go`, update the `advanceQuerier` interface (~line 21-30):

```go
type advanceQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetActiveRequestByUserID(ctx context.Context, userID uuid.UUID) (db.AdvanceRequest, error)
	GetAdvanceRequestByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error)
	GetCompanyBalance(ctx context.Context, companyID uuid.UUID) (pgtype.Numeric, error)
	CreateAdvanceRequestWithDebit(ctx context.Context, arg db.CreateAdvanceRequestParams) (db.AdvanceRequest, error)
	Transition(ctx context.Context, req db.AdvanceRequest, newStatus db.RequestStatus, opts service.TransitionOpts) (db.AdvanceRequest, error)
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
	ListAdvanceRequestsByUserID(ctx context.Context, userID uuid.UUID) ([]db.AdvanceRequest, error)
	CountAdvanceRequestsByUserToday(ctx context.Context, userID uuid.UUID) (int64, error)
	CountSuccessfulAdvanceRequestsByUserThisMonth(ctx context.Context, userID uuid.UUID) (int64, error)
}
```

Add the import `"github.com/Iknite-Space/bohikor2/internal/service"` to `advance.go`'s import block.

Replace the body of `CreateRequest` from the `var amount pgtype.Numeric` line (~line 268) to the end of the function (~line 397) with:

```go
	var amount pgtype.Numeric
	if err := amount.Scan(ps.advanceAmount.String()); err != nil {
		slog.Error("scan advance amount", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to parse advance amount")
		return
	}

	balance, err := h.queries.GetCompanyBalance(ctx, user.CompanyID)
	if err != nil {
		slog.Error("get company balance", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to check company balance")
		return
	}
	balanceDec, err := numericToDecimal(balance)
	if err != nil {
		slog.Error("parse company balance", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to check company balance")
		return
	}
	if balanceDec.LessThan(ps.advanceAmount) {
		JSONError(c, http.StatusForbidden, "insufficient_employer_float", "your employer's available float cannot cover this advance right now")
		return
	}

	newReq, err := h.queries.CreateAdvanceRequestWithDebit(ctx, db.CreateAdvanceRequestParams{
		CompanyID: user.CompanyID,
		UserID:    userID,
		AmountXaf: amount,
		Status:    db.RequestStatusInitiated,
	})
	if err != nil {
		slog.Error("create advance request", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to create advance request")
		return
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"request_id": newReq.ID,
		"amount_xaf": ps.advanceAmount.String(),
	})
	_, _ = h.queries.CreateEvent(ctx, db.CreateEventParams{
		CompanyID: pgtype.UUID{Bytes: user.CompanyID, Valid: true},
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		EventType: "request_initiated",
		Metadata:  metadata,
	})

	description := "Bohikor2 salary advance"
	transferResp, transferErr := h.campayClient.InitiateTransfer(ctx, user.PhoneNumber.String, ps.advanceAmount, description, newReq.ID.String())

	if transferErr != nil && transferResp == nil {
		// Pure transport/timeout/malformed-response error: Campay's InitiateTransfer
		// only returns a non-nil response alongside an error when Campay itself
		// responded with a structured FAILED status (see client.go). No response at
		// all means we genuinely don't know whether money moved, so this goes to
		// `processing` for the reconciler to resolve — NOT `failed` — per design §2.
		slog.Error("campay transfer transport error", "error", transferErr, "request_id", newReq.ID)

		nextRetry := time.Now().Add(30 * time.Second)
		updated, transErr := h.queries.Transition(ctx, newReq, db.RequestStatusProcessing, service.TransitionOpts{
			FailureReason: transferErr.Error(),
		})
		if transErr != nil {
			slog.Error("transition to processing failed", "error", transErr, "request_id", newReq.ID)
		} else {
			newReq = updated
		}
		_ = nextRetry // set via the reconciler's ListReconcilable query (Task 6), not needed on this row yet since next_retry_at defaults via reconciler bookkeeping

		JSONSuccess(c, http.StatusAccepted, newReq)
		return
	}

	if transferErr != nil {
		// Campay responded with a structured FAILED status (transferResp != nil).
		slog.Error("campay transfer declined", "error", transferErr, "request_id", newReq.ID)

		updated, transErr := h.queries.Transition(ctx, newReq, db.RequestStatusFailed, service.TransitionOpts{
			FailureReason:   transferErr.Error(),
			CampayPayoutRef: transferResp.Reference,
		})
		if transErr != nil {
			slog.Error("transition to failed failed", "error", transErr, "request_id", newReq.ID)
		} else {
			newReq = updated
		}

		JSONError(c, http.StatusBadGateway, "transfer_failed", "failed to process transfer: "+transferErr.Error())
		return
	}

	var finalStatus db.RequestStatus
	if transferResp.Status == "PENDING" {
		finalStatus = db.RequestStatusPending
	} else {
		finalStatus = db.RequestStatusSuccess
	}

	updated, updateErr := h.queries.Transition(ctx, newReq, finalStatus, service.TransitionOpts{
		CampayPayoutRef: transferResp.Reference,
	})
	if updateErr != nil {
		slog.Error("transition after transfer failed", "error", updateErr, "request_id", newReq.ID, "campay_ref", transferResp.Reference)

		// Campay already told us this transfer succeeded or is pending — money
		// moved (or may have). Do NOT force `failed` here: TransitionRequest
		// posts a ledger reversal on any transition to failed, which would
		// silently restore float for a payout that actually went through.
		// `processing` is exactly the "we don't yet know our own bookkeeping
		// state" status this epic introduces — the reconciler (once built) or
		// the webhook fallback resolves it correctly later using the
		// surviving campay_payout_ref/external_reference. This is also safe
		// against a duplicate request: GetActiveRequestByUserID already
		// treats `processing` as an active, blocking status.
		if _, procErr := h.queries.Transition(ctx, newReq, db.RequestStatusProcessing, service.TransitionOpts{
			FailureReason:   "post-transfer DB update failed, deferred to reconciler: " + updateErr.Error(),
			CampayPayoutRef: transferResp.Reference,
		}); procErr != nil {
			slog.Error("failed to mark request as processing after transfer", "error", procErr, "request_id", newReq.ID)
		}

		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to save request")
		return
	}
	newReq = updated

	JSONSuccess(c, http.StatusCreated, newReq)
}
```

Note: `TransitionRequest` (and thus `Transition`) already emits the `payout_success`/`payout_failed` events and posts the ledger reversal on `failed` — the old inline `CreateEvent` calls for `payout_failed`/`payout_pending`/`payout_success` are gone from this function because the shared helper now owns that. `processing` and `pending` intentionally emit no event (matches `TransitionRequest`'s documented behavior — only `success`/`failed` emit).

- [ ] **Step 7: Add the `processing` request status constant reference**

Confirm `db.RequestStatusProcessing` already exists (it does — `db/sqlc/models.go:109`, generated from the `request_status` enum). No change needed here; this step is just verification: `grep -n "RequestStatusProcessing" backend/db/sqlc/models.go` should show it.

- [ ] **Step 8: Add the eligibility reason**

In `GetEligibility` (~line 401-501 of `advance.go`), after the `checkMonthlyLimit` block and before `if !user.IsTermsAccepted`, add:

```go
	balance, balErr := h.queries.GetCompanyBalance(ctx, user.CompanyID)
	if balErr != nil {
		slog.Error("get company balance for eligibility", "error", balErr)
	} else if balanceDec, err := numericToDecimal(balance); err == nil && balanceDec.LessThan(ps.advanceAmount) {
		reasons = append(reasons, "insufficient_employer_float")
		eligible = false
	}
```

- [ ] **Step 9: Update `mockAdvanceQuerier` in `advance_test.go`**

In `backend/internal/handler/advance_test.go`, add the import `"github.com/Iknite-Space/bohikor2/internal/service"` to the import block, add new fields to `mockAdvanceQuerier` (~after line 39), and add the new methods (~after line 78, replacing nothing — keep the existing `CreateAdvanceRequest`/`UpdateAdvanceRequestStatus` methods as-is, they're simply no longer called by `CreateRequest` but remain harmless):

```go
type mockAdvanceQuerier struct {
	user           *db.User
	activeRequest  *db.AdvanceRequest
	createdRequest *db.AdvanceRequest
	updatedRequest *db.AdvanceRequest
	requests       []db.AdvanceRequest
	getUserErr     error
	getActiveErr   error
	createErr      error
	updateErr      error
	listErr        error
	countToday     int64
	countThisMonth int64
	countTodayErr  error
	countMonthErr  error
	balance        pgtype.Numeric
	balanceErr     error
	byID           *db.AdvanceRequest
	byIDErr        error
	lastTransitionStatus db.RequestStatus
	lastTransitionOpts   service.TransitionOpts
}

func (m *mockAdvanceQuerier) GetCompanyBalance(ctx context.Context, companyID uuid.UUID) (pgtype.Numeric, error) {
	if m.balanceErr != nil {
		return pgtype.Numeric{}, m.balanceErr
	}
	if m.balance.Valid {
		return m.balance, nil
	}
	var big pgtype.Numeric
	_ = big.Scan("1000000000")
	return big, nil
}

func (m *mockAdvanceQuerier) GetAdvanceRequestByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	if m.byIDErr != nil {
		return db.AdvanceRequest{}, m.byIDErr
	}
	if m.byID == nil {
		return db.AdvanceRequest{}, errTestNotFound
	}
	return *m.byID, nil
}

func (m *mockAdvanceQuerier) CreateAdvanceRequestWithDebit(ctx context.Context, arg db.CreateAdvanceRequestParams) (db.AdvanceRequest, error) {
	if m.createErr != nil {
		return db.AdvanceRequest{}, m.createErr
	}
	if m.createdRequest != nil {
		return *m.createdRequest, nil
	}
	return db.AdvanceRequest{
		ID:        uuid.New(),
		CompanyID: arg.CompanyID,
		UserID:    arg.UserID,
		AmountXaf: arg.AmountXaf,
		Status:    arg.Status,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func (m *mockAdvanceQuerier) Transition(ctx context.Context, req db.AdvanceRequest, newStatus db.RequestStatus, opts service.TransitionOpts) (db.AdvanceRequest, error) {
	m.lastTransitionStatus = newStatus
	m.lastTransitionOpts = opts
	if m.updateErr != nil {
		return db.AdvanceRequest{}, m.updateErr
	}
	if m.updatedRequest != nil {
		return *m.updatedRequest, nil
	}
	req.Status = newStatus
	if opts.FailureReason != "" {
		req.FailureReason = pgtype.Text{String: opts.FailureReason, Valid: true}
	}
	if opts.CampayPayoutRef != "" {
		req.CampayPayoutRef = pgtype.Text{String: opts.CampayPayoutRef, Valid: true}
	}
	return req, nil
}
```

- [ ] **Step 10: Fix the tests whose expectations changed**

`TestCreateRequest_TransferFailed` (in `advance_test.go`, ~line 390) simulated a bare `transferErr` with **no** `transferResp` — under the new logic that's a transport error, which now maps to `processing`/202, not `failed`/502. Replace the whole function:

```go
func TestCreateRequest_TransferTransportError_MapsToProcessing(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  userID,
			IsTermsAccepted:     true,
			PhoneVerified:       true,
			PhoneNumber:         pgtype.Text{String: "+237600000000", Valid: true},
			Status:              db.UserStatusActive,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
	}
	transferMock := &mockCampayTransferer{
		transferErr: errors.New("dial tcp: i/o timeout"),
	}
	h := NewAdvanceHandler(q, transferMock, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if q.lastTransitionStatus != db.RequestStatusProcessing {
		t.Fatalf("expected transition to processing, got %s", q.lastTransitionStatus)
	}
}

func TestCreateRequest_TransferDeclinedByCampay_MapsToFailed(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  userID,
			IsTermsAccepted:     true,
			PhoneVerified:       true,
			PhoneNumber:         pgtype.Text{String: "+237600000000", Valid: true},
			Status:              db.UserStatusActive,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
	}
	transferMock := &mockCampayTransferer{
		transferResp: &campay.TransferResponse{Reference: "campay-ref-declined", Status: "FAILED", Message: "insufficient operator funds"},
		transferErr:  errors.New("transfer failed: insufficient operator funds"),
	}
	h := NewAdvanceHandler(q, transferMock, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
	if q.lastTransitionStatus != db.RequestStatusFailed {
		t.Fatalf("expected transition to failed, got %s", q.lastTransitionStatus)
	}
}
```

No other existing `TestCreateRequest_*` test sets `createErr`/`updateErr`/`createdRequest`/`updatedRequest` in a way that changes meaning — they're routed through the new mock methods unchanged (`createErr` → `CreateAdvanceRequestWithDebit`, `updateErr` → `Transition`), so `TestCreateRequest_CreateError` and `TestCreateRequest_PostTransferUpdateError` in `advance_more_test.go` need no changes.

- [ ] **Step 11: Wire `RealAdvanceStore` into `server.go`**

In `backend/internal/server/server.go`, replace line 154:

```go
	advanceHandler := handler.NewAdvanceHandler(queries, campayClient, queries, loc)
```

with:

```go
	advanceStore := handler.NewRealAdvanceStore(queries, pool)
	advanceHandler := handler.NewAdvanceHandler(advanceStore, campayClient, queries, loc)
```

- [ ] **Step 12: Run the full suite**

Run: `cd backend && make lint && make test`
Expected: lint clean, all tests pass (including the new/rewritten `TestCreateRequest_*` and `TestRealAdvanceStore_*` tests).

- [ ] **Step 13: Commit**

```bash
cd backend && git add internal/handler/advance.go internal/handler/advance_store.go internal/handler/advance_store_test.go internal/handler/advance_test.go internal/server/server.go
git commit -m "feat(advance): ledger-gated payout — float check, transactional debit, processing status"
```

---

### Task 4: Webhook — rewire to `TransitionRequest` + fallback lookup fix

**Files:**
- Modify: `backend/internal/handler/advance.go` (`webhookQuerier`, `handleAdvanceWebhook`, `HandleCampayWebhook`)
- Modify: `backend/internal/handler/advance_test.go` (`mockWebhookQuerier`)
- Modify: `backend/internal/server/server.go` (webhook handler construction, ~line 176)

**Interfaces:**
- Consumes: `RealAdvanceStore.Transition` is **not** reused here — the webhook handler gets its own store (`RealAdvanceStore` also satisfies this need since it already has `Transition`; reuse the same `advanceStore` instance built in Task 3 rather than constructing a second one).
- Produces: `webhookQuerier.Transition(ctx, req, newStatus, opts) (db.AdvanceRequest, error)`, `webhookQuerier.GetAdvanceRequestByID(ctx, id uuid.UUID) (db.AdvanceRequest, error)`.

- [ ] **Step 1: Write the failing tests**

In `backend/internal/handler/advance_test.go`, find `mockWebhookQuerier` (~line 587) and add to it (fields + methods):

```go
	byExternalRef    *db.AdvanceRequest
	byExternalRefErr error
	lastTransitionStatus db.RequestStatus
	lastTransitionOpts   service.TransitionOpts
	transitionErr        error
```

```go
func (m *mockWebhookQuerier) GetAdvanceRequestByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	if m.byExternalRefErr != nil {
		return db.AdvanceRequest{}, m.byExternalRefErr
	}
	if m.byExternalRef == nil {
		return db.AdvanceRequest{}, errTestNotFound
	}
	return *m.byExternalRef, nil
}

func (m *mockWebhookQuerier) Transition(ctx context.Context, req db.AdvanceRequest, newStatus db.RequestStatus, opts service.TransitionOpts) (db.AdvanceRequest, error) {
	m.lastTransitionStatus = newStatus
	m.lastTransitionOpts = opts
	if m.transitionErr != nil {
		return db.AdvanceRequest{}, m.transitionErr
	}
	req.Status = newStatus
	return req, nil
}
```

Add two new tests near the existing `TestWebhook_*` tests (find the file with `grep -n "^func TestWebhook_"` — they live in `advance_test.go` per the earlier `TestWebhook_SuccessStatus`/`TestWebhook_FailedStatus` results):

```go
func TestWebhook_SuccessGoesThroughTransition(t *testing.T) {
	existing := db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID, UserID: uuid.New(), Status: db.RequestStatusPending, CreatedAt: time.Now().Add(-5 * time.Second)}
	campayRef := pgtype.Text{String: "campay-ref-1", Valid: true}
	existing.CampayPayoutRef = campayRef
	q := &mockWebhookQuerier{advanceReq: &existing}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})

	r := gin.New()
	r.POST("/webhook", h.HandleCampayWebhook)
	body := `{"reference":"campay-ref-1","status":"SUCCESSFUL","signature":"tok"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/webhook", strings.NewReader(body))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if q.lastTransitionStatus != db.RequestStatusSuccess {
		t.Fatalf("expected transition to success, got %s", q.lastTransitionStatus)
	}
}

func TestWebhook_FallsBackToExternalReferenceLookup(t *testing.T) {
	requestID := uuid.New()
	existing := db.AdvanceRequest{ID: requestID, CompanyID: testCompanyID, UserID: uuid.New(), Status: db.RequestStatusProcessing, CreatedAt: time.Now().Add(-5 * time.Second)}
	q := &mockWebhookQuerier{
		advanceErr:   errTestNotFound, // GetAdvanceRequestByCampayRef finds nothing (no ref was ever persisted)
		byExternalRef: &existing,
	}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})

	r := gin.New()
	r.POST("/webhook", h.HandleCampayWebhook)
	body := `{"reference":"campay-ref-late","status":"SUCCESSFUL","signature":"tok","external_reference":"` + requestID.String() + `"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/webhook", strings.NewReader(body))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if q.lastTransitionStatus != db.RequestStatusSuccess {
		t.Fatalf("expected fallback lookup to resolve and transition to success, got %s", q.lastTransitionStatus)
	}
}
```

(Check the exact field names on `mockWebhookQuerier` — e.g. `advanceReq`/`advanceErr` — against the existing struct in `advance_test.go` before writing these; adjust field names to match rather than guessing, since the plan author did not have that struct's exact current field names in view. This is the one spot in this plan where the implementer must read the existing `mockWebhookQuerier` definition first.)

- [ ] **Step 2: Run and verify it fails**

Run: `cd backend && go test ./internal/handler/... -run TestWebhook_SuccessGoesThroughTransition -v`
Expected: FAIL — compile error (`Transition`/`GetAdvanceRequestByID` undefined on `webhookQuerier`) or, once mocks compile, a behavioral failure because `handleAdvanceWebhook` still calls `UpdateAdvanceRequestStatus` directly.

- [ ] **Step 3: Implement**

In `advance.go`, update `webhookQuerier` (~line 562-569):

```go
type webhookQuerier interface {
	GetAdvanceRequestByCampayRef(ctx context.Context, campayPayoutRef pgtype.Text) (db.AdvanceRequest, error)
	GetAdvanceRequestByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error)
	Transition(ctx context.Context, req db.AdvanceRequest, newStatus db.RequestStatus, opts service.TransitionOpts) (db.AdvanceRequest, error)
	GetPhoneVerificationByCampayRef(ctx context.Context, campayPayoutRef pgtype.Text) (db.PhoneVerification, error)
	UpdatePhoneVerificationStatus(ctx context.Context, arg db.UpdatePhoneVerificationStatusParams) (db.PhoneVerification, error)
	SetPhoneVerified(ctx context.Context, id uuid.UUID) (db.User, error)
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
}
```

Replace `HandleCampayWebhook`'s advance-lookup block (~line 618-624) to add the fallback:

```go
	campayRef := pgtype.Text{String: wh.Reference, Valid: true}

	advanceReq, advanceErr := h.queries.GetAdvanceRequestByCampayRef(c.Request.Context(), campayRef)
	if advanceErr != nil && wh.ExternalReference != "" {
		if id, parseErr := uuid.Parse(wh.ExternalReference); parseErr == nil {
			if byExtRef, extErr := h.queries.GetAdvanceRequestByID(c.Request.Context(), id); extErr == nil {
				advanceReq, advanceErr = byExtRef, nil
			}
		}
	}
	if advanceErr == nil {
		h.handleAdvanceWebhook(c, advanceReq, wh)
		return
	}
```

Replace the body of `handleAdvanceWebhook` (~line 636-697) with:

```go
func (h *webhookHandler) handleAdvanceWebhook(c *gin.Context, existing db.AdvanceRequest, wh campay.WebhookPayload) {
	var newStatus db.RequestStatus
	var failureReason string
	switch wh.Status {
	case "SUCCESSFUL":
		newStatus = db.RequestStatusSuccess
	case "FAILED":
		newStatus = db.RequestStatusFailed
		if wh.Reason != "" && wh.Reason != "None" {
			failureReason = wh.Reason
		}
	case "PENDING":
		newStatus = db.RequestStatusPending
	default:
		slog.Warn("unknown webhook status", "status", wh.Status, "reference", wh.Reference)
		JSONOK(c, http.StatusOK)
		return
	}

	if _, err := h.queries.Transition(c.Request.Context(), existing, newStatus, service.TransitionOpts{
		FailureReason:   failureReason,
		CampayPayoutRef: wh.Reference,
	}); err != nil {
		slog.Error("transition request from webhook", "error", err, "reference", wh.Reference)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to update request")
		return
	}

	JSONOK(c, http.StatusOK)
}
```

- [ ] **Step 4: Run and verify it passes**

Run: `cd backend && go test ./internal/handler/... -v 2>&1 | grep -E "FAIL|^---"`
Expected: no FAIL lines.

- [ ] **Step 5: Wire the same store into `server.go`**

In `server.go`, `NewWebhookHandler` is constructed at ~line 176 as `handler.NewWebhookHandler(queries, campayClient)`. Change to reuse the `advanceStore` built in Task 3 Step 11 (it already satisfies the new `webhookQuerier` interface — `RealAdvanceStore` needs `GetAdvanceRequestByCampayRef`, `GetPhoneVerificationByCampayRef`, `UpdatePhoneVerificationStatus`, `SetPhoneVerified` added too, since those aren't currently on `RealAdvanceStore`). Add these four delegating methods to `advance_store.go`:

```go
func (s *RealAdvanceStore) GetAdvanceRequestByCampayRef(ctx context.Context, campayPayoutRef pgtype.Text) (db.AdvanceRequest, error) {
	return s.queries.GetAdvanceRequestByCampayRef(ctx, campayPayoutRef)
}

func (s *RealAdvanceStore) GetPhoneVerificationByCampayRef(ctx context.Context, campayPayoutRef pgtype.Text) (db.PhoneVerification, error) {
	return s.queries.GetPhoneVerificationByCampayRef(ctx, campayPayoutRef)
}

func (s *RealAdvanceStore) UpdatePhoneVerificationStatus(ctx context.Context, arg db.UpdatePhoneVerificationStatusParams) (db.PhoneVerification, error) {
	return s.queries.UpdatePhoneVerificationStatus(ctx, arg)
}

func (s *RealAdvanceStore) SetPhoneVerified(ctx context.Context, id uuid.UUID) (db.User, error) {
	return s.queries.SetPhoneVerified(ctx, id)
}
```

Then in `server.go`, change:

```go
	webhookHandler := handler.NewWebhookHandler(queries, campayClient)
```

to:

```go
	webhookHandler := handler.NewWebhookHandler(advanceStore, campayClient)
```

(This line must come after `advanceStore := handler.NewRealAdvanceStore(queries, pool)` from Task 3 Step 11 — check `server.go`'s ordering and move the webhook wiring below it if needed.)

- [ ] **Step 6: Run the full suite**

Run: `cd backend && make lint && make test`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
cd backend && git add internal/handler/advance.go internal/handler/advance_store.go internal/handler/advance_test.go internal/server/server.go
git commit -m "feat(webhook): route advance webhooks through TransitionRequest, fix external_reference fallback"
```

---

### Task 5: Reconciler package

**Files:**
- Create: `backend/internal/reconciler/reconciler.go`
- Create: `backend/internal/reconciler/reconciler_test.go`

**Interfaces:**
- Consumes: `db.Queries.ListReconcilableAdvanceRequests`, `db.Queries.UpdateAdvanceRequestReconcileAttempt` (Task 1), `service.TransitionRequest` (existing), `campay.Client.GetTransactionStatus` (Task 2).
- Produces: `reconciler.New(pool *pgxpool.Pool, queries *db.Queries, campayClient StatusPoller) *Reconciler`, `func (r *Reconciler) Run(ctx context.Context)`, `func (r *Reconciler) Tick(ctx context.Context)` (exported for the admin force-reconcile endpoint in Task 7), `func (r *Reconciler) ReconcileByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error)` (also for Task 7).

- [ ] **Step 1: Write the failing tests**

Write `backend/internal/reconciler/reconciler_test.go`:

```go
package reconciler

import (
	"context"
	"database/sql"
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

	"github.com/Iknite-Space/bohikor2/internal/campay"
	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/database"
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
	var amount pgtype.Numeric
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
		NextRetryAt:      sql.NullTime{Time: time.Now().Add(-time.Minute), Valid: true},
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
			ID: req.ID, NextRetryAt: sql.NullTime{Time: time.Now().Add(-time.Minute), Valid: true}, NeedsAdminReview: false,
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
		ID: req.ID, NextRetryAt: sql.NullTime{Time: time.Now().Add(-time.Minute), Valid: true}, NeedsAdminReview: false,
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
```

- [ ] **Step 2: Run and verify it fails**

Run: `cd backend && go test ./internal/reconciler/... -v`
Expected: FAIL — package `reconciler` doesn't exist yet (`no Go files in ...` or `undefined: New`).

- [ ] **Step 3: Implement**

Write `backend/internal/reconciler/reconciler.go`:

```go
package reconciler

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/campay"
	"github.com/Iknite-Space/bohikor2/internal/service"
)

// backoffLadder is the reconciler's poll backoff for rows still unresolved
// after each attempt (design §3): 30s, 1m, 2m, 5m, 15m, capped. After
// len(backoffLadder) attempts with no resolution, the row is handed to an admin.
var backoffLadder = []time.Duration{
	30 * time.Second,
	1 * time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
}

// noRefGracePeriod is how long a processing/initiated row with no
// campay_payout_ref is left for the webhook fallback before giving up and
// flagging it for a human — there is nothing to actively poll for these rows
// (Campay's status endpoint only accepts its own reference).
const noRefGracePeriod = 10 * time.Minute

// StatusPoller is the subset of *campay.Client the reconciler needs.
type StatusPoller interface {
	GetTransactionStatus(ctx context.Context, reference string) (*campay.TransactionStatusResponse, error)
}

type Reconciler struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	campayClient StatusPoller
}

func New(pool *pgxpool.Pool, queries *db.Queries, campayClient StatusPoller) *Reconciler {
	return &Reconciler{pool: pool, queries: queries, campayClient: campayClient}
}

// Run ticks every 30s until ctx is cancelled (graceful shutdown).
func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Tick(ctx)
		}
	}
}

// Tick processes every currently-reconcilable row once. Exported so the admin
// force-reconcile endpoint and tests can drive it synchronously.
func (r *Reconciler) Tick(ctx context.Context) {
	rows, err := r.queries.ListReconcilableAdvanceRequests(ctx)
	if err != nil {
		slog.Error("list reconcilable advance requests", "error", err)
		return
	}
	for _, req := range rows {
		r.reconcileOne(ctx, req)
	}
}

// ReconcileByID forces an immediate poll of a single row, for the admin
// /reconcile endpoint. Returns an error if the row has no campay_payout_ref
// (nothing to poll).
func (r *Reconciler) ReconcileByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	req, err := r.queries.GetAdvanceRequestByID(ctx, id)
	if err != nil {
		return db.AdvanceRequest{}, err
	}
	if !req.CampayPayoutRef.Valid || req.CampayPayoutRef.String == "" {
		return db.AdvanceRequest{}, ErrNoPayoutRef
	}
	r.pollAndTransition(ctx, req)
	return r.queries.GetAdvanceRequestByID(ctx, id)
}

func (r *Reconciler) reconcileOne(ctx context.Context, req db.AdvanceRequest) {
	if !req.CampayPayoutRef.Valid || req.CampayPayoutRef.String == "" {
		r.handleNoRef(ctx, req)
		return
	}
	r.pollAndTransition(ctx, req)
}

func (r *Reconciler) pollAndTransition(ctx context.Context, req db.AdvanceRequest) {
	status, err := r.campayClient.GetTransactionStatus(ctx, req.CampayPayoutRef.String)
	if err != nil {
		slog.Error("poll campay transaction status", "error", err, "request_id", req.ID)
		r.bumpAttempt(ctx, req)
		return
	}

	switch status.Status {
	case "SUCCESSFUL":
		r.transition(ctx, req, db.RequestStatusSuccess, service.TransitionOpts{})
	case "FAILED":
		r.transition(ctx, req, db.RequestStatusFailed, service.TransitionOpts{FailureReason: status.Reason})
	default:
		r.bumpAttempt(ctx, req)
	}
}

func (r *Reconciler) handleNoRef(ctx context.Context, req db.AdvanceRequest) {
	if time.Since(req.CreatedAt) < noRefGracePeriod {
		return
	}
	if _, err := r.queries.UpdateAdvanceRequestReconcileAttempt(ctx, db.UpdateAdvanceRequestReconcileAttemptParams{
		ID:               req.ID,
		NextRetryAt:      sql.NullTime{Valid: false},
		NeedsAdminReview: true,
	}); err != nil {
		slog.Error("flag no-ref row for admin review", "error", err, "request_id", req.ID)
	}
}

func (r *Reconciler) bumpAttempt(ctx context.Context, req db.AdvanceRequest) {
	attempt := int(req.AttemptCount)
	if attempt >= len(backoffLadder) {
		if _, err := r.queries.UpdateAdvanceRequestReconcileAttempt(ctx, db.UpdateAdvanceRequestReconcileAttemptParams{
			ID:               req.ID,
			NextRetryAt:      sql.NullTime{Valid: false},
			NeedsAdminReview: true,
		}); err != nil {
			slog.Error("flag row for admin review after max attempts", "error", err, "request_id", req.ID)
		}
		return
	}

	next := time.Now().Add(backoffLadder[attempt])
	if _, err := r.queries.UpdateAdvanceRequestReconcileAttempt(ctx, db.UpdateAdvanceRequestReconcileAttemptParams{
		ID:               req.ID,
		NextRetryAt:      sql.NullTime{Time: next, Valid: true},
		NeedsAdminReview: false,
	}); err != nil {
		slog.Error("bump reconcile attempt", "error", err, "request_id", req.ID)
	}
}

func (r *Reconciler) transition(ctx context.Context, req db.AdvanceRequest, newStatus db.RequestStatus, opts service.TransitionOpts) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		slog.Error("begin reconciler transition tx", "error", err, "request_id", req.ID)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := service.TransitionRequest(ctx, tx, req, newStatus, opts); err != nil {
		slog.Error("reconciler transition", "error", err, "request_id", req.ID)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Error("commit reconciler transition", "error", err, "request_id", req.ID)
	}
}
```

Add the sentinel error near the top of the file (after the `const noRefGracePeriod` block). It's exported so the admin handler (Task 8) can match it with `errors.Is`:

```go
// ErrNoPayoutRef is returned by ReconcileByID when the target row has no
// campay_payout_ref — there is nothing to poll Campay with.
var ErrNoPayoutRef = errors.New("advance request has no campay_payout_ref to poll")
```

(add `"errors"` to the import block).

- [ ] **Step 4: Run and verify it passes**

Run: `cd backend && go test ./internal/reconciler/... -v`
Expected: all PASS.

- [ ] **Step 5: Lint**

Run: `cd backend && make lint`
Expected: 0 issues.

- [ ] **Step 6: Commit**

```bash
cd backend && git add internal/reconciler/
git commit -m "feat(reconciler): add polling reconciler with backoff and needs_admin_review escalation"
```

---

### Task 6: Wire the reconciler into `server.New`/`Start`

**Files:**
- Modify: `backend/internal/server/server.go`

**Interfaces:**
- Consumes: `reconciler.New(pool, queries, campayClient) *reconciler.Reconciler`, `(*Reconciler).Run(ctx)` (Task 5). `campayClient` (`*campay.Client`) already satisfies `reconciler.StatusPoller` since `GetTransactionStatus` was added directly to it in Task 2 — no adapter needed.
- Produces: `Server.reconciler *reconciler.Reconciler` field (so `Start`'s shutdown path can cancel its context), started as a goroutine from `New` and stopped from `Start`'s existing SIGINT/SIGTERM handler.

- [ ] **Step 1: Add the field and start the goroutine**

In `server.go`, add the import `"github.com/Iknite-Space/bohikor2/internal/reconciler"`. Add a `reconcilerCancel context.CancelFunc` field to `Server` (~line 30-35):

```go
type Server struct {
	cfg             *config.Config
	router          *gin.Engine
	http            *http.Server
	pool            *pgxpool.Pool
	reconcilerCancel context.CancelFunc
}
```

After the `advanceStore := handler.NewRealAdvanceStore(queries, pool)` line (from Task 3 Step 11), add:

```go
	reconcilerCtx, reconcilerCancel := context.WithCancel(context.Background())
	go reconciler.New(pool, queries, campayClient).Run(reconcilerCtx)
```

In the `s := &Server{...}` literal, add `reconcilerCancel: reconcilerCancel,`.

- [ ] **Step 2: Stop it on shutdown**

In `Start()`, inside the existing signal-handling goroutine (~line 196-212), right before `s.pool.Close()`, add:

```go
			s.reconcilerCancel()
```

- [ ] **Step 3: Verify the server still boots**

Run: `cd backend && make lint && make test`
Expected: `TestIntegration_ServerNew` (in `internal/server/integration_test.go`) still passes — it boots the full `server.New` wiring, which now also starts the reconciler goroutine; confirm no panic or hang (the test doesn't call `Start`/`Shutdown`, so the goroutine outlives the test process only within that test's brief lifetime, which is fine — testcontainers cleanup happens in `TestMain`).

- [ ] **Step 4: Commit**

```bash
cd backend && git add internal/server/server.go
git commit -m "feat(server): start reconciler goroutine, stop it on graceful shutdown"
```

---

### Task 7: User retry endpoint

**Files:**
- Modify: `backend/internal/handler/advance.go`
- Modify: `backend/internal/handler/advance_test.go` (or `advance_more_test.go`)
- Modify: `backend/internal/server/server.go`

**Interfaces:**
- Consumes: `advanceQuerier.GetAdvanceRequestByID` (Task 3).
- Produces: `(*AdvanceHandler).RetryRequest(c *gin.Context)`, registered at `POST /api/advance-requests/:id/retry`.

- [ ] **Step 1: Factor `CreateRequest`'s core into a shared method**

In `advance.go`, rename the existing `CreateRequest` body's logic (everything after resolving `userID` and before the function ends) into a new unexported method `processAdvanceRequest(c *gin.Context, userID uuid.UUID)`, and make `CreateRequest` a thin wrapper:

```go
func (h *AdvanceHandler) CreateRequest(c *gin.Context) {
	val, exists := c.Get("user_id")
	if !exists {
		JSONError(c, http.StatusUnauthorized, "unauthorized", "user not authenticated")
		return
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		JSONError(c, http.StatusInternalServerError, "internal_error", "invalid user ID")
		return
	}
	h.processAdvanceRequest(c, userID)
}
```

Move everything from `ctx := c.Request.Context()` through the end of the old `CreateRequest` (as rewritten in Task 3) into:

```go
func (h *AdvanceHandler) processAdvanceRequest(c *gin.Context, userID uuid.UUID) {
	ctx := c.Request.Context()
	// ... unchanged body from Task 3's rewritten CreateRequest ...
}
```

- [ ] **Step 2: Write the failing tests**

Add to `advance_more_test.go`:

```go
func TestRetryRequest_NotFound(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid), byIDErr: errTestNotFound}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.POST("/api/advance-requests/:id/retry", func(c *gin.Context) {
		setUserContext(c, uid)
		h.RetryRequest(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests/"+uuid.NewString()+"/retry", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRetryRequest_NotTerminalFailed(t *testing.T) {
	uid := uuid.New()
	target := db.AdvanceRequest{ID: uuid.New(), UserID: uid, Status: db.RequestStatusPending}
	q := &mockAdvanceQuerier{user: eligibleUser(uid), byID: &target}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.POST("/api/advance-requests/:id/retry", func(c *gin.Context) {
		setUserContext(c, uid)
		h.RetryRequest(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests/"+target.ID.String()+"/retry", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestRetryRequest_WrongUser(t *testing.T) {
	uid := uuid.New()
	target := db.AdvanceRequest{ID: uuid.New(), UserID: uuid.New(), Status: db.RequestStatusFailed}
	q := &mockAdvanceQuerier{user: eligibleUser(uid), byID: &target}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.POST("/api/advance-requests/:id/retry", func(c *gin.Context) {
		setUserContext(c, uid)
		h.RetryRequest(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests/"+target.ID.String()+"/retry", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no cross-user leak), got %d", w.Code)
	}
}

func TestRetryRequest_TerminalFailed_CreatesNewRequest(t *testing.T) {
	uid := uuid.New()
	target := db.AdvanceRequest{ID: uuid.New(), UserID: uid, Status: db.RequestStatusFailed}
	q := &mockAdvanceQuerier{user: eligibleUser(uid), byID: &target}
	transferMock := &mockCampayTransferer{transferResp: &campay.TransferResponse{Reference: "retry-ref", Status: "SUCCESSFUL"}}
	h := NewAdvanceHandler(q, transferMock, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.POST("/api/advance-requests/:id/retry", func(c *gin.Context) {
		setUserContext(c, uid)
		h.RetryRequest(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests/"+target.ID.String()+"/retry", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 3: Run and verify it fails**

Run: `cd backend && go test ./internal/handler/... -run TestRetryRequest -v`
Expected: FAIL — `h.RetryRequest undefined`.

- [ ] **Step 4: Implement `RetryRequest`**

Add to `advance.go`:

```go
// RetryRequest lets a user re-initiate a payout after a terminal failure,
// creating a new request (fresh external_reference) rather than mutating the
// failed one. Guarded by the same eligibility + float checks as CreateRequest.
func (h *AdvanceHandler) RetryRequest(c *gin.Context) {
	val, exists := c.Get("user_id")
	if !exists {
		JSONError(c, http.StatusUnauthorized, "unauthorized", "user not authenticated")
		return
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		JSONError(c, http.StatusInternalServerError, "internal_error", "invalid user ID")
		return
	}

	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_id", "invalid request id")
		return
	}

	target, err := h.queries.GetAdvanceRequestByID(c.Request.Context(), targetID)
	if err != nil || target.UserID != userID {
		JSONError(c, http.StatusNotFound, "not_found", "advance request not found")
		return
	}

	if target.Status != db.RequestStatusFailed {
		JSONError(c, http.StatusConflict, "not_retryable", "only a terminally failed request can be retried")
		return
	}

	h.processAdvanceRequest(c, userID)
}
```

Add `byID *db.AdvanceRequest` field wiring to `mockAdvanceQuerier.GetAdvanceRequestByID` if not already present from Task 3 Step 9 (it is — reuse it).

- [ ] **Step 5: Run and verify it passes**

Run: `cd backend && go test ./internal/handler/... -run TestRetryRequest -v`
Expected: all PASS.

- [ ] **Step 6: Wire the route**

In `server.go`, inside the existing `advanceGroup` block (~line 155-162), add:

```go
		advanceGroup.POST("/:id/retry", advanceHandler.RetryRequest)
```

- [ ] **Step 7: Full suite + commit**

Run: `cd backend && make lint && make test`

```bash
cd backend && git add internal/handler/advance.go internal/handler/advance_more_test.go internal/server/server.go
git commit -m "feat(advance): add user-level retry endpoint for terminally failed requests"
```

---

### Task 8: Admin actions — reconcile, resolve, reissue

**Files:**
- Create: `backend/internal/handler/admin_requests.go`
- Create: `backend/internal/handler/admin_requests_test.go`
- Modify: `backend/internal/handler/advance_store.go` (add `ReissueAdvanceRequest`, `ResolveAdvanceRequest`)
- Modify: `backend/internal/server/server.go`

**Interfaces:**
- Consumes: `db.Queries.CreateAdvanceRequestReissue`, `db.Queries.ResolveAdvanceRequest` (Task 1), `reconciler.ReconcileByID` (Task 5), `service.TransitionOpts` (existing).
- Produces: `handler.NewAdminRequestsHandler(store adminRequestsStore, reconciler adminReconciler) *AdminRequestsHandler` with `Reconcile`, `Resolve`, `Reissue` gin handlers, registered under the existing `adminAdvanceGroup`.

- [ ] **Step 1: Add store methods**

In `advance_store.go`, add:

```go
// ReissueAdvanceRequest creates a fresh advance_requests row linked to the
// original via reissued_from_id, with its own payout_debit, in one tx. The
// DB's UNIQUE constraint on reissued_from_id blocks a concurrent double reissue.
func (s *RealAdvanceStore) ReissueAdvanceRequest(ctx context.Context, original db.AdvanceRequest) (db.AdvanceRequest, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)
	reissued, err := qtx.CreateAdvanceRequestReissue(ctx, db.CreateAdvanceRequestReissueParams{
		CompanyID:      original.CompanyID,
		UserID:         original.UserID,
		AmountXaf:      original.AmountXaf,
		Status:         db.RequestStatusInitiated,
		ReissuedFromID: pgtype.UUID{Bytes: original.ID, Valid: true},
	})
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("create reissue: %w", err)
	}

	debitAmount, err := negateNumeric(original.AmountXaf)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("negate amount: %w", err)
	}
	if _, err := qtx.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
		CompanyID:        original.CompanyID,
		EntryType:        "payout_debit",
		AmountXaf:        debitAmount,
		AdvanceRequestID: pgtype.UUID{Bytes: reissued.ID, Valid: true},
		Note:             pgtype.Text{String: "reissue of " + original.ID.String(), Valid: true},
	}); err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("post reissue debit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("commit: %w", err)
	}
	return reissued, nil
}

func (s *RealAdvanceStore) ResolveAdvanceRequest(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	return s.queries.ResolveAdvanceRequest(ctx, id)
}
```

- [ ] **Step 2: Write the failing tests**

Write `backend/internal/handler/admin_requests_test.go`:

```go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/reconciler"
)

type mockAdminRequestsStore struct {
	byID          *db.AdvanceRequest
	byIDErr       error
	reissued      *db.AdvanceRequest
	reissueErr    error
	resolved      *db.AdvanceRequest
	resolveErr    error
	createdEvent  bool
}

func (m *mockAdminRequestsStore) GetAdvanceRequestByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	if m.byIDErr != nil {
		return db.AdvanceRequest{}, m.byIDErr
	}
	if m.byID == nil {
		return db.AdvanceRequest{}, errTestNotFound
	}
	return *m.byID, nil
}

func (m *mockAdminRequestsStore) ReissueAdvanceRequest(ctx context.Context, original db.AdvanceRequest) (db.AdvanceRequest, error) {
	if m.reissueErr != nil {
		return db.AdvanceRequest{}, m.reissueErr
	}
	if m.reissued != nil {
		return *m.reissued, nil
	}
	return db.AdvanceRequest{ID: uuid.New(), CompanyID: original.CompanyID, UserID: original.UserID}, nil
}

func (m *mockAdminRequestsStore) ResolveAdvanceRequest(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	if m.resolveErr != nil {
		return db.AdvanceRequest{}, m.resolveErr
	}
	if m.resolved != nil {
		return *m.resolved, nil
	}
	return db.AdvanceRequest{ID: id}, nil
}

func (m *mockAdminRequestsStore) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	m.createdEvent = true
	return db.Event{ID: uuid.New()}, nil
}

type mockAdminReconciler struct {
	result db.AdvanceRequest
	err    error
}

func (m *mockAdminReconciler) ReconcileByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	return m.result, m.err
}

func adminRequestsTestGin(store adminRequestsStore, rec adminReconciler) (*gin.Engine, *AdminRequestsHandler) {
	h := NewAdminRequestsHandler(store, rec)
	r := makeTestGin()
	r.Use(func(c *gin.Context) { c.Set("admin_id", uuid.New().String()); c.Next() })
	r.POST("/api/admin/requests/:id/reconcile", h.Reconcile)
	r.POST("/api/admin/requests/:id/resolve", h.Resolve)
	r.POST("/api/admin/requests/:id/reissue", h.Reissue)
	return r, h
}

func TestAdminReconcile_NoPayoutRef_Returns409(t *testing.T) {
	r, _ := adminRequestsTestGin(&mockAdminRequestsStore{}, &mockAdminReconciler{err: reconciler.ErrNoPayoutRef})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/reconcile", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminReconcile_Success(t *testing.T) {
	r, _ := adminRequestsTestGin(&mockAdminRequestsStore{}, &mockAdminReconciler{result: db.AdvanceRequest{ID: uuid.New(), Status: db.RequestStatusSuccess}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/reconcile", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminResolve_RequiresNote(t *testing.T) {
	r, _ := adminRequestsTestGin(&mockAdminRequestsStore{}, &mockAdminReconciler{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/resolve", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without a note, got %d", w.Code)
	}
}

func TestAdminResolve_Success(t *testing.T) {
	store := &mockAdminRequestsStore{}
	r, _ := adminRequestsTestGin(store, &mockAdminReconciler{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/resolve", strings.NewReader(`{"note":"checked campay dashboard, funds never moved"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !store.createdEvent {
		t.Fatal("expected an audit event to be created")
	}
}

func TestAdminReissue_OnlyFailedOriginal(t *testing.T) {
	original := db.AdvanceRequest{ID: uuid.New(), Status: db.RequestStatusPending}
	r, _ := adminRequestsTestGin(&mockAdminRequestsStore{byID: &original}, &mockAdminReconciler{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+original.ID.String()+"/reissue", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a non-failed original, got %d", w.Code)
	}
}

func TestAdminReissue_Success(t *testing.T) {
	original := db.AdvanceRequest{ID: uuid.New(), Status: db.RequestStatusFailed}
	store := &mockAdminRequestsStore{byID: &original}
	r, _ := adminRequestsTestGin(store, &mockAdminReconciler{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+original.ID.String()+"/reissue", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !store.createdEvent {
		t.Fatal("expected an audit event to be created")
	}
}
```

- [ ] **Step 3: Run and verify it fails**

Run: `cd backend && go test ./internal/handler/... -run TestAdmin -v`
Expected: FAIL — `AdminRequestsHandler`/`adminRequestsStore`/`adminReconciler` undefined.

- [ ] **Step 4: Implement**

Write `backend/internal/handler/admin_requests.go`:

```go
package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/reconciler"
)

type adminRequestsStore interface {
	GetAdvanceRequestByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error)
	ReissueAdvanceRequest(ctx context.Context, original db.AdvanceRequest) (db.AdvanceRequest, error)
	ResolveAdvanceRequest(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error)
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
}

// adminReconciler is the subset of *reconciler.Reconciler this handler needs.
type adminReconciler interface {
	ReconcileByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error)
}

type AdminRequestsHandler struct {
	store      adminRequestsStore
	reconciler adminReconciler
}

func NewAdminRequestsHandler(store adminRequestsStore, rec adminReconciler) *AdminRequestsHandler {
	return &AdminRequestsHandler{store: store, reconciler: rec}
}

// Every handler below loads the target row and checks its CompanyID against
// companyIDFromContext(c) (the JWT-scoped company set by RequireAdmin) before
// acting — this is the project's documented "isolation guarantee" (see
// CLAUDE.md). GetAdvanceRequestByID is a bare primary-key lookup with no
// tenant scoping, so the handler must enforce it, exactly like the sibling
// HandleListAdminRequests does via companyIDFromContext. Returns 404 (not
// 403) on a cross-tenant ID to avoid leaking whether the row exists.

func (h *AdminRequestsHandler) Reconcile(c *gin.Context) {
	companyID, ok := companyIDFromContext(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_id", "invalid request id")
		return
	}

	target, err := h.store.GetAdvanceRequestByID(c.Request.Context(), id)
	if err != nil || target.CompanyID != companyID {
		JSONError(c, http.StatusNotFound, "not_found", "advance request not found")
		return
	}

	updated, err := h.reconciler.ReconcileByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, reconciler.ErrNoPayoutRef) {
			JSONError(c, http.StatusConflict, "nothing_to_poll", "this request has no campay_payout_ref to poll")
			return
		}
		slog.Error("force reconcile", "error", err, "request_id", id)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to reconcile request")
		return
	}

	JSONSuccess(c, http.StatusOK, updated)
}

func (h *AdminRequestsHandler) Resolve(c *gin.Context) {
	companyID, ok := companyIDFromContext(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_id", "invalid request id")
		return
	}

	target, err := h.store.GetAdvanceRequestByID(c.Request.Context(), id)
	if err != nil || target.CompanyID != companyID {
		JSONError(c, http.StatusNotFound, "not_found", "advance request not found")
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "note is required")
		return
	}

	resolved, err := h.store.ResolveAdvanceRequest(c.Request.Context(), id)
	if err != nil {
		slog.Error("resolve advance request", "error", err, "request_id", id)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to resolve request")
		return
	}

	adminIDStr := c.GetString("admin_id")
	adminID, _ := uuid.Parse(adminIDStr)
	metadata, _ := json.Marshal(map[string]interface{}{"request_id": id, "note": req.Note})
	_, _ = h.store.CreateEvent(c.Request.Context(), db.CreateEventParams{
		CompanyID: pgtype.UUID{Bytes: companyID, Valid: true},
		AdminID:   pgtype.UUID{Bytes: adminID, Valid: adminID != uuid.UUID{}},
		EventType: "request_resolved",
		Metadata:  metadata,
	})

	JSONSuccess(c, http.StatusOK, resolved)
}

func (h *AdminRequestsHandler) Reissue(c *gin.Context) {
	companyID, ok := companyIDFromContext(c)
	if !ok {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_id", "invalid request id")
		return
	}

	original, err := h.store.GetAdvanceRequestByID(c.Request.Context(), id)
	if err != nil || original.CompanyID != companyID {
		JSONError(c, http.StatusNotFound, "not_found", "advance request not found")
		return
	}
	if original.Status != db.RequestStatusFailed {
		JSONError(c, http.StatusConflict, "not_reissuable", "only a terminally failed request can be reissued")
		return
	}

	reissued, err := h.store.ReissueAdvanceRequest(c.Request.Context(), original)
	if err != nil {
		slog.Error("reissue advance request", "error", err, "request_id", id)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to reissue request")
		return
	}

	adminIDStr := c.GetString("admin_id")
	adminID, _ := uuid.Parse(adminIDStr)
	metadata, _ := json.Marshal(map[string]interface{}{"original_request_id": id, "reissued_request_id": reissued.ID})
	_, _ = h.store.CreateEvent(c.Request.Context(), db.CreateEventParams{
		CompanyID: pgtype.UUID{Bytes: companyID, Valid: true},
		AdminID:   pgtype.UUID{Bytes: adminID, Valid: adminID != uuid.UUID{}},
		EventType: "request_reissued",
		Metadata:  metadata,
	})

	JSONSuccess(c, http.StatusCreated, reissued)
}
```

Add `"encoding/json"` and `"github.com/jackc/pgx/v5/pgtype"` to the import block (needed for `json.Marshal`/`pgtype.UUID`). The `"github.com/Iknite-Space/bohikor2/internal/reconciler"` import is required — it's used for `reconciler.ErrNoPayoutRef` in `Reconcile`, not just as a doc-comment reference.

- [ ] **Step 5: Run and verify it passes**

Run: `cd backend && go test ./internal/handler/... -run TestAdmin -v`
Expected: all PASS.

- [ ] **Step 6: Wire the routes**

In `server.go`, the reconciler instance needs to be reachable by this handler. Change the reconciler construction from Task 6 Step 1 to keep a reference:

```go
	reconcilerCtx, reconcilerCancel := context.WithCancel(context.Background())
	requestReconciler := reconciler.New(pool, queries, campayClient)
	go requestReconciler.Run(reconcilerCtx)
```

Then, inside the existing `adminAdvanceGroup` block (~line 164-169), add:

```go
	adminRequestsHandler := handler.NewAdminRequestsHandler(advanceStore, requestReconciler)
	adminAdvanceGroup.POST("/:id/reconcile", adminRequestsHandler.Reconcile)
	adminAdvanceGroup.POST("/:id/resolve", adminRequestsHandler.Resolve)
	adminAdvanceGroup.POST("/:id/reissue", adminRequestsHandler.Reissue)
```

- [ ] **Step 7: Full suite + commit**

Run: `cd backend && make lint && make test`

```bash
cd backend && git add internal/handler/admin_requests.go internal/handler/admin_requests_test.go internal/handler/advance_store.go internal/server/server.go
git commit -m "feat(admin): add reconcile/resolve/reissue endpoints for stuck advance requests"
```

---

### Task 9: Platform ledger adjustment endpoint

**Files:**
- Modify: `backend/internal/handler/platform.go`
- Modify: `backend/internal/handler/platform_test.go` (or `platform_more_test.go`)
- Modify: `backend/internal/server/server.go`

**Interfaces:**
- Consumes: `platformStore.CreateLedgerEntry`, `platformStore.GetCompanyByID`, `platformStore.GetCompanyBalance` (all already on the interface).
- Produces: `(*PlatformHandler).AdjustCompanyLedger(c *gin.Context)`, registered at `POST /api/platform/companies/:id/ledger/adjustment`.

- [ ] **Step 1: Write the failing test**

Add to `backend/internal/handler/platform_test.go`, mirroring the existing `TestTopUpCompany_*` tests (check that file for the exact mock store field names before writing — it already has a `mockPlatformStore`-style fake from the `TopUpCompany` tests; reuse it):

```go
func TestAdjustCompanyLedger_PostsSignedEntry(t *testing.T) {
	companyID := uuid.New()
	store := &mockPlatformStore{company: &db.Company{ID: companyID}}
	h := NewPlatformHandler(store, &fakeHasher{})
	r := makeTestGin()
	r.POST("/api/platform/companies/:id/ledger/adjustment", func(c *gin.Context) {
		c.Set("platform_admin_id", uuid.New().String())
		h.AdjustCompanyLedger(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/platform/companies/"+companyID.String()+"/ledger/adjustment",
		strings.NewReader(`{"amount_xaf":"-1500","note":"correcting duplicate topup"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if store.lastLedgerEntry.EntryType != "adjustment" {
		t.Fatalf("expected entry_type adjustment, got %s", store.lastLedgerEntry.EntryType)
	}
}

func TestAdjustCompanyLedger_RejectsZero(t *testing.T) {
	companyID := uuid.New()
	store := &mockPlatformStore{company: &db.Company{ID: companyID}}
	h := NewPlatformHandler(store, &fakeHasher{})
	r := makeTestGin()
	r.POST("/api/platform/companies/:id/ledger/adjustment", func(c *gin.Context) {
		c.Set("platform_admin_id", uuid.New().String())
		h.AdjustCompanyLedger(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/platform/companies/"+companyID.String()+"/ledger/adjustment",
		strings.NewReader(`{"amount_xaf":"0","note":"noop"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a zero adjustment, got %d", w.Code)
	}
}
```

(Read the existing `mockPlatformStore` and hasher fake used by `TestTopUpCompany_*` first — reuse its exact field names, e.g. `lastLedgerEntry`, instead of inventing new ones, so this test fits the existing fake rather than requiring changes to it. Note: unlike `TopUpCompany`, an adjustment can be **negative** — do not reuse `isNonPositive`, which rejects negatives; a new check must only reject exactly zero.)

- [ ] **Step 2: Run and verify it fails**

Run: `cd backend && go test ./internal/handler/... -run TestAdjustCompanyLedger -v`
Expected: FAIL — `h.AdjustCompanyLedger undefined`.

- [ ] **Step 3: Implement**

Add to `platform.go`, after `TopUpCompany`:

```go
// AdjustCompanyLedger posts a manual, signed `adjustment` ledger entry
// (platform-admin only) for corrections — unlike TopUpCompany, the amount may
// be negative.
func (h *PlatformHandler) AdjustCompanyLedger(c *gin.Context) {
	companyID, ok := parseCompanyID(c)
	if !ok {
		return
	}

	var req struct {
		AmountXaf string `json:"amount_xaf" binding:"required"`
		Note      string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "amount_xaf and note are required")
		return
	}

	var amount pgtype.Numeric
	if err := amount.Scan(req.AmountXaf); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_amount", "amount_xaf must be a valid number")
		return
	}
	if amount.Int == nil || amount.Int.Sign() == 0 {
		JSONError(c, http.StatusBadRequest, "invalid_amount", "amount_xaf must not be zero")
		return
	}

	if _, err := h.store.GetCompanyByID(c.Request.Context(), companyID); err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "company not found")
		return
	}

	entry, err := h.store.CreateLedgerEntry(c.Request.Context(), db.CreateLedgerEntryParams{
		CompanyID: companyID,
		EntryType: "adjustment",
		AmountXaf: amount,
		CreatedBy: platformAdminUUID(c),
		Note:      pgtype.Text{String: req.Note, Valid: true},
	})
	if err != nil {
		slog.Error("create adjustment ledger entry", "error", err, "company_id", companyID)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to post adjustment")
		return
	}

	balance, ok := h.balanceOrFail(c, companyID)
	if !ok {
		return
	}

	JSONSuccess(c, http.StatusCreated, gin.H{
		"entry":       entry,
		"balance_xaf": numericToString(balance),
	})
}
```

- [ ] **Step 4: Run and verify it passes**

Run: `cd backend && go test ./internal/handler/... -run TestAdjustCompanyLedger -v`
Expected: all PASS.

- [ ] **Step 5: Wire the route**

In `server.go`, inside `platformGroup` (~line 127-134), add:

```go
		platformGroup.POST("/companies/:id/ledger/adjustment", platformHandler.AdjustCompanyLedger)
```

- [ ] **Step 6: Full suite + commit**

Run: `cd backend && make lint && make test`

```bash
cd backend && git add internal/handler/platform.go internal/handler/platform_test.go internal/server/server.go
git commit -m "feat(platform): add manual ledger adjustment endpoint"
```

---

### Task 10: Final verification and docs

**Files:**
- Modify: `PLAN.md` (Epic 7 section)
- Modify: `docs/superpowers/specs/2026-07-30-epic7-payout-reliability-design.md` (status line)

- [ ] **Step 1: Run the full backend verification suite**

Run: `cd backend && make lint && make test && make test-cover`
Expected: 0 lint issues, all tests pass, aggregate coverage over `internal/{handler,server,middleware,campay,service,authjwt,email,authpassword}` at or above ~92%.

- [ ] **Step 2: Manually sanity-check the route graph**

Run: `cd backend && go run cmd/server/main.go &` (needs a reachable Postgres; if unavailable locally, skip this step and note it) then `curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/health` — expect `200`. Stop the server afterward (`kill %1`).

- [ ] **Step 3: Update `PLAN.md`**

In `/Users/carlson/space/bohikor2/PLAN.md`, at the top of the Epic 7 section (~line 150), add a status line matching the Epic 6 convention elsewhere in the doc: `**Status: done** — see docs/superpowers/plans/2026-07-31-epic7-payout-reliability-implementation.md.`

- [ ] **Step 4: Update the design doc status**

In `docs/superpowers/specs/2026-07-30-epic7-payout-reliability-design.md`, change `Status: approved` (line 3) to `Status: implemented`.

- [ ] **Step 5: Commit**

```bash
git add PLAN.md docs/superpowers/specs/2026-07-30-epic7-payout-reliability-design.md
git commit -m "docs: mark Epic 7 (payout reliability) implemented"
```

- [ ] **Step 6: Push**

```bash
git push
```
