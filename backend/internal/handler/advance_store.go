package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/service"
)

// ErrInsufficientFloat is returned by CreateAdvanceRequestWithDebit and
// ReissueAdvanceRequest when the company's float, checked atomically
// under a row lock, cannot cover the requested amount.
var ErrInsufficientFloat = errors.New("insufficient employer float")

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505) — used to distinguish a legitimate double-
// reissue race (blocked by the reissued_from_id UNIQUE constraint) from a
// genuine infrastructure error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

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

	if _, err := qtx.LockCompanyForFloatCheck(ctx, arg.CompanyID); err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("lock company: %w", err)
	}
	balance, err := qtx.GetCompanyBalance(ctx, arg.CompanyID)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("get company balance: %w", err)
	}
	balanceDec, err := numericToDecimal(balance)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("parse company balance: %w", err)
	}
	amountDec, err := numericToDecimal(arg.AmountXaf)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("parse amount: %w", err)
	}
	if balanceDec.LessThan(amountDec) {
		return db.AdvanceRequest{}, ErrInsufficientFloat
	}

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

	if _, err := qtx.LockCompanyForFloatCheck(ctx, original.CompanyID); err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("lock company: %w", err)
	}
	balance, err := qtx.GetCompanyBalance(ctx, original.CompanyID)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("get company balance: %w", err)
	}
	balanceDec, err := numericToDecimal(balance)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("parse company balance: %w", err)
	}
	amountDec, err := numericToDecimal(original.AmountXaf)
	if err != nil {
		return db.AdvanceRequest{}, fmt.Errorf("parse amount: %w", err)
	}
	if balanceDec.LessThan(amountDec) {
		return db.AdvanceRequest{}, ErrInsufficientFloat
	}

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
