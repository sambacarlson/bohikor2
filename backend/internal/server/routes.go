package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/handler"
)

type userQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
}

func handleUserMe(q userQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectIDStr := c.GetString("subject_id")

		subjectID, err := uuid.Parse(subjectIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid subject id",
			})
			return
		}

		user, err := q.GetUserByID(c.Request.Context(), subjectID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "user not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": handler.SanitizeUser(user),
		})
	}
}

// handleAdminMe reads the admin and company_slug that RequireAdmin (and its
// shared companyActive check) already loaded for this request, rather than
// re-querying rows the middleware just fetched.
func handleAdminMe() gin.HandlerFunc {
	return func(c *gin.Context) {
		admin := c.MustGet("admin").(db.Admin)
		companySlug := c.GetString("company_slug")

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"id":           admin.ID,
				"company_id":   admin.CompanyID,
				"company_slug": companySlug,
				"email":        admin.Email,
				"created_at":   admin.CreatedAt,
			},
		})
	}
}
