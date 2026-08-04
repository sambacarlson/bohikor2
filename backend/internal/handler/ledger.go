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

// ledgerQuerier is the data layer HandleGetLedger needs: a company's running
// balance plus its ledger history, both already scoped by company_id.
type ledgerQuerier interface {
	GetCompanyBalance(ctx context.Context, companyID uuid.UUID) (pgtype.Numeric, error)
	ListLedgerByCompany(ctx context.Context, arg db.ListLedgerByCompanyParams) ([]db.CompanyLedger, error)
}

// HandleGetLedger returns the requesting company admin's own company balance
// and ledger history. Unlike the platform-admin ledger routes, this is
// read-only and always scoped to the caller's own company_id from context —
// never a company id from the request.
func HandleGetLedger(q ledgerQuerier) gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID, ok := companyIDFromContext(c)
		if !ok {
			return
		}

		page, perPage := parsePagination(c, 50, 200)

		balance, err := q.GetCompanyBalance(c.Request.Context(), companyID)
		if err != nil {
			slog.Error("get company balance", "error", err, "company_id", companyID)
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to read balance")
			return
		}

		entries, err := q.ListLedgerByCompany(c.Request.Context(), db.ListLedgerByCompanyParams{
			CompanyID: companyID,
			Limit:     int32(perPage),
			Offset:    int32((page - 1) * perPage),
		})
		if err != nil {
			slog.Error("list ledger", "error", err, "company_id", companyID)
			JSONError(c, http.StatusInternalServerError, "internal_error", "failed to list ledger entries")
			return
		}

		JSONSuccess(c, http.StatusOK, gin.H{
			"balance_xaf": numericToString(balance),
			"entries":     emptyIfNil(entries),
		})
	}
}
