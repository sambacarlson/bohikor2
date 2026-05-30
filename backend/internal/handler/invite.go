package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Iknite-Space/bohikor2/internal/service"
)

type InviteQuerier interface {
	Invite(ctx context.Context, email string, invitedBy string) (*service.InviteResult, error)
}

type inviteRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func HandleInvite(q InviteQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req inviteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			JSONError(c, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}

		adminID := c.GetString("admin_id")
		if adminID == "" {
			JSONError(c, http.StatusUnauthorized, "unauthorized", "missing admin identity")
			return
		}

		result, err := q.Invite(c.Request.Context(), req.Email, adminID)
		if err != nil {
			if errors.Is(err, service.ErrActiveInvitationExists) {
				JSONError(c, http.StatusConflict, "conflict", err.Error())
				return
			}
			JSONError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		JSONSuccess(c, http.StatusCreated, result.Invitation)
	}
}
