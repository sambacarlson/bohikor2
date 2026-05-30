package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type Querier interface {
	GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
}

func RequireAdmin(q Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectIDStr := c.GetString("subject_id")
		if subjectIDStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthenticated",
			})
			return
		}

		subjectType := c.GetString("subject_type")
		if subjectType != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "admin access required",
			})
			return
		}

		subjectID, err := uuid.Parse(subjectIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid subject id",
			})
			return
		}

		admin, err := q.GetAdminByID(c.Request.Context(), subjectID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "admin not found",
			})
			return
		}

		c.Set("admin_id", admin.ID.String())
		c.Set("admin_email", admin.Email)
		c.Next()
	}
}

func RequireActiveUser(q Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectIDStr := c.GetString("subject_id")
		if subjectIDStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthenticated",
			})
			return
		}

		subjectType := c.GetString("subject_type")
		if subjectType != "user" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "user access required",
			})
			return
		}

		subjectID, err := uuid.Parse(subjectIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid subject id",
			})
			return
		}

		user, err := q.GetUserByID(c.Request.Context(), subjectID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "user not found",
			})
			return
		}

		if user.Status == db.UserStatusSuspended {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "account suspended",
			})
			return
		}

		c.Set("user_id", user.ID)
		c.Set("user_email", user.Email)
		c.Next()
	}
}
