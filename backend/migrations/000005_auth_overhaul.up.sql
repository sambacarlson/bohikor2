-- 000005_auth_overhaul.up.sql
-- PIN-based auth, phone verification via Campay, login rate limiting

-- Add 'locked' to user_status enum
ALTER TYPE user_status ADD VALUE 'locked';

-- Make phone_number nullable (users may not have a phone yet)
ALTER TABLE users ALTER COLUMN phone_number DROP NOT NULL;

-- Add PIN and login security columns to users
ALTER TABLE users ADD COLUMN pin_hash TEXT;
ALTER TABLE users ADD COLUMN failed_login_attempts INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN locked_until TIMESTAMPTZ;

-- Drop phone_otps table (no longer needed)
DROP TABLE phone_otps;

-- Create phone_verifications table (Campay withdrawal-based verification)
CREATE TABLE phone_verifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    phone_number TEXT NOT NULL,
    amount_xaf NUMERIC(10, 2) NOT NULL DEFAULT 100.00,
    campay_payout_ref TEXT UNIQUE,
    status request_status NOT NULL DEFAULT 'initiated',
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_phone_verifications_user_id ON phone_verifications (user_id);
CREATE INDEX idx_phone_verifications_status ON phone_verifications (status);