package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
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
	CountAdvanceRequestsByUserToday(ctx context.Context, userID uuid.UUID) (int64, error)
	CountSuccessfulAdvanceRequestsByUserThisMonth(ctx context.Context, userID uuid.UUID) (int64, error)
}

type advanceSettingsQuerier interface {
	GetSetting(ctx context.Context, key string) (db.Setting, error)
	ListSettings(ctx context.Context) ([]db.Setting, error)
}

type campayTransferer interface {
	InitiateTransfer(ctx context.Context, phoneNumber string, amount decimal.Decimal, description string, externalRef string) (*campay.TransferResponse, error)
	InitiateCollection(ctx context.Context, phoneNumber string, amount decimal.Decimal, description string, externalRef string) (*campay.CollectResponse, error)
}

type AdvanceHandler struct {
	queries      advanceQuerier
	campayClient campayTransferer
	settings     advanceSettingsQuerier
	loc          *time.Location
}

func NewAdvanceHandler(queries advanceQuerier, campayClient campayTransferer, settings advanceSettingsQuerier, loc *time.Location) *AdvanceHandler {
	return &AdvanceHandler{
		queries:      queries,
		campayClient: campayClient,
		settings:     settings,
		loc:          loc,
	}
}

type parsedSettings struct {
	killSwitchEnabled bool
	windowStartDay    int
	windowEndDay      int
	dailyLimit        int
	monthlyLimit      int
	advanceAmount     decimal.Decimal
}

