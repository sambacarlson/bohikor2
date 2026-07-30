package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type mockEventsQuerier struct {
	events []db.ListEventsWithUserByCompanyRow
	err    error
}

func (m *mockEventsQuerier) ListEventsWithUserByCompany(ctx context.Context, arg db.ListEventsWithUserByCompanyParams) ([]db.ListEventsWithUserByCompanyRow, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.events == nil {
		return []db.ListEventsWithUserByCompanyRow{}, nil
	}
	return m.events, nil
}

func TestHandleListEvents_Empty(t *testing.T) {
	q := &mockEventsQuerier{}
	r := makeTestGin()
	r.GET("/api/admin/events", HandleListEvents(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/events", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 0 {
		t.Fatalf("expected empty array, got %d items", len(data))
	}
}

func TestHandleListEvents_WithData(t *testing.T) {
	eventID := uuid.New()
	userID := uuid.New()
	q := &mockEventsQuerier{
		events: []db.ListEventsWithUserByCompanyRow{
			{
				ID:        eventID,
				UserID:    pgtype.UUID{Bytes: userID, Valid: true},
				EventType: "payout_success",
				CreatedAt: time.Now().UTC(),
			},
		},
	}
	r := makeTestGin()
	r.GET("/api/admin/events", HandleListEvents(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/events", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(data))
	}
}

func TestHandleListEvents_DBError(t *testing.T) {
	q := &mockEventsQuerier{err: errors.New("db error")}
	r := makeTestGin()
	r.GET("/api/admin/events", HandleListEvents(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/events", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleListEvents_IncludesUserEmail(t *testing.T) {
	eventID := uuid.New()
	userID := uuid.New()
	email := "employee@company.com"
	q := &mockEventsQuerier{
		events: []db.ListEventsWithUserByCompanyRow{
			{
				ID:        eventID,
				UserID:    pgtype.UUID{Bytes: userID, Valid: true},
				EventType: "payout_success",
				UserEmail: pgtype.Text{String: email, Valid: true},
				CreatedAt: time.Now().UTC(),
			},
		},
	}
	r := makeTestGin()
	r.GET("/api/admin/events", HandleListEvents(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/events", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	data, ok := resp["data"].([]interface{})
	if !ok || len(data) != 1 {
		t.Fatalf("expected data array with 1 item, got %v", resp["data"])
	}

	item, ok := data[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected item to be object, got %T", data[0])
	}

	gotEmail, ok := item["user_email"].(string)
	if !ok || gotEmail != email {
		t.Fatalf("expected user_email %q, got %q (type: %T)", email, gotEmail, item["user_email"])
	}
}

func TestHandleListAdminRequests_IncludesUserEmail(t *testing.T) {
	reqID := uuid.New()
	userID := uuid.New()
	email := "employee@company.com"
	q := &mockAdminRequestsQuerier{
		requests: []db.ListAdvanceRequestsWithUserByCompanyRow{
			{
				ID:        reqID,
				UserID:    userID,
				Status:    db.RequestStatusSuccess,
				UserEmail: email,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	r := makeTestGin()
	r.GET("/api/admin/requests", HandleListAdminRequests(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/requests", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	data, ok := resp["data"].([]interface{})
	if !ok || len(data) != 1 {
		t.Fatalf("expected data array with 1 item, got %v", resp["data"])
	}

	item, ok := data[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected item to be object, got %T", data[0])
	}

	gotEmail, ok := item["user_email"].(string)
	if !ok || gotEmail != email {
		t.Fatalf("expected user_email %q, got %q (type: %T)", email, gotEmail, item["user_email"])
	}
}
