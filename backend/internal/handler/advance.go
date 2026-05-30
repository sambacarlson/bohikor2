package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/campay"
)

type advanceQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetActiveRequestByUserID(ctx context.Context, userID uuid.UUID) (db.AdvanceRequest, error)
	CreateAdvanceRequest(ctx context.Context, arg db.CreateAdvanceRequestParams) (db.AdvanceRequest, error)
	UpdateAdvanceRequestStatus(ctx context.Context, arg db.UpdateAdvanceRequestStatusParams) (db.AdvanceRequest, error)
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
	ListAdvanceRequestsByUserID(ctx context.Context, userID uuid.UUID) ([]db.AdvanceRequest, error)
}

type campayTransferer interface {
	InitiateTransfer(ctx context.Context, phoneNumber string, amount decimal.Decimal, description string, externalRef string) (*campay.TransferResponse, error)
}

type AdvanceHandler struct {
	queries       advanceQuerier
	campayClient  campayTransferer
	advanceAmount pgtype.Numeric
}

func NewAdvanceHandler(queries advanceQuerier, campayClient campayTransferer, advanceAmount decimal.Decimal) *AdvanceHandler {
	var amount pgtype.Numeric
	if err := amount.Scan(advanceAmount.String()); err != nil {
		slog.Error("scan advance amount", "error", err)
	}
	return &AdvanceHandler{
		queries:       queries,
		campayClient:  campayClient,
		advanceAmount: amount,
	}
}

type createRequest struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
}

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

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "phone_number is required")
		return
	}

	ctx := c.Request.Context()

	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "user not found")
		return
	}

	if !user.IsTermsAccepted {
		JSONError(c, http.StatusForbidden, "terms_not_accepted", "you must accept the terms before requesting an advance")
		return
	}

	_, err = h.queries.GetActiveRequestByUserID(ctx, userID)
	if err == nil {
		JSONError(c, http.StatusConflict, "request_in_progress", "you already have an active advance request")
		return
	}

	newReq, err := h.queries.CreateAdvanceRequest(ctx, db.CreateAdvanceRequestParams{
		UserID:    userID,
		AmountXaf: h.advanceAmount,
		Status:    db.RequestStatusInitiated,
	})
	if err != nil {
		slog.Error("create advance request", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to create advance request")
		return
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"request_id": newReq.ID,
		"amount_xaf": h.advanceAmount,
	})
	_, _ = h.queries.CreateEvent(ctx, db.CreateEventParams{
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		EventType: "request_initiated",
		Metadata:  metadata,
	})

	description := "Bohikor2 salary advance"
	amountDec, err := numericToDecimal(h.advanceAmount)
	if err != nil {
		slog.Error("convert advance amount", "error", err)
		amountDec = decimal.NewFromInt(10000)
	}
	transferResp, transferErr := h.campayClient.InitiateTransfer(ctx, req.PhoneNumber, amountDec, description, newReq.ID.String())

	if transferErr != nil {
		slog.Error("campay transfer failed", "error", transferErr, "request_id", newReq.ID)

		failureReason := transferErr.Error()
		updated, updateErr := h.queries.UpdateAdvanceRequestStatus(ctx, db.UpdateAdvanceRequestStatusParams{
			ID:            newReq.ID,
			Status:        db.RequestStatusFailed,
			FailureReason: pgtype.Text{String: failureReason, Valid: true},
		})
		if updateErr != nil {
			slog.Error("update request status to failed", "error", updateErr, "request_id", newReq.ID)
		} else {
			newReq = updated
		}

		failMeta, _ := json.Marshal(map[string]interface{}{
			"request_id": newReq.ID,
			"reason":     failureReason,
		})
		_, _ = h.queries.CreateEvent(ctx, db.CreateEventParams{
			UserID:    pgtype.UUID{Bytes: userID, Valid: true},
			EventType: "payout_failed",
			Metadata:  failMeta,
		})

		JSONError(c, http.StatusBadGateway, "transfer_failed", "failed to process transfer: "+failureReason)
		return
	}

	now := time.Now()
	elapsed := int32(now.Sub(newReq.CreatedAt).Seconds())

	campayRef := pgtype.Text{String: transferResp.Reference, Valid: true}
	if transferResp.Reference == "" {
		campayRef = pgtype.Text{Valid: false}
	}

	var finalStatus db.RequestStatus
	if transferResp.Status == "PENDING" {
		finalStatus = db.RequestStatusPending
	} else {
		finalStatus = db.RequestStatusSuccess
	}

	updated, updateErr := h.queries.UpdateAdvanceRequestStatus(ctx, db.UpdateAdvanceRequestStatusParams{
		ID:                    newReq.ID,
		Status:                finalStatus,
		CampayPayoutRef:       campayRef,
		PayoutDurationSeconds: pgtype.Int4{Int32: elapsed, Valid: true},
	})
	if updateErr != nil {
		slog.Error("update request status after transfer", "error", updateErr, "request_id", newReq.ID)
	} else {
		newReq = updated
	}

	if transferResp.Status == "PENDING" {
		eventMeta, _ := json.Marshal(map[string]interface{}{
			"request_id": newReq.ID,
			"campay_ref": transferResp.Reference,
		})
		_, _ = h.queries.CreateEvent(ctx, db.CreateEventParams{
			UserID:    pgtype.UUID{Bytes: userID, Valid: true},
			EventType: "payout_pending",
			Metadata:  eventMeta,
		})
	} else {
		eventMeta, _ := json.Marshal(map[string]interface{}{
			"request_id":              newReq.ID,
			"campay_ref":              transferResp.Reference,
			"payout_duration_seconds": elapsed,
		})
		_, _ = h.queries.CreateEvent(ctx, db.CreateEventParams{
			UserID:    pgtype.UUID{Bytes: userID, Valid: true},
			EventType: "payout_success",
			Metadata:  eventMeta,
		})
	}

	JSONSuccess(c, http.StatusCreated, newReq)
}

