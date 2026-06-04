-- 000006_epic4_settings.down.sql

ALTER TABLE advance_requests DROP CONSTRAINT IF EXISTS advance_requests_amount_xaf_check;
ALTER TABLE advance_requests ADD CONSTRAINT advance_requests_amount_xaf_check
    CHECK (amount_xaf = 10000.00);

DROP TABLE email_otp_failures;

DROP TABLE settings;
