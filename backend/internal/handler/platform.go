package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/authpassword"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// platformStore is the data layer the platform (super-admin) handler needs.
// CreateCompanyWithSettings is transactional (company row + seeded settings).
type platformStore interface {
	CreateCompanyWithSettings(ctx context.Context, arg db.CreateCompanyParams) (db.Company, error)
	GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error)
	ListCompaniesWithBalance(ctx context.Context) ([]db.ListCompaniesWithBalanceRow, error)
	UpdateCompanyStatus(ctx context.Context, arg db.UpdateCompanyStatusParams) (db.Company, error)
	CreateAdmin(ctx context.Context, arg db.CreateAdminParams) (db.Admin, error)
	CreateLedgerEntry(ctx context.Context, arg db.CreateLedgerEntryParams) (db.CompanyLedger, error)
	GetCompanyBalance(ctx context.Context, companyID uuid.UUID) (pgtype.Numeric, error)
}

type PlatformHandler struct {
	store  platformStore
	hasher authpassword.Hasher
}

func NewPlatformHandler(store platformStore, hasher authpassword.Hasher) *PlatformHandler {
	return &PlatformHandler{store: store, hasher: hasher}
}

// RealPlatformStore wires the platform store to sqlc + a pool for transactions.
type RealPlatformStore struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

func NewRealPlatformStore(queries *db.Queries, pool *pgxpool.Pool) *RealPlatformStore {
	return &RealPlatformStore{queries: queries, pool: pool}
}

// CreateCompanyWithSettings creates the company and seeds its default settings
// in one transaction, so a company never exists without its settings.
func (s *RealPlatformStore) CreateCompanyWithSettings(ctx context.Context, arg db.CreateCompanyParams) (db.Company, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Company{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)
	company, err := qtx.CreateCompany(ctx, arg)
	if err != nil {
		return db.Company{}, fmt.Errorf("create company: %w", err)
	}
	if err := qtx.SeedDefaultSettings(ctx, company.ID); err != nil {
		return db.Company{}, fmt.Errorf("seed settings: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Company{}, fmt.Errorf("commit: %w", err)
	}
	return company, nil
}

func (s *RealPlatformStore) GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error) {
	return s.queries.GetCompanyByID(ctx, id)
}

func (s *RealPlatformStore) ListCompaniesWithBalance(ctx context.Context) ([]db.ListCompaniesWithBalanceRow, error) {
	return s.queries.ListCompaniesWithBalance(ctx)
}

func (s *RealPlatformStore) UpdateCompanyStatus(ctx context.Context, arg db.UpdateCompanyStatusParams) (db.Company, error) {
	return s.queries.UpdateCompanyStatus(ctx, arg)
}

func (s *RealPlatformStore) CreateAdmin(ctx context.Context, arg db.CreateAdminParams) (db.Admin, error) {
	return s.queries.CreateAdmin(ctx, arg)
}

func (s *RealPlatformStore) CreateLedgerEntry(ctx context.Context, arg db.CreateLedgerEntryParams) (db.CompanyLedger, error) {
	return s.queries.CreateLedgerEntry(ctx, arg)
}

func (s *RealPlatformStore) GetCompanyBalance(ctx context.Context, companyID uuid.UUID) (pgtype.Numeric, error) {
	return s.queries.GetCompanyBalance(ctx, companyID)
}

// numericToString renders a pgtype.Numeric as a plain decimal string for JSON.
func numericToString(n pgtype.Numeric) string {
	v, err := n.Value()
	if err != nil || v == nil {
		return "0"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "0"
}

// parseCompanyID reads and validates the ":id" path param shared by every
// single-company platform route, writing the 400 response itself on failure.
func parseCompanyID(c *gin.Context) (uuid.UUID, bool) {
	companyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_id", "invalid company id")
		return uuid.UUID{}, false
	}
	return companyID, true
}

// balanceOrFail fetches a company's balance, writing the 500 response itself
// on failure. Shared by every handler that returns a companyResponse.
func (h *PlatformHandler) balanceOrFail(c *gin.Context, companyID uuid.UUID) (pgtype.Numeric, bool) {
	balance, err := h.store.GetCompanyBalance(c.Request.Context(), companyID)
	if err != nil {
		slog.Error("get company balance", "error", err, "company_id", companyID)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to read balance")
		return pgtype.Numeric{}, false
	}
	return balance, true
}

func companyResponse(company db.Company, balance pgtype.Numeric) gin.H {
	return gin.H{
		"id":          company.ID,
		"slug":        company.Slug,
		"name":        company.Name,
		"status":      company.Status,
		"balance_xaf": numericToString(balance),
		"created_at":  company.CreatedAt,
		"updated_at":  company.UpdatedAt,
	}
}

// CreateCompany provisions a company and seeds its default settings in one tx.
func (h *PlatformHandler) CreateCompany(c *gin.Context) {
	var req struct {
		Slug string `json:"slug" binding:"required"`
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "slug and name are required")
		return
	}

	if !slugPattern.MatchString(req.Slug) {
		JSONError(c, http.StatusBadRequest, "invalid_slug", "slug must be lowercase alphanumeric words separated by hyphens")
		return
	}

	company, err := h.store.CreateCompanyWithSettings(c.Request.Context(), db.CreateCompanyParams{
		Slug:      req.Slug,
		Name:      req.Name,
		CreatedBy: platformAdminUUID(c),
	})
	if err != nil {
		slog.Error("create company", "error", err)
		JSONError(c, http.StatusConflict, "create_failed", "failed to create company (slug may already exist)")
		return
	}

	JSONSuccess(c, http.StatusCreated, companyResponse(company, pgtype.Numeric{}))
}

