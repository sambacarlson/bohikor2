-- 000004_own_auth.down.sql

DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS phone_otps;

ALTER TABLE users ADD COLUMN firebase_uid TEXT UNIQUE;

ALTER TABLE admins DROP COLUMN password_hash;
ALTER TABLE admins ADD COLUMN firebase_uid TEXT UNIQUE NOT NULL;