func (h *AdvanceHandler) ListUserRequests(c *gin.Context) {
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

	requests, err := h.queries.ListAdvanceRequestsByUserID(c.Request.Context(), userID)
	if err != nil {
		slog.Error("list user requests", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to list requests")
		return
	}

	if requests == nil {
		requests = []db.AdvanceRequest{}
	}

	JSONSuccess(c, http.StatusOK, requests)
}

type adminRequestsQuerier interface {
	ListAdvanceRequestsWithUser(ctx context.Context, arg db.ListAdvanceRequestsWithUserParams) ([]db.ListAdvanceRequestsWithUserRow, error)
}

func HandleListAdminRequests(q adminRequestsQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := 1
		perPage := 20

		requests, err := q.ListAdvanceRequestsWithUser(c.Request.Context(), db.ListAdvanceRequestsWithUserParams{
			Limit:  int32(perPage),
			Offset: int32((page - 1) * perPage),
		})
		if err != nil {
			slog.Error("list admin requests", "error", err)
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to list requests")
			return
		}

		if requests == nil {
			requests = []db.ListAdvanceRequestsWithUserRow{}
		}

		JSONSuccess(c, http.StatusOK, requests)
	}
}

type webhookQuerier interface {
	GetAdvanceRequestByCampayRef(ctx context.Context, campayPayoutRef pgtype.Text) (db.AdvanceRequest, error)
	UpdateAdvanceRequestStatus(ctx context.Context, arg db.UpdateAdvanceRequestStatusParams) (db.AdvanceRequest, error)
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
}

type webhookVerifier interface {
	VerifyWebhook(token string) bool
}

type webhookHandler struct {
	queries      webhookQuerier
	campayClient webhookVerifier
}

func NewWebhookHandler(queries webhookQuerier, campayClient webhookVerifier) *webhookHandler {
	return &webhookHandler{queries: queries, campayClient: campayClient}
}