// CreateCompanyAdmin creates the company's first (or an additional) admin.
func (h *PlatformHandler) CreateCompanyAdmin(c *gin.Context) {
	companyID, ok := parseCompanyID(c)
	if !ok {
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "email and password are required")
		return
	}

	if _, err := h.store.GetCompanyByID(c.Request.Context(), companyID); err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "company not found")
		return
	}

	hashed, err := h.hasher.Hash(req.Password)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "hash_failed", "failed to hash password")
		return
	}

	admin, err := h.store.CreateAdmin(c.Request.Context(), db.CreateAdminParams{
		CompanyID:    companyID,
		Email:        req.Email,
		PasswordHash: hashed,
	})
	if err != nil {
		JSONError(c, http.StatusConflict, "create_failed", "failed to create admin (email may already be in use)")
		return
	}

	JSONSuccess(c, http.StatusCreated, gin.H{
		"id":         admin.ID,
		"company_id": admin.CompanyID,
		"email":      admin.Email,
		"created_at": admin.CreatedAt,
	})
}

// TopUpCompany posts a positive `topup` ledger entry (super-admin funds a company).
func (h *PlatformHandler) TopUpCompany(c *gin.Context) {
	companyID, ok := parseCompanyID(c)
	if !ok {
		return
	}

	var req struct {
		AmountXaf string `json:"amount_xaf" binding:"required"`
		Note      string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "amount_xaf is required")
		return
	}

	var amount pgtype.Numeric
	if err := amount.Scan(req.AmountXaf); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_amount", "amount_xaf must be a valid number")
		return
	}
	if isNonPositive(amount) {
		JSONError(c, http.StatusBadRequest, "invalid_amount", "amount_xaf must be positive")
		return
	}

	if _, err := h.store.GetCompanyByID(c.Request.Context(), companyID); err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "company not found")
		return
	}

	note := pgtype.Text{Valid: false}
	if req.Note != "" {
		note = pgtype.Text{String: req.Note, Valid: true}
	}

	entry, err := h.store.CreateLedgerEntry(c.Request.Context(), db.CreateLedgerEntryParams{
		CompanyID: companyID,
		EntryType: "topup",
		AmountXaf: amount,
		CreatedBy: platformAdminUUID(c),
		Note:      note,
	})
	if err != nil {
		slog.Error("create topup ledger entry", "error", err, "company_id", companyID)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to top up company")
		return
	}

	balance, ok := h.balanceOrFail(c, companyID)
	if !ok {
		return
	}

	JSONSuccess(c, http.StatusCreated, gin.H{
		"entry":       entry,
		"balance_xaf": numericToString(balance),
	})
}

func (h *PlatformHandler) ListCompanies(c *gin.Context) {
	companies, err := h.store.ListCompaniesWithBalance(c.Request.Context())
	if err != nil {
		slog.Error("list companies", "error", err)
		JSONError(c, http.StatusInternalServerError, "internal_error", "failed to list companies")
		return
	}

	result := make([]gin.H, 0, len(companies))
	for _, company := range companies {
		result = append(result, companyResponse(db.Company{
			ID:        company.ID,
			Slug:      company.Slug,
			Name:      company.Name,
			Status:    company.Status,
			CreatedBy: company.CreatedBy,
			CreatedAt: company.CreatedAt,
			UpdatedAt: company.UpdatedAt,
		}, company.Balance))
	}

	JSONSuccess(c, http.StatusOK, result)
}

func (h *PlatformHandler) GetCompany(c *gin.Context) {
	companyID, ok := parseCompanyID(c)
	if !ok {
		return
	}

	company, err := h.store.GetCompanyByID(c.Request.Context(), companyID)
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "company not found")
		return
	}

	balance, ok := h.balanceOrFail(c, companyID)
	if !ok {
		return
	}

	JSONSuccess(c, http.StatusOK, companyResponse(company, balance))
}

func (h *PlatformHandler) UpdateCompanyStatus(c *gin.Context) {
	companyID, ok := parseCompanyID(c)
	if !ok {
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "status is required")
		return
	}

	if req.Status != string(db.CompanyStatusActive) && req.Status != string(db.CompanyStatusSuspended) {
		JSONError(c, http.StatusBadRequest, "invalid_status", "status must be 'active' or 'suspended'")
		return
	}

	company, err := h.store.UpdateCompanyStatus(c.Request.Context(), db.UpdateCompanyStatusParams{
		ID:     companyID,
		Status: db.CompanyStatus(req.Status),
	})
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "company not found")
		return
	}

	balance, ok := h.balanceOrFail(c, companyID)
	if !ok {
		return
	}

	JSONSuccess(c, http.StatusOK, companyResponse(company, balance))
}

// platformAdminUUID reads the authenticated platform admin id from context.
func platformAdminUUID(c *gin.Context) pgtype.UUID {
	id, err := uuid.Parse(c.GetString("platform_admin_id"))
	if err != nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// isNonPositive reports whether a numeric amount is <= 0.
func isNonPositive(n pgtype.Numeric) bool {
	if !n.Valid || n.NaN {
		return true
	}
	return n.Int == nil || n.Int.Sign() <= 0
}
