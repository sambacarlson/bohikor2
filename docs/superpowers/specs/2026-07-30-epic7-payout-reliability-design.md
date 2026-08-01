# Epic 7 — Payout reliability & float — Design

Status: implemented
Branch: `epic-7-payout-reliability`
Related: `PLAN.md` (Epic 7 section), `docs/schema.md`, `CLAUDE.md`

## Goal

No advance-request transaction can get permanently stuck in an ambiguous state; company float
is enforced and auditable; failures are recoverable by the right actor (auto-reconciler,
employee retry, or admin action) depending on what actually happened at Campay.

## Background / research finding

PLAN.md flagged one open risk for this epic: does Campay's `GET /transaction/{reference}/`
status-lookup endpoint accept our `external_reference`, or only the `reference` Campay itself
returns when a transfer/collect is initiated? **Resolved: only Campay's own `reference`.**
Confirmed from two sources — Campay's own API docs ("a reference UUID is *returned*... use the
reference to check the transaction status", describing the value Campay hands back, not one the
caller supplies) and CamPay's official Python SDK (`CamPay/campay-python-sdk`), whose
`get_transaction_status()` example passes `"reference": "..."` with the comment "The reference
returned from campay.initCollect." `external_reference` is not accepted by this endpoint. This
confirms — rather than changes — §3's design: the no-`campay_payout_ref` branch (pure timeout,
crashed `initiated` rows) has no active-polling option and must rely solely on the
webhook-fallback fix below.

Separately, a real bug was found in the existing webhook handler while researching this: it
matches inbound webhooks only via `GetAdvanceRequestByCampayRef(wh.Reference)`. On a network
timeout during `InitiateTransfer`, no response body is ever received, so `campay_payout_ref`
stays `NULL` on that row. If Campay's webhook for that transaction arrives later anyway, it
carries `external_reference` (= our `advance_requests.id`) but the handler has no fallback
lookup by that field — the webhook is silently dropped as "unknown reference." This is fixed as
part of §3.

## 1. Shared state-machine transition helper

Three code paths mutate `advance_requests.status` and post events: `CreateRequest` (sync path),
the Campay webhook handler, and the new reconciler. All three must agree on ledger-reversal and
event-emission behavior, so this logic is extracted once into `internal/service/payout.go`:

```go
func TransitionRequest(ctx context.Context, tx pgx.Tx, req db.AdvanceRequest, newStatus db.RequestStatus, opts TransitionOpts) (db.AdvanceRequest, error)
```

Behavior:
- No-ops (returns the row unchanged) if the request is already terminal (`success`/`failed`)
  and `newStatus` doesn't change anything — this matters because the webhook, the reconciler,
  and manual admin actions can all observe/act on the same row concurrently.
- On transition to `failed`: posts a `reversal` ledger entry (reversing the original
  `payout_debit`) in the same DB transaction as the status update.
- On transition to `success` or `failed`: emits the matching `payout_success`/`payout_failed`
  event (same shape as today).
- Takes a `pgx.Tx` so callers control the transaction boundary (needed for §2's debit-in-same-tx
  requirement).

`CreateRequest`, `handleAdvanceWebhook`, and the reconciler all call this instead of hand-rolling
`UpdateAdvanceRequestStatus` + `CreateEvent` + (sometimes) a ledger write.

## 2. Ledger-gated payout

`AdvanceHandler.CreateRequest` gains a float check before calling Campay: reject with reason
`insufficient_employer_float` (403) if `GetCompanyBalance(company_id) < advance_amount`.

Request creation becomes transactional, following the existing tx pattern in
`RealPlatformStore.CreateCompanyWithSettings` (`pool.Begin` → `queries.WithTx(tx)` → work →
`tx.Commit`): in one transaction, create the `advance_requests` row (`status = initiated`) and
post a `payout_debit` ledger entry reserving the float. Commit, *then* call Campay — a DB
transaction cannot be held open across an outbound HTTP call.

Outcome mapping after the Campay call:
- `SUCCESSFUL` → `success` (unchanged from today)
- `PENDING` → `pending` (unchanged from today)
- `FAILED` → `failed` + reversal, via `TransitionRequest` (unchanged behavior, now goes through
  the shared helper)
- **New: timeout/transport error (the `InitiateTransfer` call itself errors, no response body)
  → `processing`**, with `next_retry_at` set to the first backoff interval. This replaces
  today's behavior of marking these `failed` outright — the whole point of `processing` is that
  we don't yet know whether Campay actually moved money. `campay_payout_ref` stays `NULL` on
  these rows.

## 3. Reconciler

New package `internal/reconciler/`. A background goroutine started from `server.New`, ticking
every 30s, stopped via context cancellation on the existing SIGINT/SIGTERM graceful-shutdown
path in `Server.Start`.

Each tick calls `ListReconcilable`, which returns `advance_requests` rows in three cases:
`processing`/`pending` with `next_retry_at <= now`; and `initiated` rows older than 60 seconds
(these are process-crash artifacts — the debit-and-create transaction in §2 committed, but the
server died before the Campay call ever returned, so no `campay_payout_ref` was ever set). Rows
split into two cases based on whether we have anything to poll Campay with:

