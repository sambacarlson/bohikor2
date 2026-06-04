package handler

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/authjwt"
	"github.com/Iknite-Space/bohikor2/internal/authpassword"
)

type emailSender interface {
	SendOTP(ctx context.Context, to, code string) error
}

type AuthHandler struct {
	queries       authQuerier
	tokenService  authjwt.TokenService
	hasher        authpassword.Hasher
	emailClient   emailSender
	refreshExpiry time.Duration
}

type authQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetAdminByEmail(ctx context.Context, email string) (db.Admin, error)
	CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllRefreshTokensForSubject(ctx context.Context, arg db.RevokeAllRefreshTokensForSubjectParams) error
	GetActiveInvitationByEmail(ctx context.Context, email string) (db.Invitation, error)
	CreateEmailOTP(ctx context.Context, arg db.CreateEmailOTPParams) (db.EmailOtp, error)
	GetEmailOTPByEmail(ctx context.Context, email string) (db.EmailOtp, error)
	DeleteEmailOTP(ctx context.Context, email string) error
	AcceptInvitation(ctx context.Context, email string) (db.Invitation, error)
	IncrementFailedLoginAttempts(ctx context.Context, id uuid.UUID) (db.User, error)
	ResetLoginAttempts(ctx context.Context, id uuid.UUID) (db.User, error)
	LockUserUntil(ctx context.Context, arg db.LockUserUntilParams) (db.User, error)
	LockUser(ctx context.Context, id uuid.UUID) (db.User, error)
	UpdateUserPinHash(ctx context.Context, arg db.UpdateUserPinHashParams) (db.User, error)
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
	GetEmailOTPFailure(ctx context.Context, email string) (db.EmailOtpFailure, error)
	UpsertEmailOTPFailure(ctx context.Context, arg db.UpsertEmailOTPFailureParams) (db.EmailOtpFailure, error)
	ResetEmailOTPFailures(ctx context.Context, email string) error
}

