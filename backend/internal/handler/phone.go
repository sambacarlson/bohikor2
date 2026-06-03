package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type PhoneHandler struct {
	queries  phoneQuerier
	campay   campayTransferer
	verifAmt decimal.Decimal
}

type phoneQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	UpdatePhoneNumber(ctx context.Context, arg db.UpdatePhoneNumberParams) (db.User, error)
	GetActiveRequestByUserID(ctx context.Context, userID uuid.UUID) (db.AdvanceRequest, error)
	CreatePhoneVerification(ctx context.Context, arg db.CreatePhoneVerificationParams) (db.PhoneVerification, error)
	GetActivePhoneVerificationByUser(ctx context.Context, userID uuid.UUID) (db.PhoneVerification, error)
	GetLatestPhoneVerificationByUser(ctx context.Context, userID uuid.UUID) (db.PhoneVerification, error)
	UpdatePhoneVerificationStatus(ctx context.Context, arg db.UpdatePhoneVerificationStatusParams) (db.PhoneVerification, error)
	SetPhoneVerified(ctx context.Context, id uuid.UUID) (db.User, error)
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
}

func NewPhoneHandler(queries phoneQuerier, campay campayTransferer, verifAmt decimal.Decimal) *PhoneHandler {
	return &PhoneHandler{
		queries:  queries,
		campay:   campay,
		verifAmt: verifAmt,
	}
}

func (h *PhoneHandler) AddPhoneNumber(c *gin.Context) {
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

	var req struct {
		PhoneNumber string `json:"phone_number" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "phone_number is required")
		return
	}

	user, err := h.queries.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "user not found")
		return
	}

	if user.Status != db.UserStatusActive {
		JSONError(c, http.StatusForbidden, "account_not_active", "account is not active")
		return
	}

	_, err = h.queries.GetActiveRequestByUserID(c.Request.Context(), userID)
	if err == nil {
		JSONError(c, http.StatusConflict, "request_in_progress", "cannot change phone number while an advance request is in progress")
		return
	}

	_, err = h.queries.GetActivePhoneVerificationByUser(c.Request.Context(), userID)
	if err == nil {
		JSONError(c, http.StatusConflict, "verification_in_progress", "a phone verification is already in progress")
		return
	}

	user, err = h.queries.UpdatePhoneNumber(c.Request.Context(), db.UpdatePhoneNumberParams{
		ID:          userID,
		PhoneNumber: pgtype.Text{String: req.PhoneNumber, Valid: true},
	})
	if err != nil {
		slog.Error("update phone number", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to update phone number")
		return
	}

	verif, err := h.queries.CreatePhoneVerification(c.Request.Context(), db.CreatePhoneVerificationParams{
		UserID:      userID,
		PhoneNumber: req.PhoneNumber,
		AmountXaf:   pgtype.Numeric{Valid: true},
		Status:      db.RequestStatusInitiated,
	})
	if err != nil {
		slog.Error("create phone verification", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to create phone verification")
		return
	}

	description := "Bohikor2 phone number verification"
	transferResp, transferErr := h.campay.InitiateTransfer(c.Request.Context(), req.PhoneNumber, h.verifAmt, description, verif.ID.String())

	if transferErr != nil {
		slog.Error("campay phone verification transfer failed", "error", transferErr, "verification_id", verif.ID)

		failureReason := transferErr.Error()
		_, updateErr := h.queries.UpdatePhoneVerificationStatus(c.Request.Context(), db.UpdatePhoneVerificationStatusParams{
			ID:              verif.ID,
			Status:          db.RequestStatusFailed,
			FailureReason:   pgtype.Text{String: failureReason, Valid: true},
			CampayPayoutRef: pgtype.Text{Valid: false},
		})
		if updateErr != nil {
			slog.Error("update verification status to failed", "error", updateErr)
		}

		JSONError(c, http.StatusBadGateway, "transfer_failed", "failed to initiate phone verification: "+failureReason)
		return
	}

	var finalStatus db.RequestStatus
	campayRef := pgtype.Text{String: transferResp.Reference, Valid: true}
	if transferResp.Reference == "" {
		campayRef = pgtype.Text{Valid: false}
	}

	switch transferResp.Status {
	case "PENDING":
		finalStatus = db.RequestStatusPending
	case "SUCCESSFUL":
		finalStatus = db.RequestStatusSuccess
	default:
		finalStatus = db.RequestStatusInitiated
	}

	verif, updateErr := h.queries.UpdatePhoneVerificationStatus(c.Request.Context(), db.UpdatePhoneVerificationStatusParams{
		ID:              verif.ID,
		Status:          finalStatus,
		CampayPayoutRef: campayRef,
		FailureReason:   pgtype.Text{Valid: false},
	})
	if updateErr != nil {
		slog.Error("update verification status after transfer", "error", updateErr)
	}

	if finalStatus == db.RequestStatusSuccess {
		if _, err := h.queries.SetPhoneVerified(c.Request.Context(), userID); err != nil {
			slog.Error("set phone verified", "error", err)
		}
	}

	JSONSuccess(c, http.StatusOK, gin.H{
		"verification": verif,
		"phone_number": user.PhoneNumber,
	})
}

func (h *PhoneHandler) GetPhoneVerificationStatus(c *gin.Context) {
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

	user, err := h.queries.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "user not found")
		return
	}

	phoneNum := ""
	if user.PhoneNumber.Valid {
		phoneNum = user.PhoneNumber.String
	}

	result := gin.H{
		"phone_number":   phoneNum,
		"phone_verified": user.PhoneVerified,
		"verification":   nil,
	}

	verif, err := h.queries.GetLatestPhoneVerificationByUser(c.Request.Context(), userID)
	if err == nil {
		result["verification"] = gin.H{
			"id":         verif.ID,
			"status":     verif.Status,
			"created_at": verif.CreatedAt,
		}
	}

	JSONSuccess(c, http.StatusOK, result)
}
