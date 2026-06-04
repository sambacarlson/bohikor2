package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/campay"
)

var errTestNotFound = errors.New("not found")

type mockAdvanceQuerier struct {
	user           *db.User
	activeRequest  *db.AdvanceRequest
	createdRequest *db.AdvanceRequest
	updatedRequest *db.AdvanceRequest
	requests       []db.AdvanceRequest
	getUserErr     error
	getActiveErr   error
	createErr      error
	updateErr      error
	listErr        error
	countToday     int64
	countThisMonth int64
}

func (m *mockAdvanceQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	if m.getUserErr != nil {
		return db.User{}, m.getUserErr
	}
	if m.user == nil {
		return db.User{}, errTestNotFound
	}
	return *m.user, nil
}

func (m *mockAdvanceQuerier) GetActiveRequestByUserID(ctx context.Context, userID uuid.UUID) (db.AdvanceRequest, error) {
	if m.getActiveErr != nil {
		return db.AdvanceRequest{}, m.getActiveErr
	}
	if m.activeRequest == nil {
		return db.AdvanceRequest{}, errTestNotFound
	}
	return *m.activeRequest, nil
}

func (m *mockAdvanceQuerier) CreateAdvanceRequest(ctx context.Context, arg db.CreateAdvanceRequestParams) (db.AdvanceRequest, error) {
	if m.createErr != nil {
		return db.AdvanceRequest{}, m.createErr
	}
	if m.createdRequest != nil {
		return *m.createdRequest, nil
	}
	req := db.AdvanceRequest{
		ID:        uuid.New(),
		UserID:    arg.UserID,
		AmountXaf: arg.AmountXaf,
		Status:    arg.Status,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	return req, nil
}

func (m *mockAdvanceQuerier) UpdateAdvanceRequestStatus(ctx context.Context, arg db.UpdateAdvanceRequestStatusParams) (db.AdvanceRequest, error) {
	if m.updateErr != nil {
		return db.AdvanceRequest{}, m.updateErr
	}
	if m.updatedRequest != nil {
		return *m.updatedRequest, nil
	}
	return db.AdvanceRequest{ID: arg.ID, Status: arg.Status}, nil
}

func (m *mockAdvanceQuerier) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	return db.Event{ID: uuid.New()}, nil
}

func (m *mockAdvanceQuerier) ListAdvanceRequestsByUserID(ctx context.Context, userID uuid.UUID) ([]db.AdvanceRequest, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if m.requests == nil {
		return []db.AdvanceRequest{}, nil
	}
	return m.requests, nil
}

func (m *mockAdvanceQuerier) CountAdvanceRequestsByUserToday(ctx context.Context, userID uuid.UUID) (int64, error) {
	return m.countToday, nil
}

func (m *mockAdvanceQuerier) CountSuccessfulAdvanceRequestsByUserThisMonth(ctx context.Context, userID uuid.UUID) (int64, error) {
	return m.countThisMonth, nil
}

type mockAdvanceSettingsQuerier struct {
	settings []db.Setting
	err      error
}

func defaultMockSettings() []db.Setting {
	return []db.Setting{
		{Key: "kill_switch_enabled", Value: []byte("false")},
		{Key: "request_window_start_day", Value: []byte("1")},
		{Key: "request_window_end_day", Value: []byte("31")},
		{Key: "daily_request_limit", Value: []byte("1")},
		{Key: "monthly_request_limit", Value: []byte("3")},
		{Key: "advance_amount_xaf", Value: []byte("10000")},
	}
}

func (m *mockAdvanceSettingsQuerier) ListSettings(ctx context.Context) ([]db.Setting, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.settings == nil {
		return defaultMockSettings(), nil
	}
	return m.settings, nil
}

func (m *mockAdvanceSettingsQuerier) GetSetting(ctx context.Context, key string) (db.Setting, error) {
	if m.settings == nil {
		for _, s := range defaultMockSettings() {
			if s.Key == key {
				return s, nil
			}
		}
	}
	for _, s := range m.settings {
		if s.Key == key {
			return s, nil
		}
	}
	return db.Setting{}, errTestNotFound
}

type mockCampayTransferer struct {
	resp *campay.TransferResponse
	err  error
}

func (m *mockCampayTransferer) InitiateTransfer(ctx context.Context, phoneNumber string, amount decimal.Decimal, description string, externalRef string) (*campay.TransferResponse, error) {
	return m.resp, m.err
}