func NewAuthHandler(
	queries authQuerier,
	tokenService authjwt.TokenService,
	hasher authpassword.Hasher,
	emailClient emailSender,
	refreshExpiry time.Duration,
) *AuthHandler {
	return &AuthHandler{
		queries:       queries,
		tokenService:  tokenService,
		hasher:        hasher,
		emailClient:   emailClient,
		refreshExpiry: refreshExpiry,
	}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (h *AuthHandler) generateTokenPair(subjectID string, subjectType string) (*tokenResponse, error) {
	accessToken, err := h.tokenService.GenerateAccessToken(subjectID, subjectType)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	plainRefresh, hashedRefresh, err := h.tokenService.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	subjectUUID, err := uuid.Parse(subjectID)
	if err != nil {
		return nil, fmt.Errorf("parse subject id: %w", err)
	}

	_, err = h.queries.CreateRefreshToken(context.Background(), db.CreateRefreshTokenParams{
		TokenHash:   hashedRefresh,
		SubjectID:   subjectUUID,
		SubjectType: subjectType,
		ExpiresAt:   time.Now().UTC().Add(h.refreshExpiry),
	})
	if err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: plainRefresh,
		ExpiresIn:    int64(15 * 60),
	}, nil
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		PIN   string `json:"pin" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "email and pin are required")
		return
	}

	if len(req.PIN) != 5 {
		JSONError(c, http.StatusBadRequest, "invalid_pin", "PIN must be 5 digits")
		return
	}

	user, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or PIN")
		return
	}

	if user.Status == db.UserStatusLocked {
		JSONError(c, http.StatusForbidden, "account_locked", "Your account has been locked. Please contact your manager to unlock it.")
		return
	}

	if user.Status == db.UserStatusSuspended {
		JSONError(c, http.StatusForbidden, "account_suspended", "Your account has been suspended.")
		return
	}

	if user.LockedUntil.Valid && user.LockedUntil.Time.After(time.Now().UTC()) {
		remaining := time.Until(user.LockedUntil.Time).Round(time.Minute)
		JSONError(c, http.StatusTooManyRequests, "too_many_attempts",
			fmt.Sprintf("Too many failed attempts. Try again in %s.", remaining))
		return
	}

	if !user.PinHash.Valid || user.PinHash.String == "" {
		JSONError(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or PIN")
		return
	}

	if !h.hasher.Verify(user.PinHash.String, req.PIN) {
		user, _ = h.queries.IncrementFailedLoginAttempts(c.Request.Context(), user.ID)

		if user.FailedLoginAttempts >= 6 {
			if _, err := h.queries.LockUser(c.Request.Context(), user.ID); err != nil {
				slog.Error("lock user account", "error", err, "user_id", user.ID)
			}
			JSONError(c, http.StatusForbidden, "account_locked", "Your account has been locked due to too many failed attempts. Please contact your manager.")
			return
		}

		if user.FailedLoginAttempts >= 3 {
			lockUntil := time.Now().UTC().Add(1 * time.Hour)
			if _, err := h.queries.LockUserUntil(c.Request.Context(), db.LockUserUntilParams{
				ID:          user.ID,
				LockedUntil: sql.NullTime{Time: lockUntil, Valid: true},
			}); err != nil {
				slog.Error("lock user until", "error", err, "user_id", user.ID)
			}
			JSONError(c, http.StatusTooManyRequests, "too_many_attempts",
				"Too many failed attempts. Try again in 1 hour.")
			return
		}

		JSONError(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or PIN")
		return
	}

	if _, err := h.queries.ResetLoginAttempts(c.Request.Context(), user.ID); err != nil {
		slog.Error("reset login attempts", "error", err, "user_id", user.ID)
	}

	tokens, err := h.generateTokenPair(user.ID.String(), "user")
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "token_failed", "Failed to generate tokens")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user":          sanitizeUser(user),
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_in":    tokens.ExpiresIn,
		},
	})
}

func (h *AuthHandler) CreatePin(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		PIN   string `json:"pin" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "email and pin are required")
		return
	}

	if len(req.PIN) != 5 {
		JSONError(c, http.StatusBadRequest, "invalid_pin", "PIN must be 5 digits")
		return
	}

	_, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err == nil {
		JSONError(c, http.StatusConflict, "user_exists", "User already exists. Please log in instead.")
		return
	}

	pinHash, err := h.hasher.Hash(req.PIN)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "hash_failed", "Failed to hash PIN")
		return
	}

	user, err := h.queries.CreateUser(c.Request.Context(), db.CreateUserParams{
		Email:         req.Email,
		EmailVerified: true,
		FullName:      pgtype.Text{Valid: false},
		PhoneNumber:   pgtype.Text{Valid: false},
		PhoneVerified: false,
		Status:        db.UserStatusActive,
		PinHash:       pgtype.Text{String: pinHash, Valid: true},
	})
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "create_user_failed", "Failed to create user")
		return
	}

	if _, err := h.queries.AcceptInvitation(c.Request.Context(), req.Email); err != nil {
		fmt.Printf("WARN: failed to accept invitation for %s: %v\n", req.Email, err)
	}

	metadata, _ := json.Marshal(map[string]string{"source": "mobile"})
	if _, err := h.queries.CreateEvent(c.Request.Context(), db.CreateEventParams{
		UserID:    pgtype.UUID{Bytes: user.ID, Valid: true},
		EventType: "signup_completed",
		Metadata:  metadata,
	}); err != nil {
		slog.Error("create signup event", "error", err)
	}

	tokens, err := h.generateTokenPair(user.ID.String(), "user")
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "token_failed", "Failed to generate tokens")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": gin.H{
			"user":          sanitizeUser(user),
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_in":    tokens.ExpiresIn,
		},
	})
}

func (h *AuthHandler) ForgotPin(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "email is required")
		return
	}

	if err := h.checkEmailOTPBlocked(c, req.Email); err != nil {
		return
	}

	user, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		JSONError(c, http.StatusNotFound, "not_found", "No account found with this email")
		return
	}

	if user.Status == db.UserStatusLocked {
		JSONError(c, http.StatusForbidden, "account_locked", "Your account is locked. Please contact your manager to unlock it.")
		return
	}

	code, err := generateOTP()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "otp_generation_failed", "Failed to generate OTP")
		return
	}

	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	_, err = h.queries.CreateEmailOTP(c.Request.Context(), db.CreateEmailOTPParams{
		Email:     req.Email,
		Code:      code,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "store_otp_failed", "Failed to store OTP")
		return
	}

	if err := h.emailClient.SendOTP(c.Request.Context(), req.Email, code); err != nil {
		JSONError(c, http.StatusInternalServerError, "send_otp_failed", "Failed to send OTP email")
		return
	}

	JSONOK(c, http.StatusOK)
}

