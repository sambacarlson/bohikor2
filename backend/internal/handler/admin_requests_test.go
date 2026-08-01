package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/campay"
	"github.com/Iknite-Space/bohikor2/internal/reconciler"
	"github.com/Iknite-Space/bohikor2/internal/service"
)

type mockAdminRequestsStore struct {
	byID                 *db.AdvanceRequest
	byIDErr              error
	user                 *db.User
	getUserErr           error
	reissued             *db.AdvanceRequest
	reissueErr           error
	resolved             *db.AdvanceRequest
	resolveErr           error
	createdEvent         bool
	lastTransitionStatus db.RequestStatus
	transitionErr        error
	transitionOverride   *db.AdvanceRequest
}

func (m *mockAdminRequestsStore) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	if m.getUserErr != nil {
		return db.User{}, m.getUserErr
	}
	if m.user != nil {
		return *m.user, nil
	}
	return db.User{ID: id, PhoneNumber: pgtype.Text{String: "+237600000000", Valid: true}}, nil
}

func (m *mockAdminRequestsStore) Transition(ctx context.Context, req db.AdvanceRequest, newStatus db.RequestStatus, opts service.TransitionOpts) (db.AdvanceRequest, error) {
	m.lastTransitionStatus = newStatus
	if m.transitionErr != nil {
		return db.AdvanceRequest{}, m.transitionErr
	}
	if m.transitionOverride != nil {
		return *m.transitionOverride, nil
	}
	req.Status = newStatus
	if opts.FailureReason != "" {
		req.FailureReason = pgtype.Text{String: opts.FailureReason, Valid: true}
	}
	if opts.CampayPayoutRef != "" {
		req.CampayPayoutRef = pgtype.Text{String: opts.CampayPayoutRef, Valid: true}
	}
	return req, nil
}

func (m *mockAdminRequestsStore) GetAdvanceRequestByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	if m.byIDErr != nil {
		return db.AdvanceRequest{}, m.byIDErr
	}
	if m.byID == nil {
		return db.AdvanceRequest{}, errTestNotFound
	}
	return *m.byID, nil
}

func (m *mockAdminRequestsStore) ReissueAdvanceRequest(ctx context.Context, original db.AdvanceRequest) (db.AdvanceRequest, error) {
	if m.reissueErr != nil {
		return db.AdvanceRequest{}, m.reissueErr
	}
	if m.reissued != nil {
		return *m.reissued, nil
	}
	return db.AdvanceRequest{ID: uuid.New(), CompanyID: original.CompanyID, UserID: original.UserID}, nil
}

func (m *mockAdminRequestsStore) ResolveAdvanceRequest(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	if m.resolveErr != nil {
		return db.AdvanceRequest{}, m.resolveErr
	}
	if m.resolved != nil {
		return *m.resolved, nil
	}
	return db.AdvanceRequest{ID: id}, nil
}

func (m *mockAdminRequestsStore) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	m.createdEvent = true
	return db.Event{ID: uuid.New()}, nil
}

type mockAdminReconciler struct {
	result db.AdvanceRequest
	err    error
}

func (m *mockAdminReconciler) ReconcileByID(ctx context.Context, id uuid.UUID) (db.AdvanceRequest, error) {
	return m.result, m.err
}

func adminRequestsTestGin(store adminRequestsStore, rec adminReconciler, campayClient campayTransferer) (*gin.Engine, *AdminRequestsHandler) {
	h := NewAdminRequestsHandler(store, rec, campayClient)
	r := makeTestGin()
	r.Use(func(c *gin.Context) { c.Set("admin_id", uuid.New().String()); c.Next() })
	r.POST("/api/admin/requests/:id/reconcile", h.Reconcile)
	r.POST("/api/admin/requests/:id/resolve", h.Resolve)
	r.POST("/api/admin/requests/:id/reissue", h.Reissue)
	return r, h
}

