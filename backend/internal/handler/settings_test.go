package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type mockSettingsQuerier struct {
	list      []db.Setting
	listErr   error
	upsertErr error
	upserts   []db.UpsertSettingParams
}

func (m *mockSettingsQuerier) ListSettingsByCompany(ctx context.Context, companyID uuid.UUID) ([]db.Setting, error) {
	return m.list, m.listErr
}

func (m *mockSettingsQuerier) UpsertSetting(ctx context.Context, arg db.UpsertSettingParams) (db.Setting, error) {
	m.upserts = append(m.upserts, arg)
	if m.upsertErr != nil {
		return db.Setting{}, m.upsertErr
	}
	// Echo back what was written, like the real upsert.
	return db.Setting{CompanyID: arg.CompanyID, Key: arg.Key, Value: arg.Value, UpdatedBy: arg.UpdatedBy}, nil
}

// makeTestGinWithAdmin sets both company scope and admin_id, as the real
// admin middleware chain does.
func makeTestGinWithAdmin(adminID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("company_id", testCompanyID)
		c.Set("admin_id", adminID)
		c.Next()
	})
	return r
}

func TestCoerceValue(t *testing.T) {
	cases := []struct {
		name string
		key  string
		in   string
		want string
	}{
		{"numeric string to number", "advance_amount_xaf", `"5000"`, `5000`},
		{"numeric already number stays", "daily_request_limit", `3`, `3`},
		{"bool string to bool", "kill_switch_enabled", `"true"`, `true`},
		{"bool already bool stays", "kill_switch_enabled", `false`, `false`},
		{"unknown key untouched", "some_text_key", `"hello"`, `"hello"`},
		{"numeric non-parseable string untouched", "advance_amount_xaf", `"abc"`, `"abc"`},
		{"bool non-parseable string untouched", "kill_switch_enabled", `"maybe"`, `"maybe"`},
		{"float numeric string", "advance_amount_xaf", `"1500.5"`, `1500.5`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := coerceValue(tc.key, json.RawMessage(tc.in))
			if string(got) != tc.want {
				t.Fatalf("coerceValue(%q, %s) = %s, want %s", tc.key, tc.in, got, tc.want)
			}
		})
	}
}

func TestHandleListSettings_Success(t *testing.T) {
	updatedBy := uuid.New()
	q := &mockSettingsQuerier{list: []db.Setting{
		{
			Key:       "advance_amount_xaf",
			Value:     []byte(`5000`),
			UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			Key:       "kill_switch_enabled",
			Value:     []byte(`false`),
			UpdatedAt: time.Now(),
			UpdatedBy: pgtype.UUID{Bytes: updatedBy, Valid: true},
		},
	}}
	r := makeTestGin()
	r.GET("/settings", HandleListSettings(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/settings", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 2 {
		t.Fatalf("expected 2 settings, got %d", len(data))
	}
	first := data[0].(map[string]interface{})
	if first["key"] != "advance_amount_xaf" {
		t.Fatalf("unexpected key: %v", first["key"])
	}
	if first["updated_at"] != "2026-01-02T03:04:05Z" {
		t.Fatalf("unexpected updated_at format: %v", first["updated_at"])
	}
	// First setting has no updater -> omitted.
	if _, present := first["updated_by"]; present {
		t.Fatalf("expected updated_by omitted when null")
	}
	second := data[1].(map[string]interface{})
	if second["updated_by"] != updatedBy.String() {
		t.Fatalf("expected updated_by %s, got %v", updatedBy, second["updated_by"])
	}
}

func TestHandleListSettings_DBError(t *testing.T) {
	q := &mockSettingsQuerier{listErr: errors.New("boom")}
	r := makeTestGin()
	r.GET("/settings", HandleListSettings(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/settings", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleUpdateSettings_Success(t *testing.T) {
	adminID := uuid.New()
	q := &mockSettingsQuerier{}
	r := makeTestGinWithAdmin(adminID.String())
	r.PUT("/settings", HandleUpdateSettings(q))

	body := `{"advance_amount_xaf":"7500","kill_switch_enabled":"true"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/settings", bytes.NewBufferString(body))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(q.upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(q.upserts))
	}
	// Values must be coerced before persisting, and scoped/attributed correctly.
	for _, up := range q.upserts {
		if up.CompanyID != testCompanyID {
			t.Fatalf("upsert not scoped to company")
		}
		if !up.UpdatedBy.Valid || uuid.UUID(up.UpdatedBy.Bytes) != adminID {
			t.Fatalf("upsert not attributed to admin")
		}
		switch up.Key {
		case "advance_amount_xaf":
			if string(up.Value) != "7500" {
				t.Fatalf("expected coerced numeric 7500, got %s", up.Value)
			}
		case "kill_switch_enabled":
			if string(up.Value) != "true" {
				t.Fatalf("expected coerced bool true, got %s", up.Value)
			}
		}
	}
}

func TestHandleUpdateSettings_NoAdmin(t *testing.T) {
	q := &mockSettingsQuerier{}
	r := makeTestGin() // company scope but no admin_id
	r.PUT("/settings", HandleUpdateSettings(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/settings", bytes.NewBufferString(`{}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleUpdateSettings_BadAdminID(t *testing.T) {
	q := &mockSettingsQuerier{}
	r := makeTestGinWithAdmin("not-a-uuid")
	r.PUT("/settings", HandleUpdateSettings(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/settings", bytes.NewBufferString(`{"k":"v"}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleUpdateSettings_BadJSON(t *testing.T) {
	q := &mockSettingsQuerier{}
	r := makeTestGinWithAdmin(uuid.New().String())
	r.PUT("/settings", HandleUpdateSettings(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/settings", bytes.NewBufferString(`not json`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdateSettings_UpsertError(t *testing.T) {
	q := &mockSettingsQuerier{upsertErr: errors.New("boom")}
	r := makeTestGinWithAdmin(uuid.New().String())
	r.PUT("/settings", HandleUpdateSettings(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/settings", bytes.NewBufferString(`{"daily_request_limit":"3"}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