**Has `campay_payout_ref`** (stuck `pending`, or `processing` where the response did come back
but a subsequent DB write failed): call the new `campay.Client.GetTransactionStatus(ctx, ref)`
(`GET /transaction/{ref}/`), map the returned status through `TransitionRequest`. This is the
reconciler's primary job and is fully grounded in verified Campay/SDK behavior — every
third-party SDK reviewed uses exactly this call shape. Backoff on repeated `PENDING`/no
resolution: `30s, 1m, 2m, 5m, 15m` (capped), `attempt_count` incremented each try; after 5
attempts, `needs_admin_review = true` and polling stops (still visible in the admin queue,
resolvable via `/reconcile` or `/resolve`).

**No `campay_payout_ref`** (pure timeout — `processing` with nothing to poll; or a crashed
`initiated` row per above): cannot be
actively resolved via this endpoint — confirmed, see Background above. Backstop is the
webhook-fallback fix:
`HandleCampayWebhook` gains a second lookup, `GetAdvanceRequestByID(uuid.Parse(wh.ExternalReference))`,
tried when `GetAdvanceRequestByCampayRef` finds nothing — so a late-arriving webhook for a
no-ref row still resolves it. The reconciler does not poll these; it only ages them: after a
10-minute grace period with no webhook, `needs_admin_review = true` directly (no repeated
attempts, since polling isn't possible for this class).

`GetTransactionStatus` unit tests use a mocked `http.RoundTripper`, matching the existing test
style in `internal/campay/client_more_test.go`.

## 4. API surface

All new admin/platform routes reuse the existing route-group + middleware pattern in
`routes.go`/`server.go` (guard already encodes the right role/tenancy check; no new gating logic
in handlers).

- `POST /api/advance-requests/:id/retry` (user, `RequireActiveUser` group) — allowed only when
  the target request is terminal `failed`, and the user has no other non-terminal request.
  Creates a new row (new `external_reference`), passes through the same eligibility + float
  checks as `CreateRequest`.
- `POST /api/admin/requests/:id/reconcile` (company admin) — forces an immediate
  `GetTransactionStatus` poll. Returns 409 with a clear reason if the row has no
  `campay_payout_ref` (nothing to poll).
- `POST /api/admin/requests/:id/resolve` (company admin) — marks a `needs_admin_review` request
  resolved after a manual Campay-dashboard check; requires a `note` in the body; audited event.
- `POST /api/admin/requests/:id/reissue` (company admin) — creates a new `advance_requests` row
  with `reissued_from_id` set to the original; posts a fresh `payout_debit`; audited. Blocked by
  a DB unique constraint on `reissued_from_id` if the original was already reissued (see §5).
- `POST /api/platform/companies/:id/ledger/adjustment` (platform admin) — mirrors the existing
  `ledger/topup` route; posts a manual `adjustment` ledger entry for corrections.
- `GetEligibility` gains `insufficient_employer_float` as a possible reason string, computed the
  same way as the `CreateRequest` check.

## 5. Schema changes

Additive changes to `backend/migrations/000001_schema.up/down.sql` (still the single greenfield
baseline per repo convention — no new numbered migration):

```sql
ALTER TABLE advance_requests
  ADD COLUMN reissued_from_id UUID UNIQUE REFERENCES advance_requests(id);
```

Rationale: an admin reissue is meant to happen at most once per failed/stuck original (the
action explicitly follows a manual "confirmed no funds moved" check). A DB-enforced unique
constraint blocks a double-reissue race (two admins acting concurrently, or a UI double-submit)
at the database level, rather than relying on an application-level check against event-log
metadata. It also makes "what replaced this failed request" a direct join for admin/support UI,
consistent with how this schema already models similar links (`company_ledger.advance_request_id`,
`advance_requests.campay_payout_ref`) rather than leaning on events for anything queryable.

`docs/schema.md` gets updated to document this column alongside the existing resilience columns
it already lists as "created now, exercised in Epic 7."

## 6. Testing

- `TransitionRequest`: table-driven over all reachable state pairs, including idempotent
  no-ops when called twice with the same target status.
- Ledger math: debit-then-reversal nets to zero; balance queries reflect concurrent
  debits/reversals correctly under a DB transaction.
- Reconciler: backoff schedule advances `next_retry_at` correctly; `attempt_count` caps at 5
  attempts before `needs_admin_review`; the no-ref branch skips polling and applies the 10-minute
  grace period instead of the backoff ladder.
- Retry/reissue guards: no second active request while one is non-terminal; reissue rejected at
  the DB constraint if already reissued.
- Webhook fallback: a webhook with only `external_reference` matching (no `campay_payout_ref` on
  the row) is now resolved instead of dropped as "unknown reference."
- `go test ./...` and `golangci-lint run` clean; `make test-cover` stays at or above the
  92.6% bar this repo already holds on the covered packages.

## Out of scope for this epic

- Frontend/admin UI for any of these new endpoints (Epic 8).