func TestAdminReconcile_NoPayoutRef_Returns409(t *testing.T) {
	sameCompany := &db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID}
	r, _ := adminRequestsTestGin(&mockAdminRequestsStore{byID: sameCompany}, &mockAdminReconciler{err: reconciler.ErrNoPayoutRef}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-x"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/reconcile", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminReconcile_Success(t *testing.T) {
	sameCompany := &db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID}
	r, _ := adminRequestsTestGin(&mockAdminRequestsStore{byID: sameCompany}, &mockAdminReconciler{result: db.AdvanceRequest{ID: uuid.New(), Status: db.RequestStatusSuccess}}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-x"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/reconcile", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminReconcile_WrongCompany_Returns404(t *testing.T) {
	otherCompany := &db.AdvanceRequest{ID: uuid.New(), CompanyID: uuid.New()}
	r, _ := adminRequestsTestGin(&mockAdminRequestsStore{byID: otherCompany}, &mockAdminReconciler{result: db.AdvanceRequest{ID: uuid.New(), Status: db.RequestStatusSuccess}}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-x"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/reconcile", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a request belonging to another company, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminResolve_RequiresNote(t *testing.T) {
	sameCompany := &db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID}
	r, _ := adminRequestsTestGin(&mockAdminRequestsStore{byID: sameCompany}, &mockAdminReconciler{}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-x"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/resolve", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without a note, got %d", w.Code)
	}
}

func TestAdminResolve_Success(t *testing.T) {
	sameCompany := &db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID}
	store := &mockAdminRequestsStore{byID: sameCompany}
	r, _ := adminRequestsTestGin(store, &mockAdminReconciler{}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-x"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/resolve", strings.NewReader(`{"note":"checked campay dashboard, funds never moved"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !store.createdEvent {
		t.Fatal("expected an audit event to be created")
	}
}

func TestAdminResolve_WrongCompany_Returns404(t *testing.T) {
	otherCompany := &db.AdvanceRequest{ID: uuid.New(), CompanyID: uuid.New()}
	store := &mockAdminRequestsStore{byID: otherCompany}
	r, _ := adminRequestsTestGin(store, &mockAdminReconciler{}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-x"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/resolve", strings.NewReader(`{"note":"checked campay dashboard, funds never moved"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a request belonging to another company, got %d: %s", w.Code, w.Body.String())
	}
	if store.createdEvent {
		t.Fatal("did not expect an audit event for a cross-tenant resolve attempt")
	}
}

func TestAdminResolve_NotFlagged_Returns409(t *testing.T) {
	sameCompany := &db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID}
	store := &mockAdminRequestsStore{byID: sameCompany, resolveErr: pgx.ErrNoRows}
	r, _ := adminRequestsTestGin(store, &mockAdminReconciler{}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-x"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+uuid.NewString()+"/resolve", strings.NewReader(`{"note":"checked, was never flagged"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a resolve on a non-flagged row, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminReissue_OnlyFailedOriginal(t *testing.T) {
	original := db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID, Status: db.RequestStatusPending}
	r, _ := adminRequestsTestGin(&mockAdminRequestsStore{byID: &original}, &mockAdminReconciler{}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-x"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+original.ID.String()+"/reissue", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a non-failed original, got %d", w.Code)
	}
}

func TestAdminReissue_Success(t *testing.T) {
	original := db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID, Status: db.RequestStatusFailed}
	store := &mockAdminRequestsStore{byID: &original}
	r, _ := adminRequestsTestGin(store, &mockAdminReconciler{}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-reissue"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+original.ID.String()+"/reissue", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if !store.createdEvent {
		t.Fatal("expected an audit event to be created")
	}
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["status"] != string(db.RequestStatusSuccess) {
		t.Fatalf("expected status success, got %v", data["status"])
	}
}

func TestAdminReissue_CampayDeclined_Returns502(t *testing.T) {
	original := db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID, Status: db.RequestStatusFailed}
	store := &mockAdminRequestsStore{byID: &original}
	r, _ := adminRequestsTestGin(store, &mockAdminReconciler{}, &mockCampayTransferer{transferErr: errors.New("declined"), transferResp: &campay.TransferResponse{Status: "FAILED", Reference: "ref-declined"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+original.ID.String()+"/reissue", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
	if !store.createdEvent {
		t.Fatal("expected an audit event to still be created (reissue+debit happened; only payout failed)")
	}
}

func TestAdminReissue_CampayTransportError_Returns202(t *testing.T) {
	original := db.AdvanceRequest{ID: uuid.New(), CompanyID: testCompanyID, Status: db.RequestStatusFailed}
	store := &mockAdminRequestsStore{byID: &original}
	r, _ := adminRequestsTestGin(store, &mockAdminReconciler{}, &mockCampayTransferer{transferErr: errors.New("timeout"), transferResp: nil})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+original.ID.String()+"/reissue", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminReissue_WrongCompany_Returns404(t *testing.T) {
	original := db.AdvanceRequest{ID: uuid.New(), CompanyID: uuid.New(), Status: db.RequestStatusFailed}
	store := &mockAdminRequestsStore{byID: &original}
	r, _ := adminRequestsTestGin(store, &mockAdminReconciler{}, &mockCampayTransferer{transferResp: &campay.TransferResponse{Status: "SUCCESSFUL", Reference: "ref-x"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/admin/requests/"+original.ID.String()+"/reissue", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a request belonging to another company, got %d: %s", w.Code, w.Body.String())
	}
	if store.createdEvent {
		t.Fatal("did not expect an audit event for a cross-tenant reissue attempt")
	}
}
