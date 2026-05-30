-- 000004_own_auth.up.sql
-- Remove Firebase dependency, add own auth tables

-- Drop firebase_uid from admins, add password_hash
ALTER TABLE admins DROP COLUMN firebase_uid;
ALTER TABLE admins ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

-- Drop firebase_uid from users
ALTER TABLE users DROP COLUMN firebase_uid;

-- Phone OTPs (same pattern as email_otps)
CREATE TABLE phone_otps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone_number TEXT NOT NULL,
    code TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_phone_otps_phone_number ON phone_otps (phone_number);
CREATE INDEX idx_phone_otps_expires_at ON phone_otps (expires_at);

-- Refresh tokens
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token_hash TEXT NOT NULL UNIQUE,
    subject_id UUID NOT NULL,
    subject_type TEXT NOT NULL CHECK (subject_type IN ('user', 'admin')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
CREATE INDEX idx_refresh_tokens_subject ON refresh_tokens (subject_id, subject_type);
