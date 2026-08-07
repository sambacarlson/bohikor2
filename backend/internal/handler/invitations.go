package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type invitationsQuerier interface {
	ListInvitationsByCompany(ctx context.Context, companyID uuid.UUID) ([]db.Invitation, error)
}

func HandleListInvitations(q invitationsQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID, ok := companyIDFromContext(c)
		if !ok {
			return
		}

		invitations, err := q.ListInvitationsByCompany(c.Request.Context(), companyID)
		if err != nil {
			slog.Error("list invitations", "error", err, "company_id", companyID)
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to list invitations")
			return
		}

		if invitations == nil {
			invitations = []db.Invitation{}
		}

		JSONSuccess(c, http.StatusOK, invitations)
	}
}
