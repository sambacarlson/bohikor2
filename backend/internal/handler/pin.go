package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/authpassword"
)

type pinQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	UpdateUserPinHash(ctx context.Context, arg db.UpdateUserPinHashParams) (db.User, error)
	ResetLoginAttempts(ctx context.Context, id uuid.UUID) (db.User, error)
}

type PinHandler struct {
	queries pinQuerier
	hasher  authpassword.Hasher
}

func NewPinHandler(queries pinQuerier, hasher authpassword.Hasher) *PinHandler {
	return &PinHandler{
		queries: queries,
		hasher:  hasher,
	}
}

func (h *PinHandler) ChangePin(c *gin.Context) {
	val, exists := c.Get("user_id")
	if !exists {
		JSONError(c, http.StatusUnauthorized, "unauthorized", "user not authenticated")
		return
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		slog.Error("invalid user_id type in context", "value", val)
		JSONError(c, http.StatusInternalServerError, "internal_error", "invalid user ID")
		return
	}

	var req struct {
		CurrentPIN string `json:"current_pin" binding:"required"`
		NewPIN     string `json:"new_pin" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "current_pin and new_pin are required")
		return
	}

	if len(req.NewPIN) != 5 {
		JSONError(c, http.StatusBadRequest, "invalid_pin", "PIN must be 5 digits")
		return
	}

	user, err := h.queries.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "user not found")
		return
	}

	if !user.PinHash.Valid || user.PinHash.String == "" {
		JSONError(c, http.StatusBadRequest, "no_pin_set", "No PIN is currently set. Use the initial setup flow instead.")
		return
	}

	if !h.hasher.Verify(user.PinHash.String, req.CurrentPIN) {
		JSONError(c, http.StatusUnauthorized, "invalid_pin", "Current PIN is incorrect")
		return
	}

	newPinHash, err := h.hasher.Hash(req.NewPIN)
	if err != nil {
		slog.Error("hash pin", "error", err, "user_id", userID)
		JSONError(c, http.StatusInternalServerError, "hash_failed", "Failed to hash PIN")
		return
	}

	user, err = h.queries.UpdateUserPinHash(c.Request.Context(), db.UpdateUserPinHashParams{
		ID:      userID,
		PinHash: pgtype.Text{String: newPinHash, Valid: true},
	})
	if err != nil {
		slog.Error("update pin hash", "error", err, "user_id", userID)
		JSONError(c, http.StatusInternalServerError, "internal_error", "Failed to update PIN")
		return
	}

	JSONSuccess(c, http.StatusOK, gin.H{
		"message": "PIN updated successfully",
	})
}

func (h *PinHandler) ResetPin(c *gin.Context) {
	val, exists := c.Get("user_id")
	if !exists {
		JSONError(c, http.StatusUnauthorized, "unauthorized", "user not authenticated")
		return
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		slog.Error("invalid user_id type in context", "value", val)
		JSONError(c, http.StatusInternalServerError, "internal_error", "invalid user ID")
		return
	}

	var req struct {
		NewPIN string `json:"new_pin" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "new_pin is required")
		return
	}

	if len(req.NewPIN) != 5 {
		JSONError(c, http.StatusBadRequest, "invalid_pin", "PIN must be 5 digits")
		return
	}

	user, err := h.queries.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "user not found")
		return
	}

	if user.Status == db.UserStatusLocked {
		JSONError(c, http.StatusForbidden, "account_locked", "Your account is locked. Contact your manager.")
		return
	}

	newPinHash, err := h.hasher.Hash(req.NewPIN)
	if err != nil {
		slog.Error("hash pin", "error", err, "user_id", userID)
		JSONError(c, http.StatusInternalServerError, "hash_failed", "Failed to hash PIN")
		return
	}

	_, err = h.queries.UpdateUserPinHash(c.Request.Context(), db.UpdateUserPinHashParams{
		ID:      userID,
		PinHash: pgtype.Text{String: newPinHash, Valid: true},
	})
	if err != nil {
		slog.Error("update pin hash", "error", err, "user_id", userID)
		JSONError(c, http.StatusInternalServerError, "internal_error", "Failed to update PIN")
		return
	}

	if _, err := h.queries.ResetLoginAttempts(c.Request.Context(), userID); err != nil {
		slog.Error("reset login attempts after pin reset", "error", err, "user_id", userID)
	}

	JSONSuccess(c, http.StatusOK, gin.H{
		"message": "PIN reset successfully",
	})
}
