package handler

import (
	"context"
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

func eligibleUser(uid uuid.UUID) *db.User {
	return &db.User{
		ID:              uid,
		CompanyID:       testCompanyID,
		Status:          db.UserStatusActive,
		IsTermsAccepted: true,
		PhoneVerified:   true,
		PhoneNumber:     pgtype.Text{String: "237600000000", Valid: true},
	}
}

func runEligibility(h *AdvanceHandler, uid uuid.UUID) *httptest.ResponseRecorder {
	r := makeTestGin()
	r.GET("/eligibility", func(c *gin.Context) {
		setUserContext(c, uid)
		h.GetEligibility(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/eligibility", nil)
	r.ServeHTTP(w, req)
	return w
}

func TestGetEligibility_Eligible(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid)}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	w := runEligibility(h, uid)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["eligible"] != true {
		t.Fatalf("expected eligible=true, got %v (reasons %v)", data["eligible"], data["reasons"])
	}
}

func TestGetEligibility_KillSwitchBlocks(t *testing.T) {
	uid := uuid.New()
	settings := &mockAdvanceSettingsQuerier{settings: []db.Setting{
		{Key: "kill_switch_enabled", Value: []byte("true")},
		{Key: "request_window_start_day", Value: []byte("1")},
		{Key: "request_window_end_day", Value: []byte("31")},
		{Key: "daily_request_limit", Value: []byte("1")},
		{Key: "monthly_request_limit", Value: []byte("3")},
		{Key: "advance_amount_xaf", Value: []byte("10000")},
	}}
	q := &mockAdvanceQuerier{user: eligibleUser(uid)}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, settings, time.UTC)
	w := runEligibility(h, uid)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["eligible"] != false || data["kill_switch_active"] != true {
		t.Fatalf("kill switch should block eligibility, got %v", data)
	}
}

func TestGetEligibility_TermsAndPhoneMissing(t *testing.T) {
	uid := uuid.New()
	u := &db.User{ID: uid, CompanyID: testCompanyID, Status: db.UserStatusActive, IsTermsAccepted: false, PhoneVerified: false}
	q := &mockAdvanceQuerier{user: u}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	w := runEligibility(h, uid)
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["eligible"] != false {
		t.Fatalf("expected not eligible")
	}
	reasons, _ := data["reasons"].([]interface{})
	joined := ""
	for _, r := range reasons {
		joined += r.(string) + "|"
	}
	if !strings.Contains(joined, "terms not accepted") || !strings.Contains(joined, "phone not verified") {
		t.Fatalf("expected terms+phone reasons, got %q", joined)
	}
}

func TestGetEligibility_ActiveRequestBlocks(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid), activeRequest: &db.AdvanceRequest{ID: uuid.New()}}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	w := runEligibility(h, uid)
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["eligible"] != false {
		t.Fatalf("an active request must block eligibility")
	}
}

func TestGetEligibility_Unauthenticated(t *testing.T) {
	q := &mockAdvanceQuerier{}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.GET("/eligibility", h.GetEligibility) // no user_id set
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/eligibility", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetEligibility_UserNotFound(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{getUserErr: errors.New("no rows")}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	w := runEligibility(h, uid)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetEligibility_SettingsError(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid)}
	settings := &mockAdvanceSettingsQuerier{err: errors.New("boom")}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, settings, time.UTC)
	w := runEligibility(h, uid)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------- phone-verification webhook branch ----------