func makeTestGin() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func setUserContext(c *gin.Context, userID uuid.UUID) {
	c.Set("user_id", userID)
	c.Set("subject_id", userID.String())
	c.Set("subject_type", "user")
}

func mustUnmarshalData(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", resp)
	}
	return data
}

func mustUnmarshalDataArray(t *testing.T, body []byte) []interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array, got %v", resp["data"])
	}
	return data
}

func TestCreateRequest_NotTermsAccepted(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  userID,
			IsTermsAccepted:     false,
			PhoneVerified:       true,
			PhoneNumber:         pgtype.Text{String: "+237600000000", Valid: true},
			Status:              db.UserStatusActive,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
	}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCreateRequest_ActiveRequestExists(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  userID,
			IsTermsAccepted:     true,
			PhoneVerified:       true,
			PhoneNumber:         pgtype.Text{String: "+237600000000", Valid: true},
			Status:              db.UserStatusActive,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
		activeRequest: &db.AdvanceRequest{
			ID:     uuid.New(),
			UserID: userID,
			Status: db.RequestStatusInitiated,
		},
	}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	_ = h
	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateRequest_PhoneNotVerified(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  userID,
			IsTermsAccepted:     true,
			PhoneVerified:       false,
			PhoneNumber:         pgtype.Text{Valid: false},
			Status:              db.UserStatusActive,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
	}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCreateRequest_TransferSuccess(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  userID,
			IsTermsAccepted:     true,
			PhoneVerified:       true,
			PhoneNumber:         pgtype.Text{String: "+237600000000", Valid: true},
			Status:              db.UserStatusActive,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
	}
	transferMock := &mockCampayTransferer{
		resp: &campay.TransferResponse{
			Reference: "campay-ref-success",
			Status:    "SUCCESSFUL",
		},
	}
	h := NewAdvanceHandler(q, transferMock, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["status"] != string(db.RequestStatusSuccess) {
		t.Fatalf("expected status success, got %v", data["status"])
	}
}

func TestCreateRequest_TransferPending(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  userID,
			IsTermsAccepted:     true,
			PhoneVerified:       true,
			PhoneNumber:         pgtype.Text{String: "+237600000000", Valid: true},
			Status:              db.UserStatusActive,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
	}
	transferMock := &mockCampayTransferer{
		resp: &campay.TransferResponse{
			Reference: "campay-ref-pending",
			Status:    "PENDING",
		},
	}
	h := NewAdvanceHandler(q, transferMock, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["status"] != string(db.RequestStatusPending) {
		t.Fatalf("expected status PENDING, got %v", data["status"])
	}
}