func (h *AuthHandler) SendEmailOTP(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "email is required")
		return
	}

	if err := h.checkEmailOTPBlocked(c, req.Email); err != nil {
		return
	}

	invitation, err := h.queries.GetActiveInvitationByEmail(c.Request.Context(), req.Email)
	if err != nil {
		JSONError(c, http.StatusNotFound, "no_invitation", "No invitation found for this email. Contact your manager.")
		return
	}
	if invitation.Status != db.InvitationStatusPending && invitation.Status != db.InvitationStatusSent {
		JSONError(c, http.StatusForbidden, "invitation_not_active", "Invitation is no longer active")
		return
	}

	code, err := generateOTP()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "otp_generation_failed", "Failed to generate OTP")
		return
	}

	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	_, err = h.queries.CreateEmailOTP(c.Request.Context(), db.CreateEmailOTPParams{
		Email:     req.Email,
		Code:      code,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "store_otp_failed", "Failed to store OTP")
		return
	}

	if err := h.emailClient.SendOTP(c.Request.Context(), req.Email, code); err != nil {
		JSONError(c, http.StatusInternalServerError, "send_otp_failed", "Failed to send OTP email")
		return
	}

	JSONOK(c, http.StatusOK)
}

func (h *AuthHandler) checkEmailOTPBlocked(c *gin.Context, email string) error {
	failure, err := h.queries.GetEmailOTPFailure(c.Request.Context(), email)
	if err != nil {
		return nil
	}

	now := time.Now().UTC()

	if failure.IsPermanentlyBlocked {
		JSONError(c, http.StatusForbidden, "otp_permanently_blocked", "Your account has been blocked due to too many failed OTP attempts. Please contact your manager.")
		return fmt.Errorf("permanently blocked")
	}

	if failure.BlockedUntil.Valid && failure.BlockedUntil.Time.After(now) {
		remaining := time.Until(failure.BlockedUntil.Time).Round(time.Minute)
		JSONError(c, http.StatusTooManyRequests, "otp_temporarily_blocked",
			fmt.Sprintf("Too many failed OTP attempts. Try again in %s.", remaining))
		return fmt.Errorf("temporarily blocked")
	}

	return nil
}

