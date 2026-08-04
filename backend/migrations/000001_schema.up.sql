-- 000001_schema.up.sql
-- Source of truth: docs/schema.md
-- Multi-tenant baseline for the Bohikor pivot (Epics 6-9).
-- Greenfield: no prior data. company_id is NOT NULL from the start.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ---------------------------------------------------------------------------
-- Enums
-- ---------------------------------------------------------------------------
CREATE TYPE user_status AS ENUM ('active', 'suspended', 'locked');
CREATE TYPE invitation_status AS ENUM ('pending', 'sent', 'accepted', 'revoked', 'failed');
-- 'processing' = Campay called, outcome unconfirmed (timeout/ambiguous). Never guessed.
CREATE TYPE request_status AS ENUM ('initiated', 'processing', 'pending', 'success', 'failed');
CREATE TYPE company_status AS ENUM ('active', 'suspended');

-- ---------------------------------------------------------------------------
-- Platform super-admins (global, not scoped to any company)
-- ---------------------------------------------------------------------------
CREATE TABLE platform_admins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Companies (tenants)
-- ---------------------------------------------------------------------------
CREATE TABLE companies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    status company_status NOT NULL DEFAULT 'active',
    created_by UUID REFERENCES platform_admins(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Company admins (scoped to a company)
-- ---------------------------------------------------------------------------
CREATE TABLE admins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admins_company_id ON admins (company_id);

-- ---------------------------------------------------------------------------
-- Users (Employees, scoped to a company; email stays globally unique)
-- ---------------------------------------------------------------------------
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    email TEXT UNIQUE NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    full_name TEXT,
    phone_number TEXT,
    phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
    pin_hash TEXT,
    failed_login_attempts INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    status user_status NOT NULL DEFAULT 'active',
    is_terms_accepted BOOLEAN NOT NULL DEFAULT FALSE,
    terms_accepted_at TIMESTAMPTZ,
    terms_version TEXT,
    user_ip_at_consent INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_company_id ON users (company_id);

-- ---------------------------------------------------------------------------
-- Invitations (scoped to a company)
-- ---------------------------------------------------------------------------
CREATE TABLE invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    email TEXT NOT NULL,
    status invitation_status NOT NULL DEFAULT 'pending',
    invited_by UUID REFERENCES admins(id),
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invitations_company_id ON invitations (company_id);

-- Email is globally unique, so an active invitation is enforced per email (not per company).
CREATE UNIQUE INDEX idx_one_active_invitation_per_email
ON invitations (email) WHERE (status IN ('pending', 'sent', 'accepted'));

-- ---------------------------------------------------------------------------
-- Email OTPs (temporary, pre-auth; keyed by globally-unique email)
-- ---------------------------------------------------------------------------
CREATE TABLE email_otps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT NOT NULL,
    code TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_otps_email ON email_otps (email);
CREATE INDEX idx_email_otps_expires_at ON email_otps (expires_at);

-- OTP failure tracking (pre-auth; keyed by globally-unique email)
CREATE TABLE email_otp_failures (
    email TEXT PRIMARY KEY,
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_failure_date DATE,
    blocked_until TIMESTAMPTZ,
    is_permanently_blocked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- Advance requests (scoped to a company) + payout-resilience columns (Epic 7)
-- ---------------------------------------------------------------------------
CREATE TABLE advance_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    user_id UUID NOT NULL REFERENCES users(id),
    amount_xaf NUMERIC(10, 2) NOT NULL DEFAULT 10000.00
        CHECK (amount_xaf >= 100 AND amount_xaf <= 25000),
    status request_status NOT NULL DEFAULT 'initiated',
    campay_payout_ref TEXT UNIQUE,
    failure_reason TEXT,
    payout_duration_seconds INTEGER,
    -- Resilience columns (logic lands in Epic 7)
    attempt_count INT NOT NULL DEFAULT 0,
    last_reconciled_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ,
    needs_admin_review BOOLEAN NOT NULL DEFAULT FALSE,
    reissued_from_id UUID UNIQUE REFERENCES advance_requests(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_advance_requests_company_id ON advance_requests (company_id);
CREATE INDEX idx_advance_requests_user_id ON advance_requests (user_id);
CREATE INDEX idx_advance_requests_status ON advance_requests (status);
-- Partial index: needs_admin_review is FALSE for the overwhelming majority of
-- rows, so a full index would mostly waste space. Backs the platform
-- console's cross-company needs-review queue (ListRequestsNeedingReviewAcrossCompanies).
CREATE INDEX idx_advance_requests_needs_admin_review ON advance_requests (needs_admin_review) WHERE needs_admin_review = TRUE;

-- ---------------------------------------------------------------------------
-- Company ledger (immutable entries; balance = running SUM)
-- ---------------------------------------------------------------------------
CREATE TABLE company_ledger (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    entry_type TEXT NOT NULL
        CHECK (entry_type IN ('topup', 'payout_debit', 'reversal', 'adjustment')),
    amount_xaf NUMERIC(14, 2) NOT NULL,       -- signed: topup/reversal +, debit -
    advance_request_id UUID REFERENCES advance_requests(id),  -- set for debit/reversal
    created_by UUID,                           -- platform_admin for topup/adjustment
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_company_ledger_company_id ON company_ledger (company_id);

-- ---------------------------------------------------------------------------
-- Phone verifications (Campay collect-based; scoped to a company)
-- ---------------------------------------------------------------------------
CREATE TABLE phone_verifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    phone_number TEXT NOT NULL,
    amount_xaf NUMERIC(10, 2) NOT NULL DEFAULT 100.00,
    campay_payout_ref TEXT UNIQUE,
    status request_status NOT NULL DEFAULT 'initiated',
    failure_reason TEXT,
    ussd_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_phone_verifications_company_id ON phone_verifications (company_id);
CREATE INDEX idx_phone_verifications_user_id ON phone_verifications (user_id);
CREATE INDEX idx_phone_verifications_status ON phone_verifications (status);

-- ---------------------------------------------------------------------------
-- Settings (per-company; seeded on company create). No global seed here.
-- ---------------------------------------------------------------------------
CREATE TABLE settings (
    company_id UUID NOT NULL REFERENCES companies(id),
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES admins(id),
    PRIMARY KEY (company_id, key)
);

-- ---------------------------------------------------------------------------
-- Event log (company_id nullable: platform events have no company)
-- ---------------------------------------------------------------------------
CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID REFERENCES companies(id),
    user_id UUID REFERENCES users(id),
    admin_id UUID REFERENCES admins(id),
    event_type TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_company_id ON events (company_id);
CREATE INDEX idx_events_user_id ON events (user_id);
CREATE INDEX idx_events_event_type ON events (event_type);
CREATE INDEX idx_events_created_at ON events (created_at);

-- ---------------------------------------------------------------------------
-- Refresh tokens (opaque, rotating). subject_type now allows platform_admin.
-- ---------------------------------------------------------------------------
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token_hash TEXT NOT NULL UNIQUE,
    subject_id UUID NOT NULL,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('user', 'admin', 'platform_admin')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
CREATE INDEX idx_refresh_tokens_subject ON refresh_tokens (subject_id, subject_type);
