# Data Contract & Schema

## Timezone Convention

All timestamps use `TIMESTAMPTZ` (stored in UTC). Business-logic date boundaries evaluated in `Africa/Douala` (WAT, UTC+1).

## SQL DDL (PostgreSQL 17)

### Extensions

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
```

### Enums

```sql
CREATE TYPE user_status AS ENUM ('active', 'suspended');
CREATE TYPE invitation_status AS ENUM ('pending', 'sent', 'accepted', 'revoked', 'failed');
CREATE TYPE request_status AS ENUM ('initiated', 'pending', 'success', 'failed');
```

### Admins

```sql
CREATE TABLE admins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Users (Employees)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT UNIQUE NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    full_name TEXT,
    phone_number TEXT NOT NULL,
    phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
    status user_status NOT NULL DEFAULT 'active',
    is_terms_accepted BOOLEAN NOT NULL DEFAULT FALSE,
    terms_accepted_at TIMESTAMPTZ,
    terms_version TEXT,
    user_ip_at_consent INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

> `updated_at` has no auto-update trigger. All Go update queries **must** explicitly set `updated_at = NOW()`.

### Invitations

```sql
CREATE TABLE invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT NOT NULL,
    status invitation_status NOT NULL DEFAULT 'pending',
    invited_by UUID REFERENCES admins(id),
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_one_active_invitation_per_email
ON invitations (email) WHERE (status IN ('pending', 'sent', 'accepted'));
```

### Email OTPs (temporary)

```sql
CREATE TABLE email_otps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT NOT NULL,
    code TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_otps_email ON email_otps (email);
CREATE INDEX idx_email_otps_expires_at ON email_otps (expires_at);
```

### Phone OTPs (temporary)

```sql
CREATE TABLE phone_otps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    phone_number TEXT NOT NULL,
    code TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_phone_otps_phone_number ON phone_otps (phone_number);
CREATE INDEX idx_phone_otps_expires_at ON phone_otps (expires_at);
```

> OTP codes are stored as plaintext (short-lived, 15min expiry, deleted after verification). Not hashed.

### Refresh Tokens

```sql
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
```

> Polymorphic `subject_id`/`subject_type` pattern so refresh tokens serve both users and admins.

### Events

```sql
CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    admin_id UUID REFERENCES admins(id),
    event_type TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_user_id ON events (user_id);
CREATE INDEX idx_events_event_type ON events (event_type);
CREATE INDEX idx_events_created_at ON events (created_at);
```

Auth event types: `user_invited`, `email_otp_sent`, `email_otp_verified`, `phone_otp_verified`, `signup_completed`, `user_suspended`, `user_activated`.

### Advance Requests

```sql
CREATE TABLE advance_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    amount_xaf NUMERIC(10, 2) NOT NULL DEFAULT 10000.00 CHECK (amount_xaf = 10000.00),
    status request_status NOT NULL DEFAULT 'initiated',
    campay_payout_ref TEXT UNIQUE,
    failure_reason TEXT,
    payout_duration_seconds INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_advance_requests_user_id ON advance_requests (user_id);
```

- `status` flow: `initiated` → `pending` → `success` or `failed`
- `campay_payout_ref` — Campay's transfer reference, unique
- `payout_duration_seconds` — time from `initiated` to final status
- Amount is hard-locked to 10000.00 via CHECK constraint

## Integrity Rules

- **Foreign Keys:** `ON DELETE RESTRICT` to preserve audit trails.
- **IDs:** UUIDs via `uuid_generate_v4()`.
- **Timestamps:** All `TIMESTAMPTZ`. No auto-update triggers; application layer sets `updated_at`.

## Design Decisions

| Question | Resolution |
| :--- | :--- |
| **Invitation expiry** | No automatic expiry. Valid until accepted or revoked. |
| **Phone verification** | Africa's Talking SMS OTP (Discord webhook in dev). Phone is primary identity. |
| **Data retention** | Indefinite. |
| **Terms acceptance** | Stored on `users` table. Must be accepted before requesting advance. Separate from auth flow. |