func (h *webhookHandler) HandleCampayWebhook(c *gin.Context) {
	payload, err := c.GetRawData()
	if err != nil {
		slog.Error("webhook read body", "error", err)
		JSONError(c, http.StatusBadRequest, "invalid_payload", "failed to read request body")
		return
	}

	var wh campay.WebhookPayload
	if err := json.Unmarshal(payload, &wh); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_payload", "invalid webhook body")
		return
	}

	if wh.Signature == "" {
		JSONError(c, http.StatusBadRequest, "missing_signature", "webhook body missing signature field")
		return
	}

	if !h.campayClient.VerifyWebhook(wh.Signature) {
		JSONError(c, http.StatusUnauthorized, "invalid_signature", "JWT signature verification failed")
		return
	}

	slog.Info("campay webhook received",
		"reference", wh.Reference,
		"status", wh.Status,
	)

	if wh.Reference == "" {
		JSONOK(c, http.StatusOK)
		return
	}

	existing, err := h.queries.GetAdvanceRequestByCampayRef(c.Request.Context(), pgtype.Text{String: wh.Reference, Valid: true})
	if err != nil {
		slog.Warn("webhook for unknown reference", "reference", wh.Reference)
		JSONOK(c, http.StatusOK)
		return
	}

	var newStatus db.RequestStatus
	var failureReason pgtype.Text
	switch wh.Status {
	case "SUCCESSFUL":
		newStatus = db.RequestStatusSuccess
	case "FAILED":
		newStatus = db.RequestStatusFailed
		if wh.Reason != "" && wh.Reason != "None" {
			failureReason = pgtype.Text{String: wh.Reason, Valid: true}
		}
	case "PENDING":
		newStatus = db.RequestStatusPending
	default:
		slog.Warn("unknown webhook status", "status", wh.Status, "reference", wh.Reference)
		JSONOK(c, http.StatusOK)
		return
	}

	now := time.Now()
	elapsed := int32(now.Sub(existing.CreatedAt).Seconds())

	_, err = h.queries.UpdateAdvanceRequestStatus(c.Request.Context(), db.UpdateAdvanceRequestStatusParams{
		ID:                    existing.ID,
		Status:                newStatus,
		FailureReason:         failureReason,
		PayoutDurationSeconds: pgtype.Int4{Int32: elapsed, Valid: true},
	})
	if err != nil {
		slog.Error("update request status from webhook", "error", err, "reference", wh.Reference)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to update request")
		return
	}

	userID := pgtype.UUID{Bytes: existing.UserID, Valid: true}
	eventMeta, _ := json.Marshal(map[string]interface{}{
		"request_id":              existing.ID.String(),
		"campay_ref":              wh.Reference,
		"payout_duration_seconds": elapsed,
	})
	switch newStatus {
	case db.RequestStatusSuccess:
		_, _ = h.queries.CreateEvent(c.Request.Context(), db.CreateEventParams{
			UserID:    userID,
			EventType: "payout_success",
			Metadata:  eventMeta,
		})
	case db.RequestStatusFailed:
		_, _ = h.queries.CreateEvent(c.Request.Context(), db.CreateEventParams{
			UserID:    userID,
			EventType: "payout_failed",
			Metadata:  eventMeta,
		})
	}

	JSONOK(c, http.StatusOK)
}

type userTermsQuerier interface {
	UpdateTermsAcceptance(ctx context.Context, arg db.UpdateTermsAcceptanceParams) (db.User, error)
}

func HandleAcceptTerms(q userTermsQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
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
			Version string `json:"version" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			JSONError(c, http.StatusBadRequest, "invalid_request", "version is required")
			return
		}

		now := time.Now().UTC()
		user, err := q.UpdateTermsAcceptance(c.Request.Context(), db.UpdateTermsAcceptanceParams{
			ID:              userID,
			IsTermsAccepted: true,
			TermsAcceptedAt: sql.NullTime{Time: now, Valid: true},
			TermsVersion:    pgtype.Text{String: req.Version, Valid: true},
		})
		if err != nil {
			slog.Error("accept terms", "error", err)
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to accept terms")
			return
		}

		JSONSuccess(c, http.StatusOK, user)
	}
}

func numericToDecimal(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid {
		return decimal.Zero, nil
	}
	var bi big.Int
	if n.Int != nil {
		bi.Set(n.Int)
	}
	return decimal.NewFromBigInt(&bi, n.Exp), nil
}
