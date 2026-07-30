package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

var numericSettings = map[string]bool{
	"advance_amount_xaf":       true,
	"request_window_start_day": true,
	"request_window_end_day":   true,
	"daily_request_limit":      true,
	"monthly_request_limit":    true,
}

var boolSettings = map[string]bool{
	"kill_switch_enabled": true,
}

// coerceValue converts JSON-string values from the admin UI into the correct
// JSON primitive (number for numeric settings, bool for kill_switch), since
// JSONB stores whatever JSON type we write.
func coerceValue(key string, raw json.RawMessage) json.RawMessage {
	if numericSettings[key] {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				if coerced, err := json.Marshal(f); err == nil {
					return coerced
				}
			}
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			return raw
		}
	}
	if boolSettings[key] {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if b, err := strconv.ParseBool(s); err == nil {
				if coerced, err := json.Marshal(b); err == nil {
					return coerced
				}
			}
		}
		var b bool
		if json.Unmarshal(raw, &b) == nil {
			return raw
		}
	}
	return raw
}

type settingsQuerier interface {
	ListSettingsByCompany(ctx context.Context, companyID uuid.UUID) ([]db.Setting, error)
	UpsertSetting(ctx context.Context, arg db.UpsertSettingParams) (db.Setting, error)
}

func HandleListSettings(q settingsQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID, ok := companyIDFromContext(c)
		if !ok {
			return
		}

		settings, err := q.ListSettingsByCompany(c.Request.Context(), companyID)
		if err != nil {
			slog.Error("list settings", "error", err)
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to list settings")
			return
		}

		type settingResponse struct {
			Key       string          `json:"key"`
			Value     json.RawMessage `json:"value"`
			UpdatedAt string          `json:"updated_at"`
			UpdatedBy *uuid.UUID      `json:"updated_by,omitempty"`
		}

		result := make([]settingResponse, len(settings))
		for i, s := range settings {
			val := json.RawMessage(s.Value)
			var updatedBy *uuid.UUID
			if s.UpdatedBy.Valid {
				id := uuid.UUID(s.UpdatedBy.Bytes)
				updatedBy = &id
			}
			result[i] = settingResponse{
				Key:       s.Key,
				Value:     val,
				UpdatedAt: s.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
				UpdatedBy: updatedBy,
			}
		}
		JSONSuccess(c, http.StatusOK, result)
	}
}

func HandleUpdateSettings(q settingsQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID, ok := companyIDFromContext(c)
		if !ok {
			return
		}

		adminIDStr := c.GetString("admin_id")
		if adminIDStr == "" {
			JSONError(c, http.StatusUnauthorized, "unauthorized", "admin not authenticated")
			return
		}
		adminID, err := uuid.Parse(adminIDStr)
		if err != nil {
			JSONError(c, http.StatusInternalServerError, "internal_error", "invalid admin ID")
			return
		}

		var rawSettings map[string]json.RawMessage
		if err := c.ShouldBindJSON(&rawSettings); err != nil {
			JSONError(c, http.StatusBadRequest, "invalid_request", "invalid settings payload")
			return
		}

		results := make(map[string]json.RawMessage)
		for key, value := range rawSettings {
			coerced := coerceValue(key, value)
			updated, err := q.UpsertSetting(c.Request.Context(), db.UpsertSettingParams{
				CompanyID: companyID,
				Key:       key,
				Value:     coerced,
				UpdatedBy: pgtype.UUID{Bytes: adminID, Valid: true},
			})
			if err != nil {
				slog.Error("upsert setting", "key", key, "error", err)
				JSONError(c, http.StatusInternalServerError, "internal_error", "failed to update setting: "+key)
				return
			}
			results[key] = json.RawMessage(updated.Value)
		}

		JSONSuccess(c, http.StatusOK, results)
	}
}
