package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type userQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
}

type adminQuerier interface {
	GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error)
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
			"data": user,
		})
	}
}

func handleAdminMe(q adminQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectIDStr := c.GetString("subject_id")

		subjectID, err := uuid.Parse(subjectIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid subject id",
			})
			return
		}

		admin, err := q.GetAdminByID(c.Request.Context(), subjectID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "admin not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": admin,
		})
	}
}