func (h *AuthHandler) VerifyEmailOTP(c *gin.Context) {
	var req struct {
		Email   string `json:"email" binding:"required,email"`
		Code    string `json:"code" binding:"required"`
		Purpose string `json:"purpose"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "email and code are required")
		return
	}

	if req.Purpose != "" && req.Purpose != "signup" && req.Purpose != "pin_reset" {
		JSONError(c, http.StatusBadRequest, "invalid_purpose", "purpose must be 'signup' or 'pin_reset'")
		return
	}

	storedOTP, err := h.queries.GetEmailOTPByEmail(c.Request.Context(), req.Email)
	if err != nil {
		h.recordOTPFailure(c, req.Email)
		return
	}

	if storedOTP.Code != req.Code {
		h.recordOTPFailure(c, req.Email)
		JSONError(c, http.StatusBadRequest, "invalid_otp", "Invalid OTP code")
		return
	}

	h.resetOTPFailures(c, req.Email)

	if err := h.queries.DeleteEmailOTP(c.Request.Context(), req.Email); err != nil {
		JSONError(c, http.StatusInternalServerError, "cleanup_failed", "Failed to cleanup OTP")
		return
	}

	if req.Purpose == "pin_reset" {
		user, err := h.queries.GetUserByEmail(c.Request.Context(), req.Email)
		if err != nil {
			JSONError(c, http.StatusNotFound, "not_found", "No account found with this email")
			return
		}

		if user.Status == db.UserStatusLocked {
			JSONError(c, http.StatusForbidden, "account_locked", "Your account is locked. Contact your manager.")
			return
		}

		tokens, err := h.generateTokenPair(user.ID.String(), "user")
		if err != nil {
			JSONError(c, http.StatusInternalServerError, "token_failed", "Failed to generate tokens")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"user":          sanitizeUser(user),
				"access_token":  tokens.AccessToken,
				"refresh_token": tokens.RefreshToken,
				"expires_in":    tokens.ExpiresIn,
			},
		})
		return
	}

	if _, err := h.queries.AcceptInvitation(c.Request.Context(), req.Email); err != nil {
		JSONError(c, http.StatusInternalServerError, "accept_invitation_failed", "Failed to accept invitation")
		return
	}

	JSONOK(c, http.StatusOK)
}

// recordOTPFailure increments the consecutive failure counter. On 3 failures
// the same day the user is blocked until midnight UTC; on 6 total consecutive
// failures the user is permanently locked and admin intervention is required.
func (h *AuthHandler) recordOTPFailure(c *gin.Context, email string) {
	ctx := c.Request.Context()
	now := time.Now().UTC()

	var consecutive int32 = 1
	sameDay := false

	existing, err := h.queries.GetEmailOTPFailure(ctx, email)
	if err == nil {
		consecutive = existing.ConsecutiveFailures + 1
		if existing.LastFailureDate.Valid {
			existingDateStr := existing.LastFailureDate.Time.Format("2006-01-02")
			todayStr := now.Format("2006-01-02")
			sameDay = existingDateStr == todayStr
		}
	}

	todayStr := now.Format("2006-01-02")
	lastDate := pgtype.Date{}
	if err := lastDate.Scan(todayStr); err != nil {
		slog.Error("scan date", "error", err)
	}

	var blockedUntil sql.NullTime
	isPermanent := false

	if consecutive >= 6 {
		isPermanent = true

		user, userErr := h.queries.GetUserByEmail(ctx, email)
		if userErr == nil {
			if _, lockErr := h.queries.LockUser(ctx, user.ID); lockErr != nil {
				slog.Error("lock user from OTP failures", "error", lockErr, "email", email)
			}
		}
	} else if consecutive >= 3 && sameDay {
		endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
		blockedUntil = sql.NullTime{Time: endOfDay, Valid: true}
	}

	_, upsertErr := h.queries.UpsertEmailOTPFailure(ctx, db.UpsertEmailOTPFailureParams{
		Email:                email,
		ConsecutiveFailures:  consecutive,
		LastFailureDate:      lastDate,
		BlockedUntil:         blockedUntil,
		IsPermanentlyBlocked: isPermanent,
	})
	if upsertErr != nil {
		slog.Error("upsert OTP failure", "error", upsertErr, "email", email)
	}
}

func (h *AuthHandler) resetOTPFailures(c *gin.Context, email string) {
	if err := h.queries.ResetEmailOTPFailures(c.Request.Context(), email); err != nil {
		slog.Error("reset OTP failures", "error", err, "email", email)
	}
}

func (h *AuthHandler) AdminLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "email and password are required")
		return
	}

	admin, err := h.queries.GetAdminByEmail(c.Request.Context(), req.Email)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	if !h.hasher.Verify(admin.PasswordHash, req.Password) {
		JSONError(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	tokens, err := h.generateTokenPair(admin.ID.String(), "admin")
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "token_failed", "Failed to generate tokens")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"admin":         admin,
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_in":    tokens.ExpiresIn,
		},
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "refresh_token is required")
		return
	}

	tokenHash := authjwt.HashTokenStr(req.RefreshToken)
	stored, err := h.queries.GetRefreshTokenByHash(c.Request.Context(), tokenHash)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, "invalid_refresh_token", "Invalid or expired refresh token")
		return
	}

	_ = h.queries.RevokeRefreshToken(c.Request.Context(), tokenHash)

	tokens, err := h.generateTokenPair(stored.SubjectID.String(), stored.SubjectType)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "token_failed", "Failed to generate tokens")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_in":    tokens.ExpiresIn,
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req)

	if req.RefreshToken != "" {
		tokenHash := authjwt.HashTokenStr(req.RefreshToken)
		_ = h.queries.RevokeRefreshToken(c.Request.Context(), tokenHash)
	}

	subjectIDStr := c.GetString("subject_id")
	subjectType := c.GetString("subject_type")
	if subjectIDStr != "" && subjectType != "" {
		subjectID, err := uuid.Parse(subjectIDStr)
		if err == nil {
			_ = h.queries.RevokeAllRefreshTokensForSubject(c.Request.Context(), db.RevokeAllRefreshTokensForSubjectParams{
				SubjectID:   subjectID,
				SubjectType: subjectType,
			})
		}
	}

	JSONOK(c, http.StatusOK)
}

func (h *AuthHandler) CheckInvitation(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		JSONError(c, http.StatusBadRequest, "missing_email", "email query parameter is required")
		return
	}

	invitation, err := h.queries.GetActiveInvitationByEmail(c.Request.Context(), email)
	if err != nil {
		JSONError(c, http.StatusNotFound, "no_invitation", "No invitation found for this email. Contact your manager.")
		return
	}

	JSONSuccess(c, http.StatusOK, gin.H{
		"has_invitation": true,
		"status":         string(invitation.Status),
	})
}

func generateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate random OTP: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func sanitizeUser(user db.User) gin.H {
	return gin.H{
		"id":                user.ID,
		"email":             user.Email,
		"email_verified":    user.EmailVerified,
		"full_name":         user.FullName,
		"phone_number":      user.PhoneNumber,
		"phone_verified":    user.PhoneVerified,
		"status":            user.Status,
		"is_terms_accepted": user.IsTermsAccepted,
		"terms_accepted_at": user.TermsAcceptedAt,
		"terms_version":     user.TermsVersion,
		"created_at":        user.CreatedAt,
		"updated_at":        user.UpdatedAt,
	}
}
