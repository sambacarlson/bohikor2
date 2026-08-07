package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

var ErrActiveInvitationExists = errors.New("an active invitation already exists for this email")

// ErrEmailAlreadyRegistered is returned when the invited email already
// belongs to a user (employee) or admin anywhere in the system — email is
// meant to be a single identity across the whole app, not just unique
// within each of the users/admins tables independently. This also covers
// an admin inviting their own email, since their email is already an
// admins row.
var ErrEmailAlreadyRegistered = errors.New("this email is already registered as an employee or company admin")

type InviteStore interface {
	GetInvitationByEmail(ctx context.Context, email string) (db.Invitation, error)
	CreateInvitation(ctx context.Context, email string, companyID uuid.UUID, invitedBy pgtype.UUID) (db.Invitation, error)
	UpdateInvitationStatus(ctx context.Context, status db.InvitationStatus, id pgtype.UUID) (db.Invitation, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetAdminByEmail(ctx context.Context, email string) (db.Admin, error)
}

type AdminQuerier interface {
	GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error)
	GetCompanyByID(ctx context.Context, id uuid.UUID) (db.Company, error)
}

type EmailSender interface {
	SendInvitation(ctx context.Context, email, signupURL string) error
}

type InviteService struct {
	store           InviteStore
	email           EmailSender
	querier         AdminQuerier
	frontendBaseURL string
}

func NewInviteService(store InviteStore, email EmailSender, querier AdminQuerier, frontendBaseURL string) *InviteService {
	return &InviteService{
		store:           store,
		email:           email,
		querier:         querier,
		frontendBaseURL: frontendBaseURL,
	}
}

type InviteResult struct {
	Invitation db.Invitation
}

func (s *InviteService) Invite(ctx context.Context, email string, adminID string) (*InviteResult, error) {
	id, err := uuid.Parse(adminID)
	if err != nil {
		return nil, fmt.Errorf("parse admin id: %w", err)
	}

	admin, err := s.querier.GetAdminByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("lookup admin: %w", err)
	}

	invitedBy := pgtype.UUID{Bytes: admin.ID, Valid: true}

	existing, err := s.store.GetInvitationByEmail(ctx, email)
	if err == nil {
		if existing.Status == db.InvitationStatusPending || existing.Status == db.InvitationStatusSent || existing.Status == db.InvitationStatusAccepted {
			return nil, ErrActiveInvitationExists
		}
	}

	// Email is one identity across the whole app: reject if it's already a
	// user (any company) or an admin (any company, including the inviting
	// admin's own email) before creating the invitation.
	if _, err := s.store.GetUserByEmail(ctx, email); err == nil {
		return nil, ErrEmailAlreadyRegistered
	}
	if _, err := s.store.GetAdminByEmail(ctx, email); err == nil {
		return nil, ErrEmailAlreadyRegistered
	}

	// Company lookup happens only once the invite is known to actually
	// proceed (past the duplicate-invitation check), since it's needed
	// solely to build the signup URL below.
	company, err := s.querier.GetCompanyByID(ctx, admin.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("lookup company: %w", err)
	}

	invitation, err := s.store.CreateInvitation(ctx, email, admin.CompanyID, invitedBy)
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	signupURL := fmt.Sprintf("%s/%s/signup?email=%s", strings.TrimRight(s.frontendBaseURL, "/"), company.Slug, url.QueryEscape(email))
	if err := s.email.SendInvitation(ctx, email, signupURL); err != nil {
		invID := pgtype.UUID{Bytes: invitation.ID, Valid: true}
		_, _ = s.store.UpdateInvitationStatus(ctx, db.InvitationStatusFailed, invID)
		return nil, fmt.Errorf("send invitation email: %w", err)
	}

	invID := pgtype.UUID{Bytes: invitation.ID, Valid: true}
	updated, err := s.store.UpdateInvitationStatus(ctx, db.InvitationStatusSent, invID)
	if err != nil {
		return nil, fmt.Errorf("update invitation status to sent: %w", err)
	}

	return &InviteResult{Invitation: updated}, nil
}

type RealInviteStore struct {
	queries *db.Queries
}

func NewRealInviteStore(queries *db.Queries) *RealInviteStore {
	return &RealInviteStore{queries: queries}
}

func (s *RealInviteStore) GetInvitationByEmail(ctx context.Context, email string) (db.Invitation, error) {
	return s.queries.GetInvitationByEmail(ctx, email)
}

func (s *RealInviteStore) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	return s.queries.GetUserByEmail(ctx, email)
}

func (s *RealInviteStore) GetAdminByEmail(ctx context.Context, email string) (db.Admin, error) {
	return s.queries.GetAdminByEmail(ctx, email)
}

func (s *RealInviteStore) CreateInvitation(ctx context.Context, email string, companyID uuid.UUID, invitedBy pgtype.UUID) (db.Invitation, error) {
	return s.queries.CreateInvitation(ctx, db.CreateInvitationParams{
		CompanyID: companyID,
		Email:     email,
		InvitedBy: invitedBy,
		SentAt:    time.Now().UTC(),
	})
}

func (s *RealInviteStore) UpdateInvitationStatus(ctx context.Context, status db.InvitationStatus, id pgtype.UUID) (db.Invitation, error) {
	return s.queries.UpdateInvitationStatus(ctx, db.UpdateInvitationStatusParams{
		Status: status,
		ID:     uuid.UUID(id.Bytes),
	})
}
