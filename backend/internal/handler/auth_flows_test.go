package handler

import (
	"context"
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

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/authjwt"
)

// flexAuthQuerier is a fully configurable authQuerier: every method delegates to
// an optional func field, falling back to a sensible "not found / no-op" default.
// This lets each test drive one flow precisely without a rigid fixture.
type flexAuthQuerier struct {
	getUserByID          func(uuid.UUID) (db.User, error)
	getUserByEmail       func(string) (db.User, error)
	getAdminByID         func(uuid.UUID) (db.Admin, error)
	getAdminByEmail      func(string) (db.Admin, error)
	getCompanyByID       func(uuid.UUID) (db.Company, error)
	getPlatformAdmin     func(string) (db.PlatformAdmin, error)
	createUser           func(db.CreateUserParams) (db.User, error)
	getRefreshToken      func(string) (db.RefreshToken, error)
	getActiveInvitation  func(string) (db.Invitation, error)
	createEmailOTP       func(db.CreateEmailOTPParams) (db.EmailOtp, error)
	getEmailOTPByEmail   func(string) (db.EmailOtp, error)
	deleteEmailOTP       func(string) error
	acceptInvitation     func(string) (db.Invitation, error)
	getEmailOTPFailure   func(string) (db.EmailOtpFailure, error)
	upsertOTPFailure     func(db.UpsertEmailOTPFailureParams) (db.EmailOtpFailure, error)
	createRefreshTokenFn func(db.CreateRefreshTokenParams) (db.RefreshToken, error)

	// call trackers
	revokedAll    bool
	revokedHash   string
	upsertedFail  *db.UpsertEmailOTPFailureParams
	resetFails    bool
	resetFailsErr error
	lockedUser    bool
}

func (m *flexAuthQuerier) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	if m.getUserByID != nil {
		return m.getUserByID(id)
	}
	return db.User{}, errTestNotFound
}
func (m *flexAuthQuerier) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	if m.getUserByEmail != nil {
		return m.getUserByEmail(email)
	}
	return db.User{}, errTestNotFound
}
func (m *flexAuthQuerier) GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error) {
	if m.getAdminByID != nil {
		return m.getAdminByID(id)
	}
	return db.Admin{}, errTestNotFound
}
func (m *flexAuthQuerier) GetAdminByEmail(ctx context.Context, email string) (db.Admin, error) {
	if m.getAdminByEmail != nil {
		return m.getAdminByEmail(email)
	}
	return db.Admin{}, errTestNotFound
}
func (m *flexAuthQuerier) GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error) {
	if m.getCompanyByID != nil {
		return m.getCompanyByID(id)
	}
	return db.Company{ID: id, Slug: "acme", Status: db.CompanyStatusActive}, nil
}
func (m *flexAuthQuerier) GetPlatformAdminByEmail(ctx context.Context, email string) (db.PlatformAdmin, error) {
	if m.getPlatformAdmin != nil {
		return m.getPlatformAdmin(email)
	}
	return db.PlatformAdmin{}, errTestNotFound
}
func (m *flexAuthQuerier) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	if m.createUser != nil {
		return m.createUser(arg)
	}
	return db.User{ID: uuid.New(), CompanyID: arg.CompanyID, Email: arg.Email}, nil
}
func (m *flexAuthQuerier) CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error) {
	if m.createRefreshTokenFn != nil {
		return m.createRefreshTokenFn(arg)
	}
	return db.RefreshToken{}, nil
}
func (m *flexAuthQuerier) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error) {
	if m.getRefreshToken != nil {
		return m.getRefreshToken(tokenHash)
	}
	return db.RefreshToken{}, errTestNotFound
}
func (m *flexAuthQuerier) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	m.revokedHash = tokenHash
	return nil
}
func (m *flexAuthQuerier) RevokeAllRefreshTokensForSubject(ctx context.Context, arg db.RevokeAllRefreshTokensForSubjectParams) error {
	m.revokedAll = true
	return nil
}
func (m *flexAuthQuerier) GetActiveInvitationByEmail(ctx context.Context, email string) (db.Invitation, error) {
	if m.getActiveInvitation != nil {
		return m.getActiveInvitation(email)
	}
	return db.Invitation{}, errTestNotFound
}
func (m *flexAuthQuerier) CreateEmailOTP(ctx context.Context, arg db.CreateEmailOTPParams) (db.EmailOtp, error) {
	if m.createEmailOTP != nil {
		return m.createEmailOTP(arg)
	}
	return db.EmailOtp{}, nil
}
func (m *flexAuthQuerier) GetEmailOTPByEmail(ctx context.Context, email string) (db.EmailOtp, error) {
	if m.getEmailOTPByEmail != nil {
		return m.getEmailOTPByEmail(email)
	}
	return db.EmailOtp{}, errTestNotFound
}
func (m *flexAuthQuerier) DeleteEmailOTP(ctx context.Context, email string) error {
	if m.deleteEmailOTP != nil {
		return m.deleteEmailOTP(email)
	}
	return nil
}
func (m *flexAuthQuerier) AcceptInvitation(ctx context.Context, email string) (db.Invitation, error) {
	if m.acceptInvitation != nil {
		return m.acceptInvitation(email)
	}
	return db.Invitation{}, nil
}
func (m *flexAuthQuerier) IncrementFailedLoginAttempts(ctx context.Context, id uuid.UUID) (db.User, error) {
	return db.User{}, nil
}
func (m *flexAuthQuerier) ResetLoginAttempts(ctx context.Context, id uuid.UUID) (db.User, error) {
	return db.User{}, nil
}
func (m *flexAuthQuerier) LockUserUntil(ctx context.Context, arg db.LockUserUntilParams) (db.User, error) {
	return db.User{}, nil
}
func (m *flexAuthQuerier) LockUser(ctx context.Context, id uuid.UUID) (db.User, error) {
	m.lockedUser = true
	return db.User{}, nil
}
func (m *flexAuthQuerier) UpdateUserPinHash(ctx context.Context, arg db.UpdateUserPinHashParams) (db.User, error) {
	return db.User{}, nil
}
func (m *flexAuthQuerier) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	return db.Event{}, nil
}
func (m *flexAuthQuerier) GetEmailOTPFailure(ctx context.Context, email string) (db.EmailOtpFailure, error) {
	if m.getEmailOTPFailure != nil {
		return m.getEmailOTPFailure(email)
	}
	return db.EmailOtpFailure{}, errTestNotFound
}
func (m *flexAuthQuerier) UpsertEmailOTPFailure(ctx context.Context, arg db.UpsertEmailOTPFailureParams) (db.EmailOtpFailure, error) {
	m.upsertedFail = &arg
	if m.upsertOTPFailure != nil {
		return m.upsertOTPFailure(arg)
	}
	return db.EmailOtpFailure{}, nil
}
func (m *flexAuthQuerier) ResetEmailOTPFailures(ctx context.Context, email string) error {
	m.resetFails = true
	return m.resetFailsErr
}

