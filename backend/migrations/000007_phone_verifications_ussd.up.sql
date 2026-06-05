-- 000007_phone_verifications_ussd.up.sql
-- Add ussd_code column to phone_verifications for collect API flow

ALTER TABLE phone_verifications ADD COLUMN ussd_code TEXT;