func postWebhook(h *webhookHandler, body string) *httptest.ResponseRecorder {
	r := makeTestGin()
	r.POST("/v1/webhooks/campay", h.HandleCampayWebhook)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/v1/webhooks/campay", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestPhoneWebhook_SuccessSetsVerified(t *testing.T) {
	q := &mockWebhookQuerier{phoneVerif: &db.PhoneVerification{
		ID: uuid.New(), UserID: uuid.New(), CompanyID: testCompanyID, Status: db.RequestStatusPending,
	}}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})
	w := postWebhook(h, `{"reference":"pv-ref","status":"SUCCESSFUL","signature":"valid-jwt"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !q.setVerifiedCalled {
		t.Fatalf("SUCCESSFUL phone verification webhook must set phone verified")
	}
	if q.phoneEvents != 1 {
		t.Fatalf("expected a phone_verified event, got %d", q.phoneEvents)
	}
}

func TestPhoneWebhook_FailedStatus(t *testing.T) {
	q := &mockWebhookQuerier{phoneVerif: &db.PhoneVerification{
		ID: uuid.New(), UserID: uuid.New(), Status: db.RequestStatusPending,
	}}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})
	w := postWebhook(h, `{"reference":"pv-ref","status":"FAILED","signature":"valid-jwt","reason":"user declined"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if q.setVerifiedCalled {
		t.Fatalf("FAILED verification must not set phone verified")
	}
}

func TestPhoneWebhook_UnknownStatus(t *testing.T) {
	q := &mockWebhookQuerier{phoneVerif: &db.PhoneVerification{ID: uuid.New()}}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})
	w := postWebhook(h, `{"reference":"pv-ref","status":"WEIRD","signature":"valid-jwt"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown status, got %d", w.Code)
	}
}

func TestPhoneWebhook_UpdateError(t *testing.T) {
	q := &mockWebhookQuerier{
		phoneVerif:     &db.PhoneVerification{ID: uuid.New(), Status: db.RequestStatusPending},
		phoneUpdateErr: errors.New("db down"),
	}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})
	w := postWebhook(h, `{"reference":"pv-ref","status":"SUCCESSFUL","signature":"valid-jwt"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestPhoneWebhook_SetVerifiedError(t *testing.T) {
	q := &mockWebhookQuerier{
		phoneVerif:     &db.PhoneVerification{ID: uuid.New(), UserID: uuid.New(), Status: db.RequestStatusPending},
		setVerifiedErr: errors.New("db down"),
	}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})
	w := postWebhook(h, `{"reference":"pv-ref","status":"SUCCESSFUL","signature":"valid-jwt"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------- ordinal / itoa ----------

func TestOrdinal(t *testing.T) {
	cases := map[int]string{
		0: "0th", 1: "1st", 2: "2nd", 3: "3rd", 4: "4th",
		11: "11th", 12: "12th", 13: "13th",
		21: "21st", 22: "22nd", 23: "23rd",
		100: "100th", 101: "101st", 111: "111th",
	}
	for n, want := range cases {
		if got := ordinal(n); got != want {
			t.Errorf("ordinal(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestItoaNegative(t *testing.T) {
	if got := itoa(-5); got != "-5" {
		t.Fatalf("itoa(-5) = %q, want -5", got)
	}
}

// ---------- parseJSONFloat / parseJSONBool ----------

func TestParseJSONFloat(t *testing.T) {
	cases := []struct {
		raw    string
		want   float64
		wantOK bool
	}{
		{`5000`, 5000, true},
		{`"7500"`, 7500, true},
		{`"1500.5"`, 1500.5, true},
		{`"abc"`, 0, false},
		{`true`, 0, false},
	}
	for _, tc := range cases {
		got, ok := parseJSONFloat([]byte(tc.raw))
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Errorf("parseJSONFloat(%s) = %v,%v want %v,%v", tc.raw, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestParseJSONBool(t *testing.T) {
	cases := []struct {
		raw    string
		want   bool
		wantOK bool
	}{
		{`true`, true, true},
		{`false`, false, true},
		{`"true"`, true, true},
		{`"false"`, false, true},
		{`"maybe"`, false, false},
		{`5`, false, false},
	}
	for _, tc := range cases {
		got, ok := parseJSONBool([]byte(tc.raw))
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Errorf("parseJSONBool(%s) = %v,%v want %v,%v", tc.raw, got, ok, tc.want, tc.wantOK)
		}
	}
}

// ---------- CreateRequest branch coverage ----------

func runCreateRequest(h *AdvanceHandler, uid uuid.UUID, body string) *httptest.ResponseRecorder {
	r := makeTestGin()
	r.POST("/api/advance-requests", func(c *gin.Context) {
		setUserContext(c, uid)
		h.CreateRequest(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestCreateRequest_Unauthenticated(t *testing.T) {
	q := &mockAdvanceQuerier{}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.POST("/api/advance-requests", h.CreateRequest) // no user_id
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreateRequest_UserNotFound(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{getUserErr: errors.New("no rows")}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	if w := runCreateRequest(h, uid, `{}`); w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCreateRequest_NotActive(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: &db.User{ID: uid, Status: db.UserStatusSuspended}}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	if w := runCreateRequest(h, uid, `{}`); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCreateRequest_SettingsError(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid)}
	settings := &mockAdvanceSettingsQuerier{err: errors.New("boom")}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, settings, time.UTC)
	if w := runCreateRequest(h, uid, `{}`); w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCreateRequest_KillSwitch(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid)}
	settings := &mockAdvanceSettingsQuerier{settings: []db.Setting{
		{Key: "kill_switch_enabled", Value: []byte("true")},
	}}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, settings, time.UTC)
	w := runCreateRequest(h, uid, `{}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if got := codeOf(t, w.Body.Bytes()); got != "kill_switch_active" {
		t.Fatalf("expected kill_switch_active, got %q", got)
	}
}

func TestCreateRequest_OutsideWindow(t *testing.T) {
	uid := uuid.New()
	// Choose a one-day window that is guaranteed not to be today.
	today := time.Now().In(time.UTC).Day()
	otherDay := today%28 + 1
	if otherDay == today {
		otherDay = otherDay%28 + 1
	}
	q := &mockAdvanceQuerier{user: eligibleUser(uid)}
	settings := &mockAdvanceSettingsQuerier{settings: []db.Setting{
		{Key: "request_window_start_day", Value: []byte(itoa(otherDay))},
		{Key: "request_window_end_day", Value: []byte(itoa(otherDay))},
	}}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, settings, time.UTC)
	w := runCreateRequest(h, uid, `{}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "outside_request_window" {
		t.Fatalf("expected 403 outside_request_window, got %d %s", w.Code, w.Body.String())
	}
}

func TestCreateRequest_DailyLimitReached(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid), countToday: 1}
	settings := &mockAdvanceSettingsQuerier{settings: []db.Setting{
		{Key: "daily_request_limit", Value: []byte("1")},
	}}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, settings, time.UTC)
	w := runCreateRequest(h, uid, `{}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "daily_limit_reached" {
		t.Fatalf("expected 403 daily_limit_reached, got %d %s", w.Code, w.Body.String())
	}
}

func TestCreateRequest_MonthlyLimitReached(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid), countThisMonth: 3}
	settings := &mockAdvanceSettingsQuerier{settings: []db.Setting{
		{Key: "monthly_request_limit", Value: []byte("3")},
	}}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, settings, time.UTC)
	w := runCreateRequest(h, uid, `{}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "monthly_limit_reached" {
		t.Fatalf("expected 403 monthly_limit_reached, got %d %s", w.Code, w.Body.String())
	}
}

func TestCreateRequest_InsufficientFloat_Returns403(t *testing.T) {
	uid := uuid.New()
	var lowBalance pgtype.Numeric
	if err := lowBalance.Scan("100"); err != nil {
		t.Fatalf("scan balance: %v", err)
	}
	q := &mockAdvanceQuerier{user: eligibleUser(uid), balance: lowBalance}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	w := runCreateRequest(h, uid, `{}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "insufficient_employer_float" {
		t.Fatalf("expected 403 insufficient_employer_float, got %d %s", w.Code, w.Body.String())
	}
}