// trackingEmailSender records whether an OTP was "sent" and can fail on demand.
type trackingEmailSender struct {
	sent    bool
	lastTo  string
	failErr error
}

func (s *trackingEmailSender) SendOTP(ctx context.Context, to, code string) error {
	if s.failErr != nil {
		return s.failErr
	}
	s.sent = true
	s.lastTo = to
	return nil
}

func newFlexAuthHandler(q authQuerier, email emailSender) *AuthHandler {
	svc := authjwt.NewHS256Service("test-secret-key-that-is-long-enough", 15*time.Minute)
	return NewAuthHandler(q, svc, stubHasher{}, email, 30*24*time.Hour)
}

func doReq(h *AuthHandler, method, path string, register func(*gin.Engine), body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	register(r)
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func codeOf(t *testing.T, body []byte) string {
	t.Helper()
	var resp map[string]interface{}
	_ = json.Unmarshal(body, &resp)
	if c, ok := resp["code"].(string); ok {
		return c
	}
	return ""
}

// ---------- CreatePin ----------

func TestCreatePin_Success(t *testing.T) {
	companyID := uuid.New()
	q := &flexAuthQuerier{
		getUserByEmail: func(string) (db.User, error) { return db.User{}, errTestNotFound }, // no existing user
		getActiveInvitation: func(string) (db.Invitation, error) {
			return db.Invitation{CompanyID: companyID, Status: db.InvitationStatusAccepted}, nil
		},
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/pin", func(r *gin.Engine) { r.POST("/pin", h.CreatePin) },
		`{"email":"new@acme.com","pin":"12345"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// CreatePin must require that the invitation has actually been accepted (i.e.
// OTP-verified via VerifyEmailOTP) — 'pending'/'sent' alone must not be enough,
// otherwise anyone who knows an invited email could create the account for it
// without ever proving control of the inbox.
func TestCreatePin_InvitationNotYetAccepted(t *testing.T) {
	q := &flexAuthQuerier{
		getUserByEmail: func(string) (db.User, error) { return db.User{}, errTestNotFound },
		getActiveInvitation: func(string) (db.Invitation, error) {
			return db.Invitation{Status: db.InvitationStatusSent}, nil
		},
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/pin", func(r *gin.Engine) { r.POST("/pin", h.CreatePin) },
		`{"email":"notverified@acme.com","pin":"12345"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreatePin_UserExists(t *testing.T) {
	q := &flexAuthQuerier{getUserByEmail: func(string) (db.User, error) { return db.User{ID: uuid.New()}, nil }}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/pin", func(r *gin.Engine) { r.POST("/pin", h.CreatePin) },
		`{"email":"exists@acme.com","pin":"12345"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreatePin_NoInvitation(t *testing.T) {
	q := &flexAuthQuerier{
		getUserByEmail:      func(string) (db.User, error) { return db.User{}, errTestNotFound },
		getActiveInvitation: func(string) (db.Invitation, error) { return db.Invitation{}, errTestNotFound },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/pin", func(r *gin.Engine) { r.POST("/pin", h.CreatePin) },
		`{"email":"noinvite@acme.com","pin":"12345"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCreatePin_Validation(t *testing.T) {
	q := &flexAuthQuerier{}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	cases := []struct{ name, body string }{
		{"bad email", `{"email":"nope","pin":"12345"}`},
		{"short pin", `{"email":"a@b.com","pin":"12"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doReq(h, "POST", "/pin", func(r *gin.Engine) { r.POST("/pin", h.CreatePin) }, tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestCreatePin_CreateUserError(t *testing.T) {
	q := &flexAuthQuerier{
		getUserByEmail:      func(string) (db.User, error) { return db.User{}, errTestNotFound },
		getActiveInvitation: func(string) (db.Invitation, error) { return db.Invitation{Status: db.InvitationStatusAccepted}, nil },
		createUser:          func(db.CreateUserParams) (db.User, error) { return db.User{}, errors.New("boom") },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/pin", func(r *gin.Engine) { r.POST("/pin", h.CreatePin) },
		`{"email":"new@acme.com","pin":"12345"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------- ForgotPin ----------

func TestForgotPin_Success(t *testing.T) {
	q := &flexAuthQuerier{
		getUserByEmail: func(string) (db.User, error) { return db.User{ID: uuid.New(), Status: db.UserStatusActive}, nil },
	}
	email := &trackingEmailSender{}
	h := newFlexAuthHandler(q, email)
	w := doReq(h, "POST", "/forgot", func(r *gin.Engine) { r.POST("/forgot", h.ForgotPin) },
		`{"email":"worker@acme.com"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !email.sent {
		t.Fatalf("expected an OTP email to be sent")
	}
}

func TestForgotPin_NoAccount(t *testing.T) {
	q := &flexAuthQuerier{getUserByEmail: func(string) (db.User, error) { return db.User{}, errTestNotFound }}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/forgot", func(r *gin.Engine) { r.POST("/forgot", h.ForgotPin) },
		`{"email":"nobody@acme.com"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestForgotPin_LockedAccount(t *testing.T) {
	q := &flexAuthQuerier{getUserByEmail: func(string) (db.User, error) {
		return db.User{ID: uuid.New(), Status: db.UserStatusLocked}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/forgot", func(r *gin.Engine) { r.POST("/forgot", h.ForgotPin) },
		`{"email":"locked@acme.com"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestForgotPin_SendError(t *testing.T) {
	q := &flexAuthQuerier{getUserByEmail: func(string) (db.User, error) {
		return db.User{ID: uuid.New(), Status: db.UserStatusActive}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{failErr: errors.New("smtp down")})
	w := doReq(h, "POST", "/forgot", func(r *gin.Engine) { r.POST("/forgot", h.ForgotPin) },
		`{"email":"worker@acme.com"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// checkEmailOTPBlocked paths, exercised through ForgotPin.

func TestForgotPin_PermanentlyBlocked(t *testing.T) {
	q := &flexAuthQuerier{getEmailOTPFailure: func(string) (db.EmailOtpFailure, error) {
		return db.EmailOtpFailure{IsPermanentlyBlocked: true}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/forgot", func(r *gin.Engine) { r.POST("/forgot", h.ForgotPin) },
		`{"email":"blocked@acme.com"}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "otp_permanently_blocked" {
		t.Fatalf("expected 403 otp_permanently_blocked, got %d %s", w.Code, w.Body.String())
	}
}

func TestForgotPin_TemporarilyBlocked(t *testing.T) {
	q := &flexAuthQuerier{getEmailOTPFailure: func(string) (db.EmailOtpFailure, error) {
		return db.EmailOtpFailure{BlockedUntil: pgtype.Timestamptz{Time: time.Now().UTC().Add(30 * time.Minute), Valid: true}}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/forgot", func(r *gin.Engine) { r.POST("/forgot", h.ForgotPin) },
		`{"email":"blocked@acme.com"}`)
	if w.Code != http.StatusTooManyRequests || codeOf(t, w.Body.Bytes()) != "otp_temporarily_blocked" {
		t.Fatalf("expected 429 otp_temporarily_blocked, got %d %s", w.Code, w.Body.String())
	}
}

// ---------- SendEmailOTP ----------

func TestSendEmailOTP_Success(t *testing.T) {
	q := &flexAuthQuerier{getActiveInvitation: func(string) (db.Invitation, error) {
		return db.Invitation{Status: db.InvitationStatusPending}, nil
	}}
	email := &trackingEmailSender{}
	h := newFlexAuthHandler(q, email)
	w := doReq(h, "POST", "/otp", func(r *gin.Engine) { r.POST("/otp", h.SendEmailOTP) },
		`{"email":"invitee@acme.com"}`)
	if w.Code != http.StatusOK || !email.sent {
		t.Fatalf("expected 200 with OTP sent, got %d sent=%v", w.Code, email.sent)
	}
}

func TestSendEmailOTP_NoInvitation(t *testing.T) {
	q := &flexAuthQuerier{}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/otp", func(r *gin.Engine) { r.POST("/otp", h.SendEmailOTP) },
		`{"email":"invitee@acme.com"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSendEmailOTP_InvitationNotActive(t *testing.T) {
	q := &flexAuthQuerier{getActiveInvitation: func(string) (db.Invitation, error) {
		return db.Invitation{Status: db.InvitationStatusAccepted}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/otp", func(r *gin.Engine) { r.POST("/otp", h.SendEmailOTP) },
		`{"email":"invitee@acme.com"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// ---------- VerifyEmailOTP ----------

func TestVerifyEmailOTP_SignupSuccess(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"invitee@acme.com","code":"123456","purpose":"signup"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !q.resetFails {
		t.Fatalf("a correct OTP must reset the failure counter")
	}
}

func TestVerifyEmailOTP_PinResetReturnsTokens(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
		getUserByEmail:     func(string) (db.User, error) { return db.User{ID: uuid.New(), Status: db.UserStatusActive}, nil },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"worker@acme.com","code":"123456","purpose":"pin_reset"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["data"]["access_token"] == nil {
		t.Fatalf("pin_reset verification should return tokens")
	}
}

func TestVerifyEmailOTP_WrongCodeRecordsFailure(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"invitee@acme.com","code":"000000","purpose":"signup"}`)
	if w.Code != http.StatusBadRequest || codeOf(t, w.Body.Bytes()) != "invalid_otp" {
		t.Fatalf("expected 400 invalid_otp, got %d %s", w.Code, w.Body.String())
	}
	if q.upsertedFail == nil || q.upsertedFail.ConsecutiveFailures != 1 {
		t.Fatalf("expected a recorded failure with count 1, got %+v", q.upsertedFail)
	}
}

func TestVerifyEmailOTP_BadPurpose(t *testing.T) {
	q := &flexAuthQuerier{}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"a@b.com","code":"123456","purpose":"nonsense"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// Submitting a code when no OTP is on record must behave the same as a wrong
// code: record the failed attempt AND return 400 invalid_otp (never a silent
// 200). This guards the fix for that previously-silent branch.
func TestVerifyEmailOTP_NoStoredOTP(t *testing.T) {
	q := &flexAuthQuerier{} // GetEmailOTPByEmail returns not-found by default
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"a@b.com","code":"123456"}`)
	// The failed attempt must still be recorded (rate-limiting works)...
	if q.upsertedFail == nil {
		t.Fatalf("expected a failure to be recorded when no OTP is stored")
	}
	// ...and the client must get a consistent invalid_otp error.
	if w.Code != http.StatusBadRequest || codeOf(t, w.Body.Bytes()) != "invalid_otp" {
		t.Fatalf("expected 400 invalid_otp, got status=%d body=%q", w.Code, w.Body.String())
	}
}

// recordOTPFailure: 6th consecutive failure permanently locks the user.
func TestVerifyEmailOTP_SixthFailurePermanentlyLocks(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
		getEmailOTPFailure: func(string) (db.EmailOtpFailure, error) {
			return db.EmailOtpFailure{ConsecutiveFailures: 5}, nil
		},
		getUserByEmail: func(string) (db.User, error) { return db.User{ID: uuid.New()}, nil },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"worker@acme.com","code":"000000","purpose":"signup"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if q.upsertedFail == nil || !q.upsertedFail.IsPermanentlyBlocked {
		t.Fatalf("6th consecutive failure must set permanent block, got %+v", q.upsertedFail)
	}
	if !q.lockedUser {
		t.Fatalf("6th consecutive failure must lock the user")
	}
}

// recordOTPFailure: 3rd same-day failure sets a temporary block-until.
func TestVerifyEmailOTP_ThirdSameDayFailureTempBlocks(t *testing.T) {
	today := time.Now().UTC()
	var lastDate pgtype.Date
	_ = lastDate.Scan(today.Format("2006-01-02"))
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
		getEmailOTPFailure: func(string) (db.EmailOtpFailure, error) {
			return db.EmailOtpFailure{ConsecutiveFailures: 2, LastFailureDate: lastDate}, nil
		},
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"worker@acme.com","code":"000000","purpose":"signup"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if q.upsertedFail == nil || !q.upsertedFail.BlockedUntil.Valid {
		t.Fatalf("3rd same-day failure must set a temporary block, got %+v", q.upsertedFail)
	}
}

// ---------- AdminLogin ----------

func TestAdminLogin_Success(t *testing.T) {
	q := &flexAuthQuerier{getAdminByEmail: func(string) (db.Admin, error) {
		return db.Admin{ID: uuid.New(), CompanyID: uuid.New(), Email: "admin@acme.com", PasswordHash: "hashed:secret"}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/admin/login", func(r *gin.Engine) { r.POST("/admin/login", h.AdminLogin) },
		`{"email":"admin@acme.com","password":"secret"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Regression: the response must never leak the bcrypt hash.
	if strings.Contains(w.Body.String(), "hashed:secret") || strings.Contains(w.Body.String(), "password_hash") {
		t.Fatalf("response leaks password hash: %s", w.Body.String())
	}

	var resp struct {
		Data struct {
			CompanySlug string `json:"company_slug"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Data.CompanySlug != "acme" {
		t.Fatalf("expected company_slug acme, got %q", resp.Data.CompanySlug)
	}
}

func TestAdminLogin_SuspendedCompanyBlocks(t *testing.T) {
	q := &flexAuthQuerier{
		getAdminByEmail: func(string) (db.Admin, error) {
			return db.Admin{ID: uuid.New(), CompanyID: uuid.New(), Email: "admin@acme.com", PasswordHash: "hashed:secret"}, nil
		},
		getCompanyByID: func(uuid.UUID) (db.Company, error) {
			return db.Company{Status: db.CompanyStatusSuspended}, nil
		},
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/admin/login", func(r *gin.Engine) { r.POST("/admin/login", h.AdminLogin) },
		`{"email":"admin@acme.com","password":"secret"}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "company_suspended" {
		t.Fatalf("expected 403 company_suspended, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreatePin_SuspendedCompanyBlocks(t *testing.T) {
	q := &flexAuthQuerier{
		getUserByEmail: func(string) (db.User, error) { return db.User{}, errTestNotFound },
		getActiveInvitation: func(string) (db.Invitation, error) {
			return db.Invitation{Status: db.InvitationStatusAccepted}, nil
		},
		getCompanyByID: func(uuid.UUID) (db.Company, error) {
			return db.Company{Status: db.CompanyStatusSuspended}, nil
		},
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/pin", func(r *gin.Engine) { r.POST("/pin", h.CreatePin) },
		`{"email":"new@acme.com","pin":"12345"}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "company_suspended" {
		t.Fatalf("expected 403 company_suspended, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminLogin_WrongPassword(t *testing.T) {
	q := &flexAuthQuerier{getAdminByEmail: func(string) (db.Admin, error) {
		return db.Admin{ID: uuid.New(), PasswordHash: "hashed:secret"}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/admin/login", func(r *gin.Engine) { r.POST("/admin/login", h.AdminLogin) },
		`{"email":"admin@acme.com","password":"wrong"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAdminLogin_NotFound(t *testing.T) {
	q := &flexAuthQuerier{}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/admin/login", func(r *gin.Engine) { r.POST("/admin/login", h.AdminLogin) },
		`{"email":"admin@acme.com","password":"secret"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// ---------- PlatformLogin ----------

func TestPlatformLogin_Success(t *testing.T) {
	q := &flexAuthQuerier{getPlatformAdmin: func(string) (db.PlatformAdmin, error) {
		return db.PlatformAdmin{ID: uuid.New(), Email: "root@platform.io", PasswordHash: "hashed:secret"}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/platform/login", func(r *gin.Engine) { r.POST("/platform/login", h.PlatformLogin) },
		`{"email":"root@platform.io","password":"secret"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlatformLogin_WrongPassword(t *testing.T) {
	q := &flexAuthQuerier{getPlatformAdmin: func(string) (db.PlatformAdmin, error) {
		return db.PlatformAdmin{ID: uuid.New(), PasswordHash: "hashed:secret"}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/platform/login", func(r *gin.Engine) { r.POST("/platform/login", h.PlatformLogin) },
		`{"email":"root@platform.io","password":"nope"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// ---------- RefreshToken (+ resolveCompanyForSubject) ----------

func TestRefreshToken_UserReResolvesCompany(t *testing.T) {
	subjectID := uuid.New()
	companyID := uuid.New()
	q := &flexAuthQuerier{
		getRefreshToken: func(string) (db.RefreshToken, error) {
			return db.RefreshToken{SubjectID: subjectID, SubjectType: "user"}, nil
		},
		getUserByID: func(uuid.UUID) (db.User, error) { return db.User{ID: subjectID, CompanyID: companyID}, nil },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/refresh", func(r *gin.Engine) { r.POST("/refresh", h.RefreshToken) },
		`{"refresh_token":"sometoken"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if q.revokedHash == "" {
		t.Fatalf("refresh must revoke (rotate) the presented token")
	}
}

func TestRefreshToken_AdminSubject(t *testing.T) {
	subjectID := uuid.New()
	q := &flexAuthQuerier{
		getRefreshToken: func(string) (db.RefreshToken, error) {
			return db.RefreshToken{SubjectID: subjectID, SubjectType: "admin"}, nil
		},
		getAdminByID: func(uuid.UUID) (db.Admin, error) { return db.Admin{ID: subjectID, CompanyID: uuid.New()}, nil },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/refresh", func(r *gin.Engine) { r.POST("/refresh", h.RefreshToken) },
		`{"refresh_token":"sometoken"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRefreshToken_PlatformAdminSubject(t *testing.T) {
	subjectID := uuid.New()
	q := &flexAuthQuerier{
		getRefreshToken: func(string) (db.RefreshToken, error) {
			return db.RefreshToken{SubjectID: subjectID, SubjectType: "platform_admin"}, nil
		},
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/refresh", func(r *gin.Engine) { r.POST("/refresh", h.RefreshToken) },
		`{"refresh_token":"sometoken"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRefreshToken_UnknownSubjectType(t *testing.T) {
	q := &flexAuthQuerier{
		getRefreshToken: func(string) (db.RefreshToken, error) {
			return db.RefreshToken{SubjectID: uuid.New(), SubjectType: "martian"}, nil
		},
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/refresh", func(r *gin.Engine) { r.POST("/refresh", h.RefreshToken) },
		`{"refresh_token":"tok"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unknown subject type, got %d", w.Code)
	}
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	q := &flexAuthQuerier{} // GetRefreshTokenByHash returns not-found
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/refresh", func(r *gin.Engine) { r.POST("/refresh", h.RefreshToken) },
		`{"refresh_token":"bad"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRefreshToken_SubjectGone(t *testing.T) {
	q := &flexAuthQuerier{
		getRefreshToken: func(string) (db.RefreshToken, error) {
			return db.RefreshToken{SubjectID: uuid.New(), SubjectType: "user"}, nil
		},
		getUserByID: func(uuid.UUID) (db.User, error) { return db.User{}, errTestNotFound },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/refresh", func(r *gin.Engine) { r.POST("/refresh", h.RefreshToken) },
		`{"refresh_token":"sometoken"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when subject no longer exists, got %d", w.Code)
	}
}

// ---------- Logout ----------

func TestLogout_RevokesTokenAndAllForSubject(t *testing.T) {
	q := &flexAuthQuerier{}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	subjectID := uuid.New()
	r.POST("/logout", func(c *gin.Context) {
		c.Set("subject_id", subjectID.String())
		c.Set("subject_type", "user")
		h.Logout(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "/logout", strings.NewReader(`{"refresh_token":"tok"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if q.revokedHash == "" || !q.revokedAll {
		t.Fatalf("logout must revoke the token and all subject tokens, got hash=%q all=%v", q.revokedHash, q.revokedAll)
	}
}

func TestLogout_NoBodyStillOK(t *testing.T) {
	q := &flexAuthQuerier{}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/logout", func(r *gin.Engine) { r.POST("/logout", h.Logout) }, ``)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// ---------- CheckInvitation ----------

func TestCheckInvitation_Found(t *testing.T) {
	q := &flexAuthQuerier{getActiveInvitation: func(string) (db.Invitation, error) {
		return db.Invitation{Status: db.InvitationStatusPending}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "GET", "/check?email=invitee@acme.com", func(r *gin.Engine) { r.GET("/check", h.CheckInvitation) }, ``)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCheckInvitation_MissingEmail(t *testing.T) {
	q := &flexAuthQuerier{}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "GET", "/check", func(r *gin.Engine) { r.GET("/check", h.CheckInvitation) }, ``)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCheckInvitation_NotFound(t *testing.T) {
	q := &flexAuthQuerier{}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "GET", "/check?email=none@acme.com", func(r *gin.Engine) { r.GET("/check", h.CheckInvitation) }, ``)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ---------- Login lockout ladder ----------

func TestLogin_Validation(t *testing.T) {
	q := &mockAuthQuerier{}
	h := newTestAuthHandler(q)
	cases := []struct{ name, body string }{
		{"bad email", `{"email":"nope","pin":"12345"}`},
		{"short pin", `{"email":"a@b.com","pin":"12"}`},
		{"missing pin", `{"email":"a@b.com"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := postLogin(h, tc.body); w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	q := &mockAuthQuerier{userErr: errTestNotFound}
	h := newTestAuthHandler(q)
	if w := postLogin(h, `{"email":"ghost@acme.com","pin":"12345"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogin_AccountLocked(t *testing.T) {
	q := &mockAuthQuerier{user: &db.User{
		ID: uuid.New(), Email: "w@acme.com", Status: db.UserStatusLocked,
		PinHash: pgtype.Text{String: "hashed:12345", Valid: true},
	}}
	h := newTestAuthHandler(q)
	w := postLogin(h, `{"email":"w@acme.com","pin":"12345"}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "account_locked" {
		t.Fatalf("expected 403 account_locked, got %d %s", w.Code, w.Body.String())
	}
}

func TestLogin_AccountSuspended(t *testing.T) {
	q := &mockAuthQuerier{user: &db.User{
		ID: uuid.New(), Email: "w@acme.com", Status: db.UserStatusSuspended,
		PinHash: pgtype.Text{String: "hashed:12345", Valid: true},
	}}
	h := newTestAuthHandler(q)
	w := postLogin(h, `{"email":"w@acme.com","pin":"12345"}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "account_suspended" {
		t.Fatalf("expected 403 account_suspended, got %d %s", w.Code, w.Body.String())
	}
}

func TestLogin_TemporarilyLockedUntil(t *testing.T) {
	q := &mockAuthQuerier{user: &db.User{
		ID: uuid.New(), Email: "w@acme.com", Status: db.UserStatusActive,
		PinHash:     pgtype.Text{String: "hashed:12345", Valid: true},
		LockedUntil: pgtype.Timestamptz{Time: time.Now().UTC().Add(30 * time.Minute), Valid: true},
	}}
	h := newTestAuthHandler(q)
	w := postLogin(h, `{"email":"w@acme.com","pin":"12345"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestLogin_NoPinSet(t *testing.T) {
	q := &mockAuthQuerier{user: &db.User{
		ID: uuid.New(), Email: "w@acme.com", Status: db.UserStatusActive,
		PinHash: pgtype.Text{Valid: false},
	}}
	h := newTestAuthHandler(q)
	w := postLogin(h, `{"email":"w@acme.com","pin":"12345"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLogin_ThirdWrongAttemptTempLocks(t *testing.T) {
	q := &mockAuthQuerier{user: &db.User{
		ID: uuid.New(), Email: "w@acme.com", Status: db.UserStatusActive,
		PinHash:             pgtype.Text{String: "hashed:12345", Valid: true},
		FailedLoginAttempts: 2, // next wrong attempt -> 3
	}}
	h := newTestAuthHandler(q)
	w := postLogin(h, `{"email":"w@acme.com","pin":"00000"}`)
	if w.Code != http.StatusTooManyRequests || codeOf(t, w.Body.Bytes()) != "too_many_attempts" {
		t.Fatalf("expected 429 too_many_attempts at 3rd failure, got %d %s", w.Code, w.Body.String())
	}
}

func TestLogin_SixthWrongAttemptLocks(t *testing.T) {
	q := &mockAuthQuerier{user: &db.User{
		ID: uuid.New(), Email: "w@acme.com", Status: db.UserStatusActive,
		PinHash:             pgtype.Text{String: "hashed:12345", Valid: true},
		FailedLoginAttempts: 5, // next wrong attempt -> 6
	}}
	h := newTestAuthHandler(q)
	w := postLogin(h, `{"email":"w@acme.com","pin":"00000"}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "account_locked" {
		t.Fatalf("expected 403 account_locked at 6th failure, got %d %s", w.Code, w.Body.String())
	}
}

func TestLogin_CompanyResolveError(t *testing.T) {
	companyID := uuid.New()
	q := &mockAuthQuerier{
		user: &db.User{
			ID: uuid.New(), CompanyID: companyID, Email: "w@acme.com", Status: db.UserStatusActive,
			PinHash: pgtype.Text{String: "hashed:12345", Valid: true},
		},
		companyErr: errors.New("boom"),
	}
	h := newTestAuthHandler(q)
	w := postLogin(h, `{"email":"w@acme.com","pin":"12345"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------- token_failed lever: generateTokenPair fails when the refresh token
// cannot be stored, and every auth flow must surface that as a 500. ----------

func withTokenStoreError(q *flexAuthQuerier) *flexAuthQuerier {
	q.createRefreshTokenFn = func(db.CreateRefreshTokenParams) (db.RefreshToken, error) {
		return db.RefreshToken{}, errors.New("db down")
	}
	return q
}

func TestAdminLogin_TokenFailed(t *testing.T) {
	q := withTokenStoreError(&flexAuthQuerier{getAdminByEmail: func(string) (db.Admin, error) {
		return db.Admin{ID: uuid.New(), CompanyID: uuid.New(), PasswordHash: "hashed:secret"}, nil
	}})
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/admin/login", func(r *gin.Engine) { r.POST("/admin/login", h.AdminLogin) },
		`{"email":"a@acme.com","password":"secret"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 token_failed, got %d", w.Code)
	}
}

func TestPlatformLogin_TokenFailed(t *testing.T) {
	q := withTokenStoreError(&flexAuthQuerier{getPlatformAdmin: func(string) (db.PlatformAdmin, error) {
		return db.PlatformAdmin{ID: uuid.New(), PasswordHash: "hashed:secret"}, nil
	}})
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/platform/login", func(r *gin.Engine) { r.POST("/platform/login", h.PlatformLogin) },
		`{"email":"root@p.io","password":"secret"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 token_failed, got %d", w.Code)
	}
}

func TestCreatePin_TokenFailed(t *testing.T) {
	q := withTokenStoreError(&flexAuthQuerier{
		getUserByEmail:      func(string) (db.User, error) { return db.User{}, errTestNotFound },
		getActiveInvitation: func(string) (db.Invitation, error) { return db.Invitation{Status: db.InvitationStatusAccepted}, nil },
	})
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/pin", func(r *gin.Engine) { r.POST("/pin", h.CreatePin) },
		`{"email":"new@acme.com","pin":"12345"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 token_failed, got %d", w.Code)
	}
}

func TestRefreshToken_TokenFailed(t *testing.T) {
	subjectID := uuid.New()
	q := withTokenStoreError(&flexAuthQuerier{
		getRefreshToken: func(string) (db.RefreshToken, error) {
			return db.RefreshToken{SubjectID: subjectID, SubjectType: "platform_admin"}, nil
		},
	})
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/refresh", func(r *gin.Engine) { r.POST("/refresh", h.RefreshToken) },
		`{"refresh_token":"tok"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 token_failed, got %d", w.Code)
	}
}

// ---------- OTP store failures ----------

func TestForgotPin_StoreOTPError(t *testing.T) {
	q := &flexAuthQuerier{
		getUserByEmail: func(string) (db.User, error) { return db.User{ID: uuid.New(), Status: db.UserStatusActive}, nil },
		createEmailOTP: func(db.CreateEmailOTPParams) (db.EmailOtp, error) { return db.EmailOtp{}, errors.New("db down") },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/forgot", func(r *gin.Engine) { r.POST("/forgot", h.ForgotPin) },
		`{"email":"w@acme.com"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 store_otp_failed, got %d", w.Code)
	}
}

func TestSendEmailOTP_StoreOTPError(t *testing.T) {
	q := &flexAuthQuerier{
		getActiveInvitation: func(string) (db.Invitation, error) { return db.Invitation{Status: db.InvitationStatusPending}, nil },
		createEmailOTP:      func(db.CreateEmailOTPParams) (db.EmailOtp, error) { return db.EmailOtp{}, errors.New("db down") },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/otp", func(r *gin.Engine) { r.POST("/otp", h.SendEmailOTP) },
		`{"email":"invitee@acme.com"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 store_otp_failed, got %d", w.Code)
	}
}

func TestSendEmailOTP_SendError(t *testing.T) {
	q := &flexAuthQuerier{getActiveInvitation: func(string) (db.Invitation, error) {
		return db.Invitation{Status: db.InvitationStatusPending}, nil
	}}
	h := newFlexAuthHandler(q, &trackingEmailSender{failErr: errors.New("smtp down")})
	w := doReq(h, "POST", "/otp", func(r *gin.Engine) { r.POST("/otp", h.SendEmailOTP) },
		`{"email":"invitee@acme.com"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 send_otp_failed, got %d", w.Code)
	}
}

func TestVerifyEmailOTP_CleanupError(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
		deleteEmailOTP:     func(string) error { return errors.New("db down") },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"a@b.com","code":"123456","purpose":"signup"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 cleanup_failed, got %d", w.Code)
	}
}

func TestVerifyEmailOTP_PinResetUserNotFound(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
		getUserByEmail:     func(string) (db.User, error) { return db.User{}, errTestNotFound },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"w@acme.com","code":"123456","purpose":"pin_reset"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestVerifyEmailOTP_PinResetLockedAccount(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
		getUserByEmail:     func(string) (db.User, error) { return db.User{ID: uuid.New(), Status: db.UserStatusLocked}, nil },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"w@acme.com","code":"123456","purpose":"pin_reset"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 account_locked, got %d", w.Code)
	}
}

func TestVerifyEmailOTP_PinResetSuspendedCompanyBlocks(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
		getUserByEmail:     func(string) (db.User, error) { return db.User{ID: uuid.New(), Status: db.UserStatusActive}, nil },
		getCompanyByID: func(uuid.UUID) (db.Company, error) {
			return db.Company{Status: db.CompanyStatusSuspended}, nil
		},
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"w@acme.com","code":"123456","purpose":"pin_reset"}`)
	if w.Code != http.StatusForbidden || codeOf(t, w.Body.Bytes()) != "company_suspended" {
		t.Fatalf("expected 403 company_suspended, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerifyEmailOTP_SignupAcceptInvitationError(t *testing.T) {
	q := &flexAuthQuerier{
		getEmailOTPByEmail: func(string) (db.EmailOtp, error) { return db.EmailOtp{Code: "123456"}, nil },
		acceptInvitation:   func(string) (db.Invitation, error) { return db.Invitation{}, errors.New("db down") },
	}
	h := newFlexAuthHandler(q, &trackingEmailSender{})
	w := doReq(h, "POST", "/verify", func(r *gin.Engine) { r.POST("/verify", h.VerifyEmailOTP) },
		`{"email":"invitee@acme.com","code":"123456","purpose":"signup"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 accept_invitation_failed, got %d", w.Code)
	}
}

func TestCreatePin_HashError(t *testing.T) {
	q := &flexAuthQuerier{
		getUserByEmail:      func(string) (db.User, error) { return db.User{}, errTestNotFound },
		getActiveInvitation: func(string) (db.Invitation, error) { return db.Invitation{Status: db.InvitationStatusAccepted}, nil },
	}
	svc := authjwt.NewHS256Service("test-secret-key-that-is-long-enough", 15*time.Minute)
	h := NewAuthHandler(q, svc, failingHasher{}, &trackingEmailSender{}, 30*24*time.Hour)
	w := doReq(h, "POST", "/pin", func(r *gin.Engine) { r.POST("/pin", h.CreatePin) },
		`{"email":"new@acme.com","pin":"12345"}`)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 hash_failed, got %d", w.Code)
	}
}

// failingHasher always fails to hash (Verify still works for setup paths).
type failingHasher struct{}

func (failingHasher) Hash(string) (string, error)    { return "", errors.New("hash boom") }
func (failingHasher) Verify(hash, plain string) bool { return hash == "hashed:"+plain }
