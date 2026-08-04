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
	GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error)
	GetPlatformAdminByID(ctx context.Context, id uuid.UUID) (db.PlatformAdmin, error)
}

func getLockedMessage(status db.UserStatus) string {
	switch status {
	case db.UserStatusSuspended:
		return "account suspended"
	case db.UserStatusLocked:
		return "account locked"
	default:
		return "account restricted"
	}
}

// resolveSubject extracts and validates the subject id/type set by JWTAuth,
// aborting with the appropriate response itself on failure. Shared by
// RequireAdmin, RequireActiveUser, and RequirePlatformAdmin, each of which
// then loads its own entity type by the returned id.
func resolveSubject(c *gin.Context, wantType, deniedMessage string) (uuid.UUID, bool) {
	subjectIDStr := c.GetString("subject_id")
	if subjectIDStr == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "unauthenticated",
		})
		return uuid.UUID{}, false
	}

	if c.GetString("subject_type") != wantType {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": deniedMessage,
		})
		return uuid.UUID{}, false
	}

	subjectID, err := uuid.Parse(subjectIDStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid subject id",
		})
		return uuid.UUID{}, false
	}

	return subjectID, true
}

// companyActive loads the company and returns false (with a written response) if
// it is missing or suspended. Shared by RequireAdmin and RequireActiveUser.
func companyActive(c *gin.Context, q Querier, companyID uuid.UUID) bool {
	company, err := q.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "company not found",
		})
		return false
	}
	if company.Status == db.CompanyStatusSuspended {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "company suspended",
		})
		return false
	}
	c.Set("company_slug", company.Slug)
	return true
}

func RequireAdmin(q Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectID, ok := resolveSubject(c, "admin", "admin access required")
		if !ok {
			return
		}

		admin, err := q.GetAdminByID(c.Request.Context(), subjectID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "admin not found",
			})
			return
		}

		// Isolation guarantee: the company claimed in the token must match the record.
		if admin.CompanyID.String() != c.GetString("claim_company_id") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "company mismatch",
			})
			return
		}

		if !companyActive(c, q, admin.CompanyID) {
			return
		}

		c.Set("admin_id", admin.ID.String())
		c.Set("admin_email", admin.Email)
		c.Set("company_id", admin.CompanyID)
		c.Set("admin", admin)
		c.Next()
	}
}

func RequireActiveUser(q Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectID, ok := resolveSubject(c, "user", "user access required")
		if !ok {
			return
		}

		user, err := q.GetUserByID(c.Request.Context(), subjectID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "user not found",
			})
			return
		}

		// Isolation guarantee: the company claimed in the token must match the record.
		if user.CompanyID.String() != c.GetString("claim_company_id") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "company mismatch",
			})
			return
		}

		if user.Status == db.UserStatusSuspended || user.Status == db.UserStatusLocked {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": getLockedMessage(user.Status),
			})
			return
		}

		if !companyActive(c, q, user.CompanyID) {
			return
		}

		c.Set("user_id", user.ID)
		c.Set("user_email", user.Email)
		c.Set("company_id", user.CompanyID)
		c.Next()
	}
}

// RequirePlatformAdmin gates the /api/platform/* routes. Platform admins are
// global (no company); their token carries an empty company_id claim.
func RequirePlatformAdmin(q Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		subjectID, ok := resolveSubject(c, "platform_admin", "platform admin access required")
		if !ok {
			return
		}

		pa, err := q.GetPlatformAdminByID(c.Request.Context(), subjectID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "platform admin not found",
			})
			return
		}

		c.Set("platform_admin_id", pa.ID.String())
		c.Set("platform_admin_email", pa.Email)
		c.Next()
	}
}
