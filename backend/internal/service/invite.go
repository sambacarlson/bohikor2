package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/Iknite-Space/bohikor2/db/sqlc"
)

var ErrActiveInvitationExists = errors.New("an active invitation already exists for this email")

type InviteStore interface {
	GetInvitationByEmail(ctx context.Context, email string) (db.Invitation, error)
	CreateInvitation(ctx context.Context, email string, companyID uuid.UUID, invitedBy pgtype.UUID) (db.Invitation, error)
	UpdateInvitationStatus(ctx context.Context, status db.InvitationStatus, id pgtype.UUID) (db.Invitation, error)
}

type AdminQuerier interface {
	GetAdminByID(ctx context.Context, id uuid.UUID) (db.Admin, error)
}

type EmailSender interface {
	SendInvitation(ctx context.Context, email string) error
}

type InviteService struct {
	store   InviteStore
	email   EmailSender
	querier AdminQuerier
}

func NewInviteService(store InviteStore, email EmailSender, querier AdminQuerier) *InviteService {
	return &InviteService{
		store:   store,
		email:   email,
		querier: querier,
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

	invitation, err := s.store.CreateInvitation(ctx, email, admin.CompanyID, invitedBy)
	if err != nil {
		return nil, fmt.Errorf("create invitation: %w", err)
	}

	if err := s.email.SendInvitation(ctx, email); err != nil {
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
