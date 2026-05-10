-- Reverse 0002: drop caregiver-profile columns and Caretex UID.
DROP INDEX IF EXISTS idx_ctxnurse_users_caretx_uid;
DROP INDEX IF EXISTS idx_ctxnurse_users_role;

ALTER TABLE ctxnurse_users
    DROP COLUMN IF EXISTS caretx_uid,
    DROP COLUMN IF EXISTS photo_url,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS role;
