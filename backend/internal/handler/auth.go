package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
	"github.com/Iknite-Space/bohikor2/internal/authjwt"
	"github.com/Iknite-Space/bohikor2/internal/authpassword"
	"github.com/Iknite-Space/bohikor2/internal/sms"
)

type AuthHandler struct {
	queries      authQuerier
	tokenService authjwt.TokenService
	hasher       authpassword.Hasher
	smsSender    sms.Sender
	refreshExpiry time.Duration
}

type authQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetUserByPhoneNumber(ctx context.Context, phoneNumber string) (db.User, error)
	CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error)
	GetAdminByEmail(ctx context.Context, email string) (db.Admin, error)
	CreatePhoneOTP(ctx context.Context, arg db.CreatePhoneOTPParams) (db.PhoneOtp, error)
	GetPhoneOTPByPhoneNumber(ctx context.Context, phoneNumber string) (db.PhoneOtp, error)
	DeletePhoneOTP(ctx context.Context, phoneNumber string) error
	CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (db.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllRefreshTokensForSubject(ctx context.Context, arg db.RevokeAllRefreshTokensForSubjectParams) error
	GetActiveInvitationByEmail(ctx context.Context, email string) (db.Invitation, error)
	CreateEmailOTP(ctx context.Context, arg db.CreateEmailOTPParams) (db.EmailOtp, error)
	GetEmailOTPByEmail(ctx context.Context, email string) (db.EmailOtp, error)
	DeleteEmailOTP(ctx context.Context, email string) error
	AcceptInvitation(ctx context.Context, email string) (db.Invitation, error)
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
}

func NewAuthHandler(
	queries authQuerier,
	tokenService authjwt.TokenService,
	hasher authpassword.Hasher,
	smsSender sms.Sender,
	refreshExpiry time.Duration,
) *AuthHandler {
	return &AuthHandler{
		queries:        queries,
		tokenService:   tokenService,
		hasher:         hasher,
		smsSender:      smsSender,
		refreshExpiry:  refreshExpiry,
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

func (h *AuthHandler) SendPhoneOTP(c *gin.Context) {
	var req struct {
		PhoneNumber string `json:"phone_number" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "phone_number is required")
		return
	}

	code, err := generateOTP()
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "otp_generation_failed", "Failed to generate OTP")
		return
	}

	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	_, err = h.queries.CreatePhoneOTP(c.Request.Context(), db.CreatePhoneOTPParams{
		PhoneNumber: req.PhoneNumber,
		Code:        code,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "store_otp_failed", "Failed to store OTP")
		return
	}

	if err := h.smsSender.SendOTP(c.Request.Context(), req.PhoneNumber, code); err != nil {
		JSONError(c, http.StatusInternalServerError, "send_otp_failed", "Failed to send OTP via SMS")
		return
	}

	JSONOK(c, http.StatusOK)
}

func (h *AuthHandler) VerifyPhoneOTP(c *gin.Context) {
	var req struct {
		PhoneNumber string `json:"phone_number" binding:"required"`
		Code        string `json:"code" binding:"required"`
		Email       string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "phone_number and code are required")
		return
	}

	storedOTP, err := h.queries.GetPhoneOTPByPhoneNumber(c.Request.Context(), req.PhoneNumber)
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_otp", "Invalid or expired OTP")
		return
	}

	if storedOTP.Code != req.Code {
		JSONError(c, http.StatusBadRequest, "invalid_otp", "Invalid OTP code")
		return
	}

	_ = h.queries.DeletePhoneOTP(c.Request.Context(), req.PhoneNumber)

	existingUser, err := h.queries.GetUserByPhoneNumber(c.Request.Context(), req.PhoneNumber)
	if err == nil {
		tokens, err := h.generateTokenPair(existingUser.ID.String(), "user")
		if err != nil {
			JSONError(c, http.StatusInternalServerError, "token_failed", "Failed to generate tokens")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"user":          existingUser,
				"access_token":  tokens.AccessToken,
				"refresh_token": tokens.RefreshToken,
				"expires_in":    tokens.ExpiresIn,
			},
		})
		return
	}

	if req.Email == "" {
		JSONError(c, http.StatusBadRequest, "email_required", "New users must provide an email address")
		return
	}

	_, err = h.queries.GetUserByEmail(c.Request.Context(), req.Email)
	if err == nil {
		JSONError(c, http.StatusConflict, "user_exists", "User already exists with this email. Please log in instead.")
		return
	}

	_, err = h.queries.GetActiveInvitationByEmail(c.Request.Context(), req.Email)
	if err != nil {
		JSONError(c, http.StatusForbidden, "no_invitation", "No active invitation found for this email")
		return
	}

	user, err := h.queries.CreateUser(c.Request.Context(), db.CreateUserParams{
		Email:         req.Email,
		EmailVerified: true,
		FullName:      pgtype.Text{Valid: false},
		PhoneNumber:   req.PhoneNumber,
		PhoneVerified: true,
		Status:        db.UserStatusActive,
	})
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "create_user_failed", "Failed to create user")
		return
	}

	_, _ = h.queries.AcceptInvitation(c.Request.Context(), req.Email)

	metadata, _ := json.Marshal(map[string]string{"source": "mobile"})
	_, _ = h.queries.CreateEvent(c.Request.Context(), db.CreateEventParams{
		UserID:    pgtype.UUID{Bytes: user.ID, Valid: true},
		EventType: "signup_completed",
		Metadata:  metadata,
	})

	tokens, err := h.generateTokenPair(user.ID.String(), "user")
	if err != nil {
		JSONError(c, http.StatusInternalServerError, "token_failed", "Failed to generate tokens")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": gin.H{
			"user":          user,
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_in":    tokens.ExpiresIn,
		},
	})
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

func (h *AuthHandler) SendEmailOTP(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "email is required")
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

	JSONOK(c, http.StatusOK)
}

func (h *AuthHandler) VerifyEmailOTP(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_request", "email and code are required")
		return
	}

	storedOTP, err := h.queries.GetEmailOTPByEmail(c.Request.Context(), req.Email)
	if err != nil {
		JSONError(c, http.StatusBadRequest, "invalid_otp", "Invalid or expired OTP")
		return
	}

	if storedOTP.Code != req.Code {
		JSONError(c, http.StatusBadRequest, "invalid_otp", "Invalid OTP code")
		return
	}

	if err := h.queries.DeleteEmailOTP(c.Request.Context(), req.Email); err != nil {
		JSONError(c, http.StatusInternalServerError, "cleanup_failed", "Failed to cleanup OTP")
		return
	}

	if _, err := h.queries.AcceptInvitation(c.Request.Context(), req.Email); err != nil {
		JSONError(c, http.StatusInternalServerError, "accept_invitation_failed", "Failed to accept invitation")
		return
	}

	JSONOK(c, http.StatusOK)
}

func generateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate random OTP: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