func TestCreateRequest_TransferFailed(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  userID,
			IsTermsAccepted:     true,
			PhoneVerified:       true,
			PhoneNumber:         pgtype.Text{String: "+237600000000", Valid: true},
			Status:              db.UserStatusActive,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
	}
	transferMock := &mockCampayTransferer{
		err: errors.New("transfer failed: insufficient funds"),
	}
	h := NewAdvanceHandler(q, transferMock, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

func TestCreateRequest_RequireActiveUser_ShouldBeEnforcedByMiddleware(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  userID,
			IsTermsAccepted:     true,
			PhoneVerified:       true,
			PhoneNumber:         pgtype.Text{String: "+237600000000", Valid: true},
			Status:              db.UserStatusSuspended,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
	}
	transferMock := &mockCampayTransferer{
		resp: &campay.TransferResponse{
			Reference: "campay-ref",
			Status:    "SUCCESSFUL",
		},
	}
	h := NewAdvanceHandler(q, transferMock, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.CreateRequest(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestListUserRequests_Empty(t *testing.T) {
	userID := uuid.New()
	q := &mockAdvanceQuerier{}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.GET("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.ListUserRequests(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/advance-requests", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 0 {
		t.Fatalf("expected empty array, got %d items", len(data))
	}
}

func TestListUserRequests_WithData(t *testing.T) {
	userID := uuid.New()
	reqID := uuid.New()
	q := &mockAdvanceQuerier{
		requests: []db.AdvanceRequest{
			{ID: reqID, UserID: userID, Status: db.RequestStatusSuccess, CreatedAt: time.Now().UTC()},
		},
	}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.GET("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, userID)
		h.ListUserRequests(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/advance-requests", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(data))
	}
}

func TestAcceptTerms_Success(t *testing.T) {
	userID := uuid.New()
	q := &mockTermsQuerier{
		user: &db.User{
			ID:              userID,
			IsTermsAccepted: true,
		},
	}

	r := makeTestGin()
	r.PUT("/api/users/terms", func(c *gin.Context) {
		c.Set("user_id", userID)
		HandleAcceptTerms(q)(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/users/terms",
		strings.NewReader(`{"version":"v1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["is_terms_accepted"] != true {
		t.Fatalf("expected is_terms_accepted true, got %v", data["is_terms_accepted"])
	}
}

func TestAcceptTerms_MissingVersion(t *testing.T) {
	userID := uuid.New()
	q := &mockTermsQuerier{}

	r := makeTestGin()
	r.PUT("/api/users/terms", func(c *gin.Context) {
		c.Set("user_id", userID)
		HandleAcceptTerms(q)(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/users/terms",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

type mockTermsQuerier struct {
	user *db.User
	err  error
}

func (m *mockTermsQuerier) UpdateTermsAcceptance(ctx context.Context, arg db.UpdateTermsAcceptanceParams) (db.User, error) {
	if m.err != nil {
		return db.User{}, m.err
	}
	if m.user != nil {
		return *m.user, nil
	}
	return db.User{}, errors.New("not found")
}

type mockWebhookQuerier struct {
	request   *db.AdvanceRequest
	getErr    error
	updateErr error
}

func (m *mockWebhookQuerier) GetAdvanceRequestByCampayRef(ctx context.Context, campayPayoutRef pgtype.Text) (db.AdvanceRequest, error) {
	if m.getErr != nil {
		return db.AdvanceRequest{}, m.getErr
	}
	if m.request == nil {
		return db.AdvanceRequest{}, errTestNotFound
	}
	return *m.request, nil
}

func (m *mockWebhookQuerier) UpdateAdvanceRequestStatus(ctx context.Context, arg db.UpdateAdvanceRequestStatusParams) (db.AdvanceRequest, error) {
	if m.updateErr != nil {
		return db.AdvanceRequest{}, m.updateErr
	}
	return db.AdvanceRequest{ID: arg.ID, Status: arg.Status}, nil
}

func (m *mockWebhookQuerier) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	return db.Event{ID: uuid.New()}, nil
}

func (m *mockWebhookQuerier) GetPhoneVerificationByCampayRef(ctx context.Context, campayPayoutRef pgtype.Text) (db.PhoneVerification, error) {
	return db.PhoneVerification{}, errTestNotFound
}

func (m *mockWebhookQuerier) UpdatePhoneVerificationStatus(ctx context.Context, arg db.UpdatePhoneVerificationStatusParams) (db.PhoneVerification, error) {
	return db.PhoneVerification{}, nil
}

func (m *mockWebhookQuerier) SetPhoneVerified(ctx context.Context, id uuid.UUID) (db.User, error) {
	return db.User{}, nil
}

type mockWebhookVerifier struct {
	valid bool
}

func (m *mockWebhookVerifier) VerifyWebhook(token string) bool {
	return m.valid
}

func TestWebhook_MissingSignatureField(t *testing.T) {
	v := &mockWebhookVerifier{valid: false}
	q := &mockWebhookQuerier{}
	h := NewWebhookHandler(q, v)

	r := makeTestGin()
	r.POST("/v1/webhooks/campay", h.HandleCampayWebhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/v1/webhooks/campay",
		strings.NewReader(`{"reference":"ref-1","status":"SUCCESSFUL"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestWebhook_InvalidSignature(t *testing.T) {
	v := &mockWebhookVerifier{valid: false}
	q := &mockWebhookQuerier{}
	h := NewWebhookHandler(q, v)

	r := makeTestGin()
	r.POST("/v1/webhooks/campay", h.HandleCampayWebhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/v1/webhooks/campay",
		strings.NewReader(`{"reference":"ref-1","status":"SUCCESSFUL","signature":"invalid-jwt"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestWebhook_SuccessStatus(t *testing.T) {
	reqID := uuid.New()
	userID := uuid.New()
	q := &mockWebhookQuerier{
		request: &db.AdvanceRequest{
			ID:        reqID,
			UserID:    userID,
			Status:    db.RequestStatusPending,
			CreatedAt: time.Now().UTC().Add(-1 * time.Minute),
		},
	}
	v := &mockWebhookVerifier{valid: true}
	h := NewWebhookHandler(q, v)

	r := makeTestGin()
	r.POST("/v1/webhooks/campay", h.HandleCampayWebhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/v1/webhooks/campay",
		strings.NewReader(`{"reference":"ref-1","status":"SUCCESSFUL","signature":"valid-jwt"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhook_FailedStatus(t *testing.T) {
	reqID := uuid.New()
	userID := uuid.New()
	q := &mockWebhookQuerier{
		request: &db.AdvanceRequest{
			ID:        reqID,
			UserID:    userID,
			Status:    db.RequestStatusPending,
			CreatedAt: time.Now().UTC().Add(-2 * time.Minute),
		},
	}
	v := &mockWebhookVerifier{valid: true}
	h := NewWebhookHandler(q, v)

	r := makeTestGin()
	r.POST("/v1/webhooks/campay", h.HandleCampayWebhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/v1/webhooks/campay",
		strings.NewReader(`{"reference":"ref-1","status":"FAILED","signature":"valid-jwt","reason":"insufficient balance"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhook_UnknownReference(t *testing.T) {
	q := &mockWebhookQuerier{}
	v := &mockWebhookVerifier{valid: true}
	h := NewWebhookHandler(q, v)

	r := makeTestGin()
	r.POST("/v1/webhooks/campay", h.HandleCampayWebhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/v1/webhooks/campay",
		strings.NewReader(`{"reference":"unknown-ref","status":"SUCCESSFUL","signature":"valid-jwt"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown ref, got %d", w.Code)
	}
}

func TestWebhook_EmptyReference(t *testing.T) {
	q := &mockWebhookQuerier{}
	v := &mockWebhookVerifier{valid: true}
	h := NewWebhookHandler(q, v)

	r := makeTestGin()
	r.POST("/v1/webhooks/campay", h.HandleCampayWebhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/v1/webhooks/campay",
		strings.NewReader(`{"reference":"","status":"SUCCESSFUL","signature":"valid-jwt"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestWebhook_InvalidJSON(t *testing.T) {
	q := &mockWebhookQuerier{}
	v := &mockWebhookVerifier{valid: true}
	h := NewWebhookHandler(q, v)

	r := makeTestGin()
	r.POST("/v1/webhooks/campay", h.HandleCampayWebhook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/v1/webhooks/campay",
		strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleListAdminRequests_Empty(t *testing.T) {
	q := &mockAdminRequestsQuerier{}
	r := makeTestGin()
	r.GET("/api/admin/requests", HandleListAdminRequests(q))

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/requests", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 0 {
		t.Fatalf("expected empty array, got %d items", len(data))
	}
}

func TestHandleListAdminRequests_WithData(t *testing.T) {
	reqID := uuid.New()
	userID := uuid.New()
	email := "user@example.com"
	q := &mockAdminRequestsQuerier{
		requests: []db.ListAdvanceRequestsWithUserRow{
			{ID: reqID, UserID: userID, Status: db.RequestStatusSuccess, UserEmail: email, CreatedAt: time.Now().UTC()},
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

	data := mustUnmarshalDataArray(t, w.Body.Bytes())
	if len(data) != 1 {
		t.Fatalf("expected 1 item, got %d", len(data))
	}
}

type mockAdminRequestsQuerier struct {
	requests []db.ListAdvanceRequestsWithUserRow
	err      error
}

func (m *mockAdminRequestsQuerier) ListAdvanceRequestsWithUser(ctx context.Context, arg db.ListAdvanceRequestsWithUserParams) ([]db.ListAdvanceRequestsWithUserRow, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.requests == nil {
		return []db.ListAdvanceRequestsWithUserRow{}, nil
	}
	return m.requests, nil
}

func TestRequireActiveUser_Unauthenticated(t *testing.T) {
	q := &mockAdvanceQuerier{
		user: &db.User{
			ID:                  uuid.New(),
			IsTermsAccepted:     false,
			PhoneVerified:       true,
			PhoneNumber:         pgtype.Text{String: "+237600000000", Valid: true},
			Status:              db.UserStatusActive,
			PinHash:             pgtype.Text{Valid: false},
			FailedLoginAttempts: 0,
			LockedUntil:         sql.NullTime{},
		},
	}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)

	r := makeTestGin()
	r.POST("/api/advance-requests", h.CreateRequest)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
