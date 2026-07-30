package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/campay"
)

type mockPhoneQuerier struct {
	user   db.User
	getErr error

	// errNoActiveRequest/Verification simulate "not found" (i.e. nothing active).
	activeRequestErr      error // nil => an active request exists (conflict)
	activeVerificationErr error // nil => an active verification exists (conflict)

	updatePhoneErr error

	createVerif    db.PhoneVerification
	createVerifErr error

	updateStatusResults []db.PhoneVerification
	updateStatusErrs    []error
	updateStatusCalls   int

	latestVerif    db.PhoneVerification
	latestVerifErr error

	events int
}

func (m *mockPhoneQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return m.user, m.getErr
}
func (m *mockPhoneQuerier) UpdatePhoneNumber(ctx context.Context, arg db.UpdatePhoneNumberParams) (db.User, error) {
	if m.updatePhoneErr != nil {
		return db.User{}, m.updatePhoneErr
	}
	u := m.user
	u.PhoneNumber = arg.PhoneNumber
	return u, nil
}
func (m *mockPhoneQuerier) GetActiveRequestByUserID(ctx context.Context, userID uuid.UUID) (db.AdvanceRequest, error) {
	return db.AdvanceRequest{}, m.activeRequestErr
}
func (m *mockPhoneQuerier) CreatePhoneVerification(ctx context.Context, arg db.CreatePhoneVerificationParams) (db.PhoneVerification, error) {
	return m.createVerif, m.createVerifErr
}
func (m *mockPhoneQuerier) GetActivePhoneVerificationByUser(ctx context.Context, userID uuid.UUID) (db.PhoneVerification, error) {
	return db.PhoneVerification{}, m.activeVerificationErr
}
func (m *mockPhoneQuerier) GetLatestPhoneVerificationByUser(ctx context.Context, userID uuid.UUID) (db.PhoneVerification, error) {
	return m.latestVerif, m.latestVerifErr
}
func (m *mockPhoneQuerier) UpdatePhoneVerificationStatus(ctx context.Context, arg db.UpdatePhoneVerificationStatusParams) (db.PhoneVerification, error) {
	i := m.updateStatusCalls
	m.updateStatusCalls++
	var res db.PhoneVerification
	var err error
	if i < len(m.updateStatusResults) {
		res = m.updateStatusResults[i]
	}
	if i < len(m.updateStatusErrs) {
		err = m.updateStatusErrs[i]
	}
	res.Status = arg.Status
	return res, err
}
func (m *mockPhoneQuerier) SetPhoneVerified(ctx context.Context, id uuid.UUID) (db.User, error) {
	return m.user, nil
}
func (m *mockPhoneQuerier) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	m.events++
	return db.Event{}, nil
}

func newPhoneHandler(q phoneQuerier, campayMock *mockCampayTransferer) *PhoneHandler {
	return NewPhoneHandler(q, campayMock, decimal.NewFromInt(100))
}

// activeUser is the baseline: an active user with no in-flight request/verification.
func activePhoneUser(uid uuid.UUID) db.User {
	return db.User{ID: uid, CompanyID: testCompanyID, Status: db.UserStatusActive}
}

func TestAddPhoneNumber_Success(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{
		user:                  activePhoneUser(uid),
		activeRequestErr:      errors.New("no rows"),
		activeVerificationErr: errors.New("no rows"),
		createVerif:           db.PhoneVerification{ID: uuid.New()},
		updateStatusResults:   []db.PhoneVerification{{ID: uuid.New()}},
		updateStatusErrs:      []error{nil},
	}
	campayMock := &mockCampayTransferer{collectResp: &campay.CollectResponse{Reference: "ref-1", UssdCode: "*126#"}}
	h := newPhoneHandler(q, campayMock)
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)

	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, &uid)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if q.events != 1 {
		t.Fatalf("expected an initiated event to be recorded, got %d", q.events)
	}
}

