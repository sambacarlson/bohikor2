package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

type mockInvitationsQuerier struct {
	list      []db.Invitation
	err       error
	gotCompID uuid.UUID
}

func (m *mockInvitationsQuerier) ListInvitationsByCompany(ctx context.Context, companyID uuid.UUID) ([]db.Invitation, error) {
	m.gotCompID = companyID
	return m.list, m.err
}

func TestHandleListInvitations_Empty(t *testing.T) {
	q := &mockInvitationsQuerier{list: nil}
	r := makeTestGin()
	r.GET("/inv", HandleListInvitations(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/inv", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if data := mustUnmarshalDataArray(t, w.Body.Bytes()); len(data) != 0 {
		t.Fatalf("expected empty array, got %d", len(data))
	}
	if q.gotCompID != testCompanyID {
		t.Fatalf("expected company scope %v, got %v", testCompanyID, q.gotCompID)
	}
}

func TestHandleListInvitations_WithData(t *testing.T) {
	q := &mockInvitationsQuerier{list: []db.Invitation{
		{ID: uuid.New(), Email: "invitee@x.com", Status: db.InvitationStatusPending},
	}}
	r := makeTestGin()
	r.GET("/inv", HandleListInvitations(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/inv", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if data := mustUnmarshalDataArray(t, w.Body.Bytes()); len(data) != 1 {
		t.Fatalf("expected 1 invitation, got %d", len(data))
	}
}

func TestHandleListInvitations_DBError(t *testing.T) {
	q := &mockInvitationsQuerier{err: errors.New("boom")}
	r := makeTestGin()
	r.GET("/inv", HandleListInvitations(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/inv", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHandleListInvitations_MissingScope(t *testing.T) {
	q := &mockInvitationsQuerier{}
	r := makeTestGinNoScope()
	r.GET("/inv", HandleListInvitations(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/inv", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
