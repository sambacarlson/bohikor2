-- 000001_schema.down.sql
-- Reverse of 000001_schema.up.sql
-- Drop order respects foreign key dependencies.

DROP TABLE refresh_tokens;
DROP TABLE events;
DROP TABLE settings;
DROP TABLE phone_verifications;
DROP TABLE company_ledger;
DROP TABLE advance_requests;
DROP TABLE email_otp_failures;
DROP TABLE email_otps;
DROP TABLE invitations;
DROP TABLE users;
DROP TABLE admins;
DROP TABLE companies;
DROP TABLE platform_admins;

DROP TYPE company_status;
DROP TYPE request_status;
DROP TYPE invitation_status;
DROP TYPE user_status;
