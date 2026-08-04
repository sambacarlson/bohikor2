package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

func JSONError(c *gin.Context, status int, code, message string, details ...any) {
	resp := ErrorResponse{
		Error: message,
		Code:  code,
	}
	if len(details) > 0 {
		resp.Details = details[0]
	}
	c.JSON(status, resp)
}

func JSONSuccess(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

func JSONOK(c *gin.Context, status int) {
	c.JSON(status, gin.H{"status": "ok"})
}

// parsePagination reads "page"/"per_page" query params, falling back to
// page 1 and defaultPerPage, and clamping per_page to [1, maxPerPage].
// Malformed or out-of-range values fall back to the default rather than
// erroring, since pagination is advisory, not a validated input contract.
func parsePagination(c *gin.Context, defaultPerPage, maxPerPage int) (page, perPage int) {
	page = 1
	if v, err := strconv.Atoi(c.Query("page")); err == nil && v > 0 {
		page = v
	}

	perPage = defaultPerPage
	if v, err := strconv.Atoi(c.Query("per_page")); err == nil && v > 0 && v <= maxPerPage {
		perPage = v
	}

	return page, perPage
}

// emptyIfNil replaces a nil slice with an empty one so list endpoints
// consistently serialize "[]" rather than "null" when there are no rows.
func emptyIfNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// respondList runs fetch and writes a 500 (logging errLogMsg) on failure,
// otherwise responds 200 with the nil-safe rows. Shared by list endpoints
// whose only per-endpoint logic is which query to run.
func respondList[T any](c *gin.Context, errLogMsg, errUserMsg string, fetch func() ([]T, error)) {
	rows, err := fetch()
	if err != nil {
		slog.Error(errLogMsg, "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", errUserMsg)
		return
	}
	JSONSuccess(c, http.StatusOK, emptyIfNil(rows))
}
