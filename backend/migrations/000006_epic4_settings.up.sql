-- 000006_epic4_settings.up.sql
-- Settings, OTP rate limiting, flexible advance amount

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES admins(id)
);

INSERT INTO settings (key, value) VALUES
    ('kill_switch_enabled', 'false'),
    ('request_window_start_day', '15'),
    ('request_window_end_day', '0'),
    ('daily_request_limit', '0'),
    ('monthly_request_limit', '1'),
    ('advance_amount_xaf', '10000');

CREATE TABLE email_otp_failures (
    email TEXT PRIMARY KEY,
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_failure_date DATE,
    blocked_until TIMESTAMPTZ,
    is_permanently_blocked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE advance_requests DROP CONSTRAINT IF EXISTS advance_requests_amount_xaf_check;
ALTER TABLE advance_requests ADD CONSTRAINT advance_requests_amount_xaf_check
    CHECK (amount_xaf >= 100 AND amount_xaf <= 25000);
