package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type eventsQuerier interface {
	ListEventsWithUserByCompany(ctx context.Context, arg db.ListEventsWithUserByCompanyParams) ([]db.ListEventsWithUserByCompanyRow, error)
}

func HandleListEvents(q eventsQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID, ok := companyIDFromContext(c)
		if !ok {
			return
		}

		page := 1
		perPage := 50

		events, err := q.ListEventsWithUserByCompany(c.Request.Context(), db.ListEventsWithUserByCompanyParams{
			CompanyID: pgtype.UUID{Bytes: companyID, Valid: true},
			Limit:     int32(perPage),
			Offset:    int32((page - 1) * perPage),
		})
		if err != nil {
			slog.Error("list events", "error", err)
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to list events")
			return
		}

		if events == nil {
			events = []db.ListEventsWithUserByCompanyRow{}
		}

		JSONSuccess(c, http.StatusOK, events)
	}
}

// companyIDFromContext reads the company scope that RequireAdmin/RequireActiveUser
// placed in the Gin context. Writes a 500 and returns false when absent.
func companyIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get("company_id")
	if !exists {
		JSONError(c, http.StatusInternalServerError, "internal_error", "missing company scope")
		return uuid.UUID{}, false
	}
	companyID, ok := val.(uuid.UUID)
	if !ok {
		JSONError(c, http.StatusInternalServerError, "internal_error", "invalid company scope")
		return uuid.UUID{}, false
	}
	return companyID, true
}
