package reconciler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// ErrNoPayoutRef is returned by ReconcileByID when the target row has no
// campay_payout_ref — there is nothing to poll Campay with.
var ErrNoPayoutRef = errors.New("advance request has no campay_payout_ref to poll")

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
		NextRetryAt:      pgtype.Timestamptz{Valid: false},
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
			NextRetryAt:      pgtype.Timestamptz{Valid: false},
			NeedsAdminReview: true,
		}); err != nil {
			slog.Error("flag row for admin review after max attempts", "error", err, "request_id", req.ID)
		}
		return
	}

	next := time.Now().Add(backoffLadder[attempt])
	if _, err := r.queries.UpdateAdvanceRequestReconcileAttempt(ctx, db.UpdateAdvanceRequestReconcileAttemptParams{
		ID:               req.ID,
		NextRetryAt:      pgtype.Timestamptz{Time: next, Valid: true},
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
