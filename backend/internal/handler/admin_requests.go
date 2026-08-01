package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/reconciler"
	"github.com/Iknite-Space/bohikor2/internal/service"
)

type adminRequestsStore interface {
	GetAdvanceRequestByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	ReissueAdvanceRequest(ctx context.Context, original db.AdvanceRequest) (db.AdvanceRequest, error)
	ResolveAdvanceRequest(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error)
	Transition(ctx context.Context, req db.AdvanceRequest, newStatus db.RequestStatus, opts service.TransitionOpts) (db.AdvanceRequest, error)
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
}

// adminReconciler is the subset of *reconciler.Reconciler this handler needs.
type adminReconciler interface {
	ReconcileByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error)
}

type AdminRequestsHandler struct {
	store        adminRequestsStore
	reconciler   adminReconciler
	campayClient campayTransferer
}

func NewAdminRequestsHandler(store adminRequestsStore, rec adminReconciler, campayClient campayTransferer) *AdminRequestsHandler {
	return &AdminRequestsHandler{store: store, reconciler: rec, campayClient: campayClient}
}

// Reconcile forces an immediate Campay poll for a single advance request,
// bypassing the reconciler's normal backoff schedule.
func (h *AdminRequestsHandler) Reconcile(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_id", "invalid request id")
		return
	}

	companyID, ok := companyIDFromContext(c)
	if !ok {
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

// Resolve marks a request as reviewed after a manual check (e.g. against the
// Campay dashboard), recording the admin's note as an audit event.
func (h *AdminRequestsHandler) Resolve(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_id", "invalid request id")
		return
	}

	companyID, ok := companyIDFromContext(c)
	if !ok {
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
		if errors.Is(err, pgx.ErrNoRows) {
			JSONError(c, http.StatusConflict, "not_flagged_for_review", "this request is not currently flagged for admin review")
			return
		}
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

// Reissue re-initiates a payout for a request that terminally failed,
// creating a new advance_requests row linked via reissued_from_id.
func (h *AdminRequestsHandler) Reissue(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_id", "invalid request id")
		return
	}

	companyID, ok := companyIDFromContext(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	original, err := h.store.GetAdvanceRequestByID(ctx, id)
	if err != nil || original.CompanyID != companyID {
		JSONError(c, http.StatusNotFound, "not_found", "advance request not found")
		return
	}
	if original.Status != db.RequestStatusFailed {
		JSONError(c, http.StatusConflict, "not_reissuable", "only a terminally failed request can be reissued")
		return
	}

	user, err := h.store.GetUserByID(ctx, original.UserID)
	if err != nil {
		slog.Error("load user for reissue", "error", err, "request_id", id)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to load user")
		return
	}

	reissued, err := h.store.ReissueAdvanceRequest(ctx, original)
	if err != nil {
		if errors.Is(err, ErrInsufficientFloat) {
			JSONError(c, http.StatusForbidden, "insufficient_employer_float", "employer's available float cannot cover this reissue")
			return
		}
		if isUniqueViolation(err) {
			JSONError(c, http.StatusConflict, "already_reissued", "this request has already been reissued")
			return
		}
		slog.Error("reissue advance request", "error", err, "request_id", id)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to reissue request")
		return
	}

	amountDec, err := numericToDecimal(reissued.AmountXaf)
	if err != nil {
		slog.Error("parse reissue amount", "error", err, "request_id", reissued.ID)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to process reissue")
		return
	}

	finalReq, persisted := disburseAndResolve(ctx, h.store, h.campayClient, reissued, user.PhoneNumber.String, amountDec)

	adminIDStr := c.GetString("admin_id")
	adminID, _ := uuid.Parse(adminIDStr)
	metadata, _ := json.Marshal(map[string]interface{}{
		"original_request_id": id,
		"reissued_request_id": finalReq.ID,
		"final_status":        string(finalReq.Status),
	})
	_, _ = h.store.CreateEvent(ctx, db.CreateEventParams{
		CompanyID: pgtype.UUID{Bytes: companyID, Valid: true},
		AdminID:   pgtype.UUID{Bytes: adminID, Valid: adminID != uuid.UUID{}},
		EventType: "request_reissued",
		Metadata:  metadata,
	})

	if !persisted {
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to save reissued request")
		return
	}

	switch finalReq.Status {
	case db.RequestStatusProcessing:
		JSONSuccess(c, http.StatusAccepted, finalReq)
	case db.RequestStatusFailed:
		JSONError(c, http.StatusBadGateway, "transfer_failed", "failed to process transfer: "+finalReq.FailureReason.String)
	default:
		JSONSuccess(c, http.StatusCreated, finalReq)
	}
}
