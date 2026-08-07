package reconciler

import (
	"context"
	"encoding/json"
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

// Tick processes every currently-reconcilable row once (advance requests,
// then phone verifications). Exported so the admin force-reconcile endpoint
// and tests can drive it synchronously.
func (r *Reconciler) Tick(ctx context.Context) {
	rows, err := r.queries.ListReconcilableAdvanceRequests(ctx)
	if err != nil {
		slog.Error("list reconcilable advance requests", "error", err)
	} else {
		for _, req := range rows {
			r.reconcileOne(ctx, req)
		}
	}

	phoneRows, err := r.queries.ListReconcilablePhoneVerifications(ctx)
	if err != nil {
		slog.Error("list reconcilable phone verifications", "error", err)
		return
	}
	for _, v := range phoneRows {
		r.reconcilePhoneVerificationOne(ctx, v)
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
	slog.Info("reconciling advance request", "request_id", req.ID, "campay_ref", req.CampayPayoutRef.String)

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
		return
	}
	slog.Info("advance request reconciliation resolved", "request_id", req.ID, "status", newStatus)
}

// ---------------------------------------------------------------------------
// Phone verifications — parallels the advance-request functions above.
// Simpler than the payout path: phone verification never touches the
// company ledger, so there's no service.TransitionRequest/transaction
// needed, matching handlePhoneVerificationWebhook's (internal/handler/
// advance.go) existing plain-update pattern for this resource.
// ---------------------------------------------------------------------------

func (r *Reconciler) reconcilePhoneVerificationOne(ctx context.Context, v db.PhoneVerification) {
	if !v.CampayPayoutRef.Valid || v.CampayPayoutRef.String == "" {
		r.handlePhoneVerificationNoRef(ctx, v)
		return
	}
	r.pollAndTransitionPhoneVerification(ctx, v)
}

func (r *Reconciler) pollAndTransitionPhoneVerification(ctx context.Context, v db.PhoneVerification) {
	slog.Info("reconciling phone verification", "verification_id", v.ID, "campay_ref", v.CampayPayoutRef.String)

	status, err := r.campayClient.GetTransactionStatus(ctx, v.CampayPayoutRef.String)
	if err != nil {
		slog.Error("poll campay transaction status for phone verification", "error", err, "verification_id", v.ID)
		r.bumpPhoneVerificationAttempt(ctx, v)
		return
	}

	switch status.Status {
	case "SUCCESSFUL":
		r.transitionPhoneVerification(ctx, v, db.RequestStatusSuccess, "")
	case "FAILED":
		r.transitionPhoneVerification(ctx, v, db.RequestStatusFailed, status.Reason)
	default:
		r.bumpPhoneVerificationAttempt(ctx, v)
	}
}

func (r *Reconciler) handlePhoneVerificationNoRef(ctx context.Context, v db.PhoneVerification) {
	if time.Since(v.CreatedAt) < noRefGracePeriod {
		return
	}
	if _, err := r.queries.UpdatePhoneVerificationReconcileAttempt(ctx, db.UpdatePhoneVerificationReconcileAttemptParams{
		ID:               v.ID,
		NextRetryAt:      pgtype.Timestamptz{Valid: false},
		NeedsAdminReview: true,
	}); err != nil {
		slog.Error("flag no-ref phone verification for admin review", "error", err, "verification_id", v.ID)
	}
}

func (r *Reconciler) bumpPhoneVerificationAttempt(ctx context.Context, v db.PhoneVerification) {
	attempt := int(v.AttemptCount)
	if attempt >= len(backoffLadder) {
		if _, err := r.queries.UpdatePhoneVerificationReconcileAttempt(ctx, db.UpdatePhoneVerificationReconcileAttemptParams{
			ID:               v.ID,
			NextRetryAt:      pgtype.Timestamptz{Valid: false},
			NeedsAdminReview: true,
		}); err != nil {
			slog.Error("flag phone verification for admin review after max attempts", "error", err, "verification_id", v.ID)
		}
		return
	}

	next := time.Now().Add(backoffLadder[attempt])
	if _, err := r.queries.UpdatePhoneVerificationReconcileAttempt(ctx, db.UpdatePhoneVerificationReconcileAttemptParams{
		ID:               v.ID,
		NextRetryAt:      pgtype.Timestamptz{Time: next, Valid: true},
		NeedsAdminReview: false,
	}); err != nil {
		slog.Error("bump phone verification reconcile attempt", "error", err, "verification_id", v.ID)
	}
}

// transitionPhoneVerification mirrors handlePhoneVerificationWebhook's
// success path (SetPhoneVerified + phone_verified audit event) so a
// reconciler-driven resolution behaves identically to a late-arriving
// webhook resolving the same row.
func (r *Reconciler) transitionPhoneVerification(ctx context.Context, v db.PhoneVerification, newStatus db.RequestStatus, failureReason string) {
	var reason pgtype.Text
	if failureReason != "" {
		reason = pgtype.Text{String: failureReason, Valid: true}
	}
	updated, err := r.queries.UpdatePhoneVerificationStatus(ctx, db.UpdatePhoneVerificationStatusParams{
		ID:              v.ID,
		Status:          newStatus,
		FailureReason:   reason,
		CampayPayoutRef: v.CampayPayoutRef,
		UssdCode:        v.UssdCode,
	})
	if err != nil {
		slog.Error("reconciler transition phone verification", "error", err, "verification_id", v.ID)
		return
	}
	slog.Info("phone verification reconciliation resolved", "verification_id", v.ID, "status", newStatus)

	if newStatus != db.RequestStatusSuccess {
		return
	}

	if _, err := r.queries.SetPhoneVerified(ctx, v.UserID); err != nil {
		slog.Error("set phone verified from reconciler", "error", err, "user_id", v.UserID)
		return
	}

	eventMeta, _ := json.Marshal(map[string]interface{}{
		"verification_id": updated.ID.String(),
		"phone_number":    updated.PhoneNumber,
		"source":          "reconciler",
	})
	_, _ = r.queries.CreateEvent(ctx, db.CreateEventParams{
		CompanyID: pgtype.UUID{Bytes: updated.CompanyID, Valid: true},
		UserID:    pgtype.UUID{Bytes: updated.UserID, Valid: true},
		EventType: "phone_verified",
		Metadata:  eventMeta,
	})
}
