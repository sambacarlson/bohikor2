package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

// TransitionOpts carries the fields that vary across TransitionRequest's
// callers (CreateRequest, the Campay webhook, the reconciler).
type TransitionOpts struct {
	// FailureReason is recorded on the row and included in the emitted event
	// when newStatus is failed.
	FailureReason string
	// CampayPayoutRef overrides the row's campay_payout_ref when non-empty.
	// Left empty, the request's existing ref (if any) is preserved.
	CampayPayoutRef string
}

// TransitionRequest is the single place that mutates advance_requests.status,
// so CreateRequest, the Campay webhook handler, and the reconciler all agree
// on ledger-reversal and event-emission behavior instead of each hand-rolling
// UpdateAdvanceRequestStatus + CreateEvent + (sometimes) a ledger write.
//
// It takes a pgx.Tx so the caller controls the transaction boundary (needed
// so a debit-and-create can commit before an outbound Campay call, per the
// design's §2 ledger-gated payout).
func TransitionRequest(ctx context.Context, tx pgx.Tx, req db.AdvanceRequest, newStatus db.RequestStatus, opts TransitionOpts) (db.AdvanceRequest, error) {
	// No-op if the request is already terminal and nothing would change: the
	// webhook, the reconciler, and manual admin actions can all race to act
	// on the same row, and a duplicate terminal delivery must not double-post
	// a ledger reversal or a duplicate event.
	if isTerminalStatus(req.Status) {
		return req, nil
	}

	q := db.New(tx)

	campayRef := req.CampayPayoutRef
	if opts.CampayPayoutRef != "" {
		campayRef = pgtype.Text{String: opts.CampayPayoutRef, Valid: true}
	}

	elapsed := int32(time.Since(req.CreatedAt).Seconds())

	updateParams := db.UpdateAdvanceRequestStatusParams{
		ID:                    req.ID,
		Status:                newStatus,
		PayoutDurationSeconds: pgtype.Int4{Int32: elapsed, Valid: true},
		CampayPayoutRef:       campayRef,
	}
	if opts.FailureReason != "" {
		updateParams.FailureReason = pgtype.Text{String: opts.FailureReason, Valid: true}
	}

	updated, err := q.UpdateAdvanceRequestStatus(ctx, updateParams)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("update advance request status: %w", err)
	}

	if newStatus == db.RequestStatusFailed {
		// Reverses the payout_debit posted at request creation (§2): debit is
		// -AmountXaf, so the reversal is the same magnitude, positive.
		if _, err := q.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			CompanyID:        req.CompanyID,
			EntryType:        "reversal",
			AmountXaf:        req.AmountXaf,
			AdvanceRequestID: pgtype.UUID{Bytes: req.ID, Valid: true},
			Note:             pgtype.Text{String: "reversal for failed payout", Valid: true},
		}); err != nil {
			return db.AdvanceRequest{}, fmt.Errorf("post ledger reversal: %w", err)
		}
	}

	if newStatus == db.RequestStatusSuccess || newStatus == db.RequestStatusFailed {
		eventType := "payout_success"
		if newStatus == db.RequestStatusFailed {
			eventType = "payout_failed"
		}
		metadata, _ := json.Marshal(map[string]interface{}{
			"request_id":              updated.ID,
			"campay_ref":              updated.CampayPayoutRef.String,
			"payout_duration_seconds": elapsed,
			"reason":                  opts.FailureReason,
		})
		if _, err := q.CreateEvent(ctx, db.CreateEventParams{
			CompanyID: pgtype.UUID{Bytes: req.CompanyID, Valid: true},
			UserID:    pgtype.UUID{Bytes: req.UserID, Valid: true},
			EventType: eventType,
			Metadata:  metadata,
		}); err != nil {
			return db.AdvanceRequest{}, fmt.Errorf("post %s event: %w", eventType, err)
		}
	}

	return updated, nil
}

func isTerminalStatus(s db.RequestStatus) bool {
	return s == db.RequestStatusSuccess || s == db.RequestStatusFailed
}
