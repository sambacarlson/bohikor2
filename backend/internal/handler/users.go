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
	ListUsersByCompany(ctx context.Context, arg db.ListUsersByCompanyParams) ([]db.User, error)
	UnlockUser(ctx context.Context, arg db.UnlockUserParams) (db.User, error)
	UpdateUserStatus(ctx context.Context, arg db.UpdateUserStatusParams) (db.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	ResetEmailOTPFailures(ctx context.Context, email string) error
}

func HandleListUsers(q usersQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID, ok := companyIDFromContext(c)
		if !ok {
			return
		}

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

		users, err := q.ListUsersByCompany(c.Request.Context(), db.ListUsersByCompanyParams{
			CompanyID: companyID,
			Limit:     int32(perPage),
			Offset:    int32((page - 1) * perPage),
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

// HandleUnlockUser unlocks the user and also resets their OTP failure
// counter — admins should clear both PIN and OTP blocks in one action.
// Scoped to the admin's company so cross-tenant ids resolve to "not found".
func HandleUnlockUser(q usersQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID, ok := companyIDFromContext(c)
		if !ok {
			return
		}

		userIDStr := c.Param("id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			JSONError(c, http.StatusBadRequest, "invalid_id", "invalid user ID")
			return
		}

		user, err := q.UnlockUser(c.Request.Context(), db.UnlockUserParams{
			ID:        userID,
			CompanyID: companyID,
		})
		if err != nil {
			JSONError(c, http.StatusNotFound, "not_found", "user not found")
			return
		}

		if err := q.ResetEmailOTPFailures(c.Request.Context(), user.Email); err != nil {
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to reset OTP failures")
			return
		}

		JSONSuccess(c, http.StatusOK, user)
	}
}

func HandleSuspendUser(q usersQuerier) gin.HandlerFunc {
	return updateUserStatusHandler(q, db.UserStatusSuspended)
}

func HandleActivateUser(q usersQuerier) gin.HandlerFunc {
	return updateUserStatusHandler(q, db.UserStatusActive)
}

// updateUserStatusHandler suspends/activates a target user within the admin's company.
func updateUserStatusHandler(q usersQuerier, status db.UserStatus) gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID, ok := companyIDFromContext(c)
		if !ok {
			return
		}

		userIDStr := c.Param("id")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			JSONError(c, http.StatusBadRequest, "invalid_id", "invalid user ID")
			return
		}

		user, err := q.UpdateUserStatus(c.Request.Context(), db.UpdateUserStatusParams{
			ID:        userID,
			Status:    status,
			CompanyID: companyID,
		})
		if err != nil {
			JSONError(c, http.StatusNotFound, "not_found", "user not found")
			return
		}

		JSONSuccess(c, http.StatusOK, user)
	}
}
