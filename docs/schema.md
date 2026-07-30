# Data Contract & Schema

Multi-tenant baseline (Epic 6+). Every domain table carries a `company_id`; the company
is derived from the JWT claim on authenticated calls, never from the request body/URL.
Email is **globally unique**, so login resolves a user's company from their email.

## Timezone Convention

All timestamps use `TIMESTAMPTZ` (stored in UTC). Business-logic date boundaries evaluated in `Africa/Douala` (WAT, UTC+1).

## SQL DDL (PostgreSQL 18.4)

### Extensions

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
```

### Enums

```sql
CREATE TYPE user_status AS ENUM ('active', 'suspended', 'locked');
CREATE TYPE invitation_status AS ENUM ('pending', 'sent', 'accepted', 'revoked', 'failed');
CREATE TYPE request_status AS ENUM ('initiated', 'processing', 'pending', 'success', 'failed');
CREATE TYPE company_status AS ENUM ('active', 'suspended');
```

> `processing` = Campay was called but the outcome is unconfirmed (timeout/ambiguous). It is
> never guessed; a reconciler resolves it (Epic 7). Terminal states are `success` / `failed`.

### Platform Admins (super-admins, global)

```sql
CREATE TABLE platform_admins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Platform admins provision companies + their first admin and set each company's balance.
They belong to no company; their access token carries an **empty** `company_id` claim.

### Companies (tenants)

```sql
CREATE TABLE companies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    status company_status NOT NULL DEFAULT 'active',   -- active | suspended
    created_by UUID REFERENCES platform_admins(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Admins (company-scoped)

```sql
CREATE TABLE admins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_admins_company_id ON admins (company_id);
```

### Users (Employees, company-scoped)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    email TEXT UNIQUE NOT NULL,                          -- globally unique
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
```

> `updated_at` has no auto-update trigger. All Go update queries **must** explicitly set `updated_at = NOW()`.

### Invitations (company-scoped)

```sql
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

-- Email is globally unique, so one active invitation is enforced per email.
CREATE UNIQUE INDEX idx_one_active_invitation_per_email
ON invitations (email) WHERE (status IN ('pending', 'sent', 'accepted'));
```

> The invitation is the **sole source** of a new user's `company_id`: signup (`CreatePin`)
> reads the active invitation by email and stamps the new user with its company.

### Email OTPs & OTP failures (pre-auth, keyed by globally-unique email)

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

CREATE TABLE email_otp_failures (
    email TEXT PRIMARY KEY,
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_failure_date DATE,
    blocked_until TIMESTAMPTZ,
    is_permanently_blocked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

> These are pre-auth (no company context yet) and are intentionally **not** company-scoped;
> email is globally unique so it fully identifies the account.

### Advance Requests (company-scoped) + resilience columns

```sql
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
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_advance_requests_company_id ON advance_requests (company_id);
CREATE INDEX idx_advance_requests_user_id ON advance_requests (user_id);
CREATE INDEX idx_advance_requests_status ON advance_requests (status);
```

- `id` doubles as the Campay `external_reference` (idempotency key).
- The resilience columns are created now (greenfield) but exercised in Epic 7's reconciler.

### Company Ledger (immutable; balance = running SUM)

```sql
CREATE TABLE company_ledger (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id),
    entry_type TEXT NOT NULL
        CHECK (entry_type IN ('topup', 'payout_debit', 'reversal', 'adjustment')),
    amount_xaf NUMERIC(14, 2) NOT NULL,       -- signed: topup/reversal +, debit -
    advance_request_id UUID REFERENCES advance_requests(id),
    created_by UUID,                           -- platform_admin for topup/adjustment
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_company_ledger_company_id ON company_ledger (company_id);
-- balance(company_id) = SUM(amount_xaf)
```

- Epic 6 uses `topup` only (super-admin funds a company) + `GetCompanyBalance`.
- `payout_debit` / `reversal` (payout float reservation) arrive in Epic 7.

### Phone Verifications (Campay Collect, company-scoped)

```sql
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
```

> Phone verification uses Campay Collect API (`POST /collect/`): the backend requests a small
> payment from the phone; the user dials the returned USSD code and enters their PIN; a webhook
> confirms and marks the phone verified. Amount configurable via `CAMPAY_PHONE_VERIFICATION_AMOUNT`.

### Settings (per-company; seeded on company create)

```sql
CREATE TABLE settings (
    company_id UUID NOT NULL REFERENCES companies(id),
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES admins(id),
    PRIMARY KEY (company_id, key)
);
```

Seeded per company on creation (no global seed): `kill_switch_enabled=false`,
`request_window_start_day=15`, `request_window_end_day=0`, `daily_request_limit=0`,
`monthly_request_limit=1`, `advance_amount_xaf=10000`.

### Events (audit log; `company_id` nullable for platform events)

```sql
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
```

Event types include: `signup_completed`, `request_initiated`, `payout_pending`, `payout_success`,
`payout_failed`, `phone_verification_initiated`, `phone_verified`.

### Refresh Tokens

```sql
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
```

> Polymorphic `subject_id`/`subject_type` serves users, company admins, and platform admins.
> On refresh, `company_id` is **re-derived** from the subject (empty for `platform_admin`), so
> reassignment or company suspension is reflected in the next access token.

## JWT & Scoping

- Access-token claims: `sub` (subject id), `role` ∈ `user | admin | platform_admin`, `company_id`
  (empty for platform admins).
- `JWTAuth` middleware sets `subject_id`, `subject_type`, `claim_company_id` in the Gin context.
- `RequireAdmin` / `RequireActiveUser` load the record, assert `record.company_id == claim` (403
  "company mismatch" otherwise), reject a suspended company, then set `company_id` in the context.
- `RequirePlatformAdmin` gates `/api/platform/*`.
- Every tenant query takes its `company_id` from the loaded record / claim — never from the request.

## Integrity Rules

- **Foreign Keys:** preserve audit trails; `ON DELETE RESTRICT` where deletion would orphan history.
- **IDs:** UUIDs via `uuid_generate_v4()`.
- **Timestamps:** all `TIMESTAMPTZ`; no auto-update triggers — the app sets `updated_at`.
- **Migrations:** single numbered baseline (`000001`), no `IF [NOT] EXISTS`. Greenfield.

## Design Decisions

| Question | Resolution |
| :--- | :--- |
| **Tenancy** | Every domain table has `company_id NOT NULL` (nullable only on `events`). |
| **Email uniqueness** | Globally unique; login resolves company from email → returns `company_slug`. |
| **Onboarding** | Platform super-admin provisions companies + first admin + balance. Self-signup deferred. |
| **Campay** | One platform Campay account; each company draws payouts from its own ledger balance. |
| **Top-ups** | Super-admin only. Company admins see balance (view-only). |
| **Invitation expiry** | No automatic expiry. Valid until accepted or revoked. |
| **Terms acceptance** | Stored on `users`; required before requesting an advance. |
