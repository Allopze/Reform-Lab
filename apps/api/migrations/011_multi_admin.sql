-- Remove single-admin constraint to allow multiple administrators.
DROP INDEX IF EXISTS idx_users_single_admin;
