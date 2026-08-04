package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

var errTestLedger = errors.New("ledger boom")

type mockLedgerQuerier struct {
	balance    pgtype.Numeric
	balanceErr error
	entries    []db.CompanyLedger
	entriesErr error
	lastArg    db.ListLedgerByCompanyParams
}

func (m *mockLedgerQuerier) GetCompanyBalance(ctx context.Context, companyID uuid.UUID) (pgtype.Numeric, error) {
	if m.balanceErr != nil {
		return pgtype.Numeric{}, m.balanceErr
	}
	return m.balance, nil
}

func (m *mockLedgerQuerier) ListLedgerByCompany(ctx context.Context, arg db.ListLedgerByCompanyParams) ([]db.CompanyLedger, error) {
	m.lastArg = arg
	if m.entriesErr != nil {
		return nil, m.entriesErr
	}
	return m.entries, nil
}

func TestHandleGetLedger_Success(t *testing.T) {
	var bal pgtype.Numeric
	_ = bal.Scan("15000.00")

	q := &mockLedgerQuerier{
		balance: bal,
		entries: []db.CompanyLedger{
			{ID: uuid.New(), CompanyID: testCompanyID, EntryType: "topup"},
		},
	}
	r := makeTestGin()
	r.GET("/api/admin/ledger", HandleGetLedger(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/ledger", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["balance_xaf"] != "15000.00" {
		t.Fatalf("expected balance 15000.00, got %v", data["balance_xaf"])
	}
	entries, ok := data["entries"].([]interface{})
	if !ok || len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %v", data["entries"])
	}
}

func TestHandleGetLedger_RespectsPaginationParams(t *testing.T) {
	var bal pgtype.Numeric
	_ = bal.Scan("0")
	q := &mockLedgerQuerier{balance: bal}
	r := makeTestGin()
	r.GET("/api/admin/ledger", HandleGetLedger(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/ledger?page=3&per_page=20", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if q.lastArg.Limit != 20 {
		t.Fatalf("expected limit 20, got %d", q.lastArg.Limit)
	}
	if q.lastArg.Offset != 40 {
		t.Fatalf("expected offset 40 (page 3, per_page 20), got %d", q.lastArg.Offset)
	}
}

func TestHandleGetLedger_ClampsOversizedPerPage(t *testing.T) {
	var bal pgtype.Numeric
	_ = bal.Scan("0")
	q := &mockLedgerQuerier{balance: bal}
	r := makeTestGin()
	r.GET("/api/admin/ledger", HandleGetLedger(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/ledger?per_page=9999", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if q.lastArg.Limit != 50 {
		t.Fatalf("expected per_page above max to fall back to default 50, got %d", q.lastArg.Limit)
	}
}

func TestHandleGetLedger_EmptyEntries(t *testing.T) {
	var bal pgtype.Numeric
	_ = bal.Scan("0")

	q := &mockLedgerQuerier{balance: bal}
	r := makeTestGin()
	r.GET("/api/admin/ledger", HandleGetLedger(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/ledger", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	data := mustUnmarshalData(t, w.Body.Bytes())
	entries, ok := data["entries"].([]interface{})
	if !ok || len(entries) != 0 {
		t.Fatalf("expected empty entries slice, got %v", data["entries"])
	}
}

func TestHandleGetLedger_BalanceError(t *testing.T) {
	q := &mockLedgerQuerier{balanceErr: errTestLedger}
	r := makeTestGin()
	r.GET("/api/admin/ledger", HandleGetLedger(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/ledger", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleGetLedger_ListError(t *testing.T) {
	var bal pgtype.Numeric
	_ = bal.Scan("0")
	q := &mockLedgerQuerier{balance: bal, entriesErr: errTestLedger}
	r := makeTestGin()
	r.GET("/api/admin/ledger", HandleGetLedger(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/ledger", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
