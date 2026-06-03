-- 000005_auth_overhaul.down.sql
-- Reverse the auth overhaul migration

-- Drop phone_verifications
DROP TABLE phone_verifications;

-- Re-create phone_otps
CREATE TABLE phone_otps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone_number TEXT NOT NULL,
    code TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_phone_otps_phone_number ON phone_otps (phone_number);
CREATE INDEX idx_phone_otps_expires_at ON phone_otps (expires_at);

-- Remove PIN and login security columns
ALTER TABLE users DROP COLUMN locked_until;
ALTER TABLE users DROP COLUMN failed_login_attempts;
ALTER TABLE users DROP COLUMN pin_hash;

-- Make phone_number NOT NULL again
ALTER TABLE users ALTER COLUMN phone_number SET NOT NULL;

-- Remove 'locked' from user_status enum (requires rebuilding the enum)
-- Note: PostgreSQL does not support removing enum values.
-- This is a known limitation; in production you would need to rebuild the enum.
-- For development, you may need to manually handle this.