func TestGetEligibility_InsufficientFloatBlocks(t *testing.T) {
	uid := uuid.New()
	var lowBalance pgtype.Numeric
	if err := lowBalance.Scan("100"); err != nil {
		t.Fatalf("scan balance: %v", err)
	}
	q := &mockAdvanceQuerier{user: eligibleUser(uid), balance: lowBalance}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	w := runEligibility(h, uid)
	data := mustUnmarshalData(t, w.Body.Bytes())
	if data["eligible"] != false {
		t.Fatalf("expected not eligible when employer float is insufficient")
	}
	reasons, _ := data["reasons"].([]interface{})
	found := false
	for _, r := range reasons {
		if r.(string) == "insufficient_employer_float" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected insufficient_employer_float reason, got %v", reasons)
	}
}

func TestCreateRequest_GetCompanyBalanceError(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid), balanceErr: errors.New("db down")}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	w := runCreateRequest(h, uid, `{}`)
	if w.Code != http.StatusInternalServerError || codeOf(t, w.Body.Bytes()) != "internal_error" {
		t.Fatalf("expected 500 internal_error when GetCompanyBalance fails, got %d %s", w.Code, w.Body.String())
	}
}

func TestCreateRequest_ParseCompanyBalanceError(t *testing.T) {
	uid := uuid.New()
	// A NaN numeric scans as Valid with no error, but numericToDecimal's
	// downstream decimal.NewFromString("NaN") fails to parse it — this is the
	// cheapest way to reach the "parse company balance" error branch, which is
	// otherwise unreachable via a plain DB error (that's balanceErr, covered above).
	unparseable := pgtype.Numeric{Valid: true, NaN: true}
	q := &mockAdvanceQuerier{user: eligibleUser(uid), balance: unparseable}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	w := runCreateRequest(h, uid, `{}`)
	if w.Code != http.StatusInternalServerError || codeOf(t, w.Body.Bytes()) != "internal_error" {
		t.Fatalf("expected 500 internal_error when company balance can't be parsed, got %d %s", w.Code, w.Body.String())
	}
}

