package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type usersQuerier interface {
	ListUsers(ctx context.Context, arg db.ListUsersParams) ([]db.User, error)
	UnlockUser(ctx context.Context, id uuid.UUID) (db.User, error)
}

func HandleListUsers(q usersQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageStr := c.DefaultQuery("page", "1")
		perPageStr := c.DefaultQuery("per_page", "20")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}
		perPage, err := strconv.Atoi(perPageStr)
		if err != nil || perPage < 1 || perPage > 100 {
			perPage = 20
		}

		users, err := q.ListUsers(c.Request.Context(), db.ListUsersParams{
			Limit:  int32(perPage),
			Offset: int32((page - 1) * perPage),
		})
		if err != nil {
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to list users")
			return
		}

		if users == nil {
			users = []db.User{}
		}

		JSONSuccess(c, http.StatusOK, users)
	}
}

func HandleUnlockUser(q usersQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			JSONError(c, http.StatusBadRequest, "invalid_id", "invalid user ID")
			return
		}

		user, err := q.UnlockUser(c.Request.Context(), userID)
		if err != nil {
			JSONError(c, http.StatusNotFound, "not_found", "user not found")
			return
		}

		JSONSuccess(c, http.StatusOK, user)
	}
}