func TestAddPhoneNumber_Unauthenticated(t *testing.T) {
	q := &mockPhoneQuerier{}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(nil)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAddPhoneNumber_MissingPhone(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{user: activePhoneUser(uid)}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{}`, &uid)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddPhoneNumber_UserNotFound(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{getErr: errors.New("no rows")}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, &uid)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAddPhoneNumber_NotActive(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{user: db.User{ID: uid, Status: db.UserStatusSuspended}}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, &uid)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestAddPhoneNumber_RequestInProgress(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{
		user:             activePhoneUser(uid),
		activeRequestErr: nil, // active request exists
	}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, &uid)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 request_in_progress, got %d", w.Code)
	}
}

func TestAddPhoneNumber_VerificationInProgress(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{
		user:                  activePhoneUser(uid),
		activeRequestErr:      errors.New("no rows"),
		activeVerificationErr: nil, // active verification exists
	}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, &uid)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 verification_in_progress, got %d", w.Code)
	}
}

func TestAddPhoneNumber_UpdatePhoneError(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{
		user:                  activePhoneUser(uid),
		activeRequestErr:      errors.New("no rows"),
		activeVerificationErr: errors.New("no rows"),
		updatePhoneErr:        errors.New("boom"),
	}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, &uid)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAddPhoneNumber_CreateVerificationError(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{
		user:                  activePhoneUser(uid),
		activeRequestErr:      errors.New("no rows"),
		activeVerificationErr: errors.New("no rows"),
		createVerifErr:        errors.New("boom"),
	}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, &uid)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAddPhoneNumber_CollectFailsMarksFailed(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{
		user:                  activePhoneUser(uid),
		activeRequestErr:      errors.New("no rows"),
		activeVerificationErr: errors.New("no rows"),
		createVerif:           db.PhoneVerification{ID: uuid.New()},
		updateStatusResults:   []db.PhoneVerification{{}},
		updateStatusErrs:      []error{nil},
	}
	campayMock := &mockCampayTransferer{collectErr: errors.New("gateway down")}
	h := newPhoneHandler(q, campayMock)
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, &uid)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 collect_failed, got %d", w.Code)
	}
	// The verification should have been transitioned to failed.
	if q.updateStatusCalls != 1 {
		t.Fatalf("expected one status update marking failed, got %d", q.updateStatusCalls)
	}
}

func TestAddPhoneNumber_PostCollectUpdateError(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{
		user:                  activePhoneUser(uid),
		activeRequestErr:      errors.New("no rows"),
		activeVerificationErr: errors.New("no rows"),
		createVerif:           db.PhoneVerification{ID: uuid.New()},
		// First status update (pending) fails; second (mark failed) succeeds.
		updateStatusResults: []db.PhoneVerification{{}, {}},
		updateStatusErrs:    []error{errors.New("db down"), nil},
	}
	campayMock := &mockCampayTransferer{collectResp: &campay.CollectResponse{Reference: "ref-1", UssdCode: "*126#"}}
	h := newPhoneHandler(q, campayMock)
	r := ginWithUser(uid)
	r.POST("/phone", h.AddPhoneNumber)
	w := pinRequest(t, r, "POST", "/phone", `{"phone_number":"237600000000"}`, &uid)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if q.updateStatusCalls != 2 {
		t.Fatalf("expected a compensating failed-update, got %d calls", q.updateStatusCalls)
	}
}

func TestGetPhoneVerificationStatus_WithVerification(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{
		user:        db.User{ID: uid, PhoneVerified: true, PhoneNumber: pgtypeText("237600000000")},
		latestVerif: db.PhoneVerification{ID: uuid.New(), Status: db.RequestStatusPending, UssdCode: pgtypeText("*126#")},
	}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.GET("/phone/status", h.GetPhoneVerificationStatus)
	w := pinRequest(t, r, "GET", "/phone/status", ``, &uid)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["phone_verified"] != true {
		t.Fatalf("expected phone_verified true")
	}
	if data["verification"] == nil {
		t.Fatalf("expected verification object")
	}
}

func TestGetPhoneVerificationStatus_NoVerification(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{
		user:           db.User{ID: uid},
		latestVerifErr: errors.New("no rows"),
	}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.GET("/phone/status", h.GetPhoneVerificationStatus)
	w := pinRequest(t, r, "GET", "/phone/status", ``, &uid)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["verification"] != nil {
		t.Fatalf("expected nil verification, got %v", data["verification"])
	}
}

func TestGetPhoneVerificationStatus_Unauthenticated(t *testing.T) {
	q := &mockPhoneQuerier{}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(nil)
	r.GET("/phone/status", h.GetPhoneVerificationStatus)
	w := pinRequest(t, r, "GET", "/phone/status", ``, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetPhoneVerificationStatus_UserNotFound(t *testing.T) {
	uid := uuid.New()
	q := &mockPhoneQuerier{getErr: errors.New("no rows")}
	h := newPhoneHandler(q, &mockCampayTransferer{})
	r := ginWithUser(uid)
	r.GET("/phone/status", h.GetPhoneVerificationStatus)
	w := pinRequest(t, r, "GET", "/phone/status", ``, &uid)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
