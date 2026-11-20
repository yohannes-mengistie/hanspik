DROP INDEX IF EXISTS idx_users_disabled_at;
ALTER TABLE users DROP COLUMN IF EXISTS disabled_at;