// parseJSONFloat tries native JSON number parsing first, then string fallback.
// Handles both correctly-stored numbers and legacy string-encoded values.
func parseJSONFloat(raw []byte) (float64, bool) {
	var v float64
	if json.Unmarshal(raw, &v) == nil {
		return v, true
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// parseJSONBool tries native JSON bool parsing first, then string fallback
// (e.g. "true"/"false" stored as JSON string by legacy admin UI).
func parseJSONBool(raw []byte) (bool, bool) {
	var v bool
	if json.Unmarshal(raw, &v) == nil {
		return v, true
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if b, err := strconv.ParseBool(s); err == nil {
			return b, true
		}
	}
	return false, false
}

// loadSettings reads all settings from the DB. Missing or unparseable keys
// fall back to defaults (advanceAmount=10000, limits=0/unlimited).
func (h *AdvanceHandler) loadSettings(ctx context.Context) (*parsedSettings, error) {
	all, err := h.settings.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	ps := &parsedSettings{
		advanceAmount: decimal.NewFromInt(10000),
	}
	for _, s := range all {
		switch s.Key {
		case "kill_switch_enabled":
			if v, ok := parseJSONBool(s.Value); ok {
				ps.killSwitchEnabled = v
			}
		case "request_window_start_day":
			if v, ok := parseJSONFloat(s.Value); ok {
				ps.windowStartDay = int(v)
			}
		case "request_window_end_day":
			if v, ok := parseJSONFloat(s.Value); ok {
				ps.windowEndDay = int(v)
			}
		case "daily_request_limit":
			if v, ok := parseJSONFloat(s.Value); ok {
				ps.dailyLimit = int(v)
			}
		case "monthly_request_limit":
			if v, ok := parseJSONFloat(s.Value); ok {
				ps.monthlyLimit = int(v)
			}
		case "advance_amount_xaf":
			if v, ok := parseJSONFloat(s.Value); ok {
				ps.advanceAmount = decimal.NewFromFloat(v)
			}
		}
	}
	return ps, nil
}

func (h *AdvanceHandler) checkKillSwitch(ps *parsedSettings) string {
	if ps.killSwitchEnabled {
		return "advance requests are currently disabled by the system administrator"
	}
	return ""
}

func (h *AdvanceHandler) checkRequestWindow(ps *parsedSettings) string {
	now := time.Now().In(h.loc)
	day := now.Day()
	daysInMonth := daysIn(now.Year(), now.Month())

	endDay := ps.windowEndDay
	if endDay <= 0 || endDay > daysInMonth {
		endDay = daysInMonth
	}

	if day < ps.windowStartDay || day > endDay {
		return "advance requests are only accepted between the " + ordinal(ps.windowStartDay) + " and " + ordinal(endDay) + " of the month"
	}
	return ""
}

func (h *AdvanceHandler) checkDailyLimit(ctx context.Context, ps *parsedSettings, userID uuid.UUID) (string, int, error) {
	if ps.dailyLimit <= 0 {
		return "", 0, nil
	}
	count, err := h.queries.CountAdvanceRequestsByUserToday(ctx, userID)
	if err != nil {
		return "", 0, err
	}
	remaining := ps.dailyLimit - int(count)
	if remaining <= 0 {
		return "daily request limit reached (" + itoa(ps.dailyLimit) + "/" + itoa(ps.dailyLimit) + ")", 0, nil
	}
	return "", remaining, nil
}

func (h *AdvanceHandler) checkMonthlyLimit(ctx context.Context, ps *parsedSettings, userID uuid.UUID) (string, int, error) {
	if ps.monthlyLimit <= 0 {
		return "", 0, nil
	}
	count, err := h.queries.CountSuccessfulAdvanceRequestsByUserThisMonth(ctx, userID)
	if err != nil {
		return "", 0, err
	}
	remaining := ps.monthlyLimit - int(count)
	if remaining <= 0 {
		return "monthly request limit reached (" + itoa(ps.monthlyLimit) + "/" + itoa(ps.monthlyLimit) + ")", 0, nil
	}
	return "", remaining, nil
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

	ctx := c.Request.Context()

	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "user not found")
		return
	}

	if user.Status != db.UserStatusActive {
		JSONError(c, http.StatusForbidden, "account_not_active", "account is not active")
		return
	}

	if !user.IsTermsAccepted {
		JSONError(c, http.StatusForbidden, "terms_not_accepted", "you must accept the terms before requesting an advance")
		return
	}

	if !user.PhoneVerified || !user.PhoneNumber.Valid || user.PhoneNumber.String == "" {
		JSONError(c, http.StatusForbidden, "phone_not_verified", "you must verify your phone number before requesting an advance")
		return
	}

	_, err = h.queries.GetActiveRequestByUserID(ctx, userID)
	if err == nil {
		JSONError(c, http.StatusConflict, "request_in_progress", "you already have an active advance request")
		return
	}

	ps, err := h.loadSettings(ctx)
	if err != nil {
		slog.Error("load settings", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to load settings")
		return
	}

	if msg := h.checkKillSwitch(ps); msg != "" {
		JSONError(c, http.StatusForbidden, "kill_switch_active", msg)
		return
	}

	if msg := h.checkRequestWindow(ps); msg != "" {
		JSONError(c, http.StatusForbidden, "outside_request_window", msg)
		return
	}

	if msg, _, err := h.checkDailyLimit(ctx, ps, userID); err != nil {
		slog.Error("check daily limit", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to check daily limit")
		return
	} else if msg != "" {
		JSONError(c, http.StatusForbidden, "daily_limit_reached", msg)
		return
	}

	if msg, _, err := h.checkMonthlyLimit(ctx, ps, userID); err != nil {
		slog.Error("check monthly limit", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to check monthly limit")
		return
	} else if msg != "" {
		JSONError(c, http.StatusForbidden, "monthly_limit_reached", msg)
		return
	}

	var amount pgtype.Numeric
	if err := amount.Scan(ps.advanceAmount.String()); err != nil {
		slog.Error("scan advance amount", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to parse advance amount")
		return
	}

	newReq, err := h.queries.CreateAdvanceRequest(ctx, db.CreateAdvanceRequestParams{
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
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		EventType: "request_initiated",
		Metadata:  metadata,
	})

	description := "Bohikor2 salary advance"
	transferResp, transferErr := h.campayClient.InitiateTransfer(ctx, user.PhoneNumber.String, ps.advanceAmount, description, newReq.ID.String())

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
		slog.Error("update request status after transfer failed", "error", updateErr, "request_id", newReq.ID, "campay_ref", transferResp.Reference)

		_, failErr := h.queries.UpdateAdvanceRequestStatus(ctx, db.UpdateAdvanceRequestStatusParams{
			ID:                    newReq.ID,
			Status:                db.RequestStatusFailed,
			FailureReason:         pgtype.Text{String: "post-transfer DB update failed: " + updateErr.Error(), Valid: true},
			CampayPayoutRef:       campayRef,
			PayoutDurationSeconds: pgtype.Int4{Valid: false},
		})
		if failErr != nil {
			slog.Error("failed to mark request as failed after transfer", "error", failErr, "request_id", newReq.ID)
		}

		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to save request")
		return
	}
	newReq = updated

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

// GetEligibility evaluates all pre-conditions (kill switch, request window,
// daily/monthly limits, terms, phone, active request) and returns status + reasons.
func (h *AdvanceHandler) GetEligibility(c *gin.Context) {
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

	ctx := c.Request.Context()

	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "user not found")
		return
	}

	ps, err := h.loadSettings(ctx)
	if err != nil {
		slog.Error("load settings for eligibility", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to load settings")
		return
	}

	reasons := []string{}
	eligible := true

	if msg := h.checkKillSwitch(ps); msg != "" {
		reasons = append(reasons, msg)
		eligible = false
	}

	windowMsg := h.checkRequestWindow(ps)
	inWindow := windowMsg == ""
	if windowMsg != "" {
		reasons = append(reasons, windowMsg)
		eligible = false
	}

	dailyMsg, dailyRemaining, err := h.checkDailyLimit(ctx, ps, userID)
	if err != nil {
		slog.Error("check daily limit for eligibility", "error", err)
	}
	if dailyMsg != "" {
		reasons = append(reasons, dailyMsg)
		eligible = false
	}

	monthlyMsg, monthlyRemaining, err := h.checkMonthlyLimit(ctx, ps, userID)
	if err != nil {
		slog.Error("check monthly limit for eligibility", "error", err)
	}
	if monthlyMsg != "" {
		reasons = append(reasons, monthlyMsg)
		eligible = false
	}

	if !user.IsTermsAccepted {
		reasons = append(reasons, "terms not accepted")
		eligible = false
	}

	if !user.PhoneVerified || !user.PhoneNumber.Valid || user.PhoneNumber.String == "" {
		reasons = append(reasons, "phone not verified")
		eligible = false
	}

	_, activeErr := h.queries.GetActiveRequestByUserID(ctx, userID)
	if activeErr == nil {
		reasons = append(reasons, "you already have an active advance request")
		eligible = false
	}

	now := time.Now().In(h.loc)
	daysInMonth := daysIn(now.Year(), now.Month())
	endDay := ps.windowEndDay
	if endDay <= 0 || endDay > daysInMonth {
		endDay = daysInMonth
	}

	type windowInfo struct {
		StartDay int  `json:"start_day"`
		EndDay   int  `json:"end_day"`
		InWindow bool `json:"in_window"`
	}

	JSONSuccess(c, http.StatusOK, gin.H{
		"eligible":                   eligible,
		"reasons":                    reasons,
		"kill_switch_active":         ps.killSwitchEnabled,
		"request_window":             windowInfo{StartDay: ps.windowStartDay, EndDay: endDay, InWindow: inWindow},
		"daily_requests_remaining":   dailyRemaining,
		"monthly_requests_remaining": monthlyRemaining,
		"advance_amount_xaf":         ps.advanceAmount.String(),
		"phone_verified":             user.PhoneVerified,
		"terms_accepted":             user.IsTermsAccepted,
	})
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
	GetPhoneVerificationByCampayRef(ctx context.Context, campayPayoutRef pgtype.Text) (db.PhoneVerification, error)
	UpdatePhoneVerificationStatus(ctx context.Context, arg db.UpdatePhoneVerificationStatusParams) (db.PhoneVerification, error)
	SetPhoneVerified(ctx context.Context, id uuid.UUID) (db.User, error)
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

	campayRef := pgtype.Text{String: wh.Reference, Valid: true}

	advanceReq, advanceErr := h.queries.GetAdvanceRequestByCampayRef(c.Request.Context(), campayRef)
	if advanceErr == nil {
		h.handleAdvanceWebhook(c, advanceReq, wh)
		return
	}

	phoneVerif, phoneErr := h.queries.GetPhoneVerificationByCampayRef(c.Request.Context(), campayRef)
	if phoneErr == nil {
		h.handlePhoneVerificationWebhook(c, phoneVerif, wh)
		return
	}

	slog.Warn("webhook for unknown reference", "reference", wh.Reference)
	JSONOK(c, http.StatusOK)
}

func (h *webhookHandler) handleAdvanceWebhook(c *gin.Context, existing db.AdvanceRequest, wh campay.WebhookPayload) {
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

	_, err := h.queries.UpdateAdvanceRequestStatus(c.Request.Context(), db.UpdateAdvanceRequestStatusParams{
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

	if existing.Status != newStatus {
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
	}

	JSONOK(c, http.StatusOK)
}

func (h *webhookHandler) handlePhoneVerificationWebhook(c *gin.Context, verif db.PhoneVerification, wh campay.WebhookPayload) {
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
		slog.Warn("unknown webhook status for phone verification", "status", wh.Status, "reference", wh.Reference)
		JSONOK(c, http.StatusOK)
		return
	}

	_, err := h.queries.UpdatePhoneVerificationStatus(c.Request.Context(), db.UpdatePhoneVerificationStatusParams{
		ID:              verif.ID,
		Status:          newStatus,
		FailureReason:   failureReason,
		CampayPayoutRef: pgtype.Text{String: wh.Reference, Valid: true},
	})
	if err != nil {
		slog.Error("update phone verification status from webhook", "error", err, "reference", wh.Reference)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to update phone verification")
		return
	}

	if newStatus == db.RequestStatusSuccess {
		if _, err := h.queries.SetPhoneVerified(c.Request.Context(), verif.UserID); err != nil {
			slog.Error("set phone verified from webhook", "error", err, "user_id", verif.UserID)
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to set phone verified")
			return
		}

		if verif.Status != newStatus {
			eventMeta, _ := json.Marshal(map[string]interface{}{
				"verification_id": verif.ID.String(),
				"phone_number":    verif.PhoneNumber,
			})
			_, _ = h.queries.CreateEvent(c.Request.Context(), db.CreateEventParams{
				UserID:    pgtype.UUID{Bytes: verif.UserID, Valid: true},
				EventType: "phone_verified",
				Metadata:  eventMeta,
			})
		}
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

func daysIn(year int, m time.Month) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func ordinal(n int) string {
	suffix := "th"
	switch n % 10 {
	case 1:
		if n%100 != 11 {
			suffix = "st"
		}
	case 2:
		if n%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if n%100 != 13 {
			suffix = "rd"
		}
	}
	return itoa(n) + suffix
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