func TestNumericToDecimal_InvalidReturnsZero(t *testing.T) {
	d, err := numericToDecimal(pgtype.Numeric{Valid: false})
	if err != nil {
		t.Fatalf("expected no error for an invalid numeric, got %v", err)
	}
	if !d.Equal(decimal.Zero) {
		t.Fatalf("expected decimal.Zero for an invalid numeric, got %s", d.String())
	}
}

func TestCreateRequest_CreateError(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid), createErr: errors.New("db down")}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	if w := runCreateRequest(h, uid, `{}`); w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCreateRequest_PostTransferUpdateError(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{
		user:      eligibleUser(uid),
		updateErr: errors.New("db down"),
	}
	campayMock := &mockCampayTransferer{transferResp: &campay.TransferResponse{Reference: "ref-1", Status: "SUCCESSFUL"}}
	h := NewAdvanceHandler(q, campayMock, &mockAdvanceSettingsQuerier{}, time.UTC)
	w := runCreateRequest(h, uid, `{}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on post-transfer update failure, got %d: %s", w.Code, w.Body.String())
	}
	// Campay already confirmed SUCCESSFUL — the fallback must land on `processing`,
	// never `failed`, or TransitionRequest would post a ledger reversal for a
	// payout that actually went through (see task-3 review finding 1).
	if q.lastTransitionStatus != db.RequestStatusProcessing {
		t.Fatalf("expected fallback transition to processing (not failed) to avoid a bogus ledger reversal, got %s", q.lastTransitionStatus)
	}
}

// ---------- ListUserRequests / HandleAcceptTerms extra branches ----------

func TestListUserRequests_Unauthenticated(t *testing.T) {
	q := &mockAdvanceQuerier{}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.GET("/api/advance-requests", h.ListUserRequests)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/advance-requests", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListUserRequests_DBError(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{listErr: errors.New("boom")}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.GET("/api/advance-requests", func(c *gin.Context) { setUserContext(c, uid); h.ListUserRequests(c) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/advance-requests", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAcceptTerms_DBError(t *testing.T) {
	uid := uuid.New()
	q := &mockUserTermsQuerier{err: errors.New("boom")}
	r := makeTestGin()
	r.PUT("/api/users/terms", func(c *gin.Context) { setUserContext(c, uid); HandleAcceptTerms(q)(c) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/users/terms", strings.NewReader(`{"version":"1.0"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAcceptTerms_Unauthenticated(t *testing.T) {
	q := &mockUserTermsQuerier{}
	r := makeTestGin()
	r.PUT("/api/users/terms", HandleAcceptTerms(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/users/terms", strings.NewReader(`{"version":"1.0"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

type mockUserTermsQuerier struct {
	err error
}

func (m *mockUserTermsQuerier) UpdateTermsAcceptance(ctx context.Context, arg db.UpdateTermsAcceptanceParams) (db.User, error) {
	if m.err != nil {
		return db.User{}, m.err
	}
	return db.User{ID: arg.ID, IsTermsAccepted: true}, nil
}

func TestHandleListAdminRequests_DBError(t *testing.T) {
	q := &mockAdminRequestsQuerier{err: errors.New("boom")}
	r := makeTestGin()
	r.GET("/api/admin/requests", HandleListAdminRequests(q))
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/admin/requests", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------- advance webhook branch coverage ----------

func TestAdvanceWebhook_PendingStatus(t *testing.T) {
	q := &mockWebhookQuerier{request: &db.AdvanceRequest{
		ID: uuid.New(), UserID: uuid.New(), CompanyID: testCompanyID,
		Status: db.RequestStatusProcessing, CreatedAt: time.Now().UTC().Add(-time.Minute),
	}}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})
	w := postWebhook(h, `{"reference":"ref-1","status":"PENDING","signature":"valid-jwt"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAdvanceWebhook_UnknownStatus(t *testing.T) {
	q := &mockWebhookQuerier{request: &db.AdvanceRequest{ID: uuid.New(), Status: db.RequestStatusPending}}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})
	w := postWebhook(h, `{"reference":"ref-1","status":"WEIRD","signature":"valid-jwt"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown status, got %d", w.Code)
	}
}

func TestAdvanceWebhook_UpdateError(t *testing.T) {
	q := &mockWebhookQuerier{
		request:       &db.AdvanceRequest{ID: uuid.New(), Status: db.RequestStatusPending},
		transitionErr: errors.New("db down"),
	}
	h := NewWebhookHandler(q, &mockWebhookVerifier{valid: true})
	w := postWebhook(h, `{"reference":"ref-1","status":"SUCCESSFUL","signature":"valid-jwt"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestRetryRequest_NotFound(t *testing.T) {
	uid := uuid.New()
	q := &mockAdvanceQuerier{user: eligibleUser(uid), byIDErr: errTestNotFound}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.POST("/api/advance-requests/:id/retry", func(c *gin.Context) {
		setUserContext(c, uid)
		h.RetryRequest(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests/"+uuid.NewString()+"/retry", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRetryRequest_NotTerminalFailed(t *testing.T) {
	uid := uuid.New()
	target := db.AdvanceRequest{ID: uuid.New(), UserID: uid, Status: db.RequestStatusPending}
	q := &mockAdvanceQuerier{user: eligibleUser(uid), byID: &target}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.POST("/api/advance-requests/:id/retry", func(c *gin.Context) {
		setUserContext(c, uid)
		h.RetryRequest(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests/"+target.ID.String()+"/retry", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestRetryRequest_WrongUser(t *testing.T) {
	uid := uuid.New()
	target := db.AdvanceRequest{ID: uuid.New(), UserID: uuid.New(), Status: db.RequestStatusFailed}
	q := &mockAdvanceQuerier{user: eligibleUser(uid), byID: &target}
	h := NewAdvanceHandler(q, &mockCampayTransferer{}, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.POST("/api/advance-requests/:id/retry", func(c *gin.Context) {
		setUserContext(c, uid)
		h.RetryRequest(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests/"+target.ID.String()+"/retry", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 (no cross-user leak), got %d", w.Code)
	}
}

func TestRetryRequest_TerminalFailed_CreatesNewRequest(t *testing.T) {
	uid := uuid.New()
	target := db.AdvanceRequest{ID: uuid.New(), UserID: uid, Status: db.RequestStatusFailed}
	q := &mockAdvanceQuerier{user: eligibleUser(uid), byID: &target}
	transferMock := &mockCampayTransferer{transferResp: &campay.TransferResponse{Reference: "retry-ref", Status: "SUCCESSFUL"}}
	h := NewAdvanceHandler(q, transferMock, &mockAdvanceSettingsQuerier{}, time.UTC)
	r := makeTestGin()
	r.POST("/api/advance-requests/:id/retry", func(c *gin.Context) {
		setUserContext(c, uid)
		h.RetryRequest(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/advance-requests/"+target.ID.String()+"/retry", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}
