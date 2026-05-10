-- Migration 0002: extend ctxnurse_users with caregiver-profile columns and the
-- external Caretex UID. These columns power the GET /api/v1/users picker used
-- when assigning tasks to caregivers, and the POST /api/v1/sync/caretx/users
-- endpoint that imports staff from the external Caretx platform.

ALTER TABLE ctxnurse_users
    ADD COLUMN IF NOT EXISTS role        VARCHAR(32),
    ADD COLUMN IF NOT EXISTS phone       VARCHAR(64),
    ADD COLUMN IF NOT EXISTS photo_url   VARCHAR(512),
    ADD COLUMN IF NOT EXISTS caretx_uid  VARCHAR(128);

-- Index for the role classifier so the picker can filter quickly
-- (e.g. "show only nurses on this department").
CREATE INDEX IF NOT EXISTS idx_ctxnurse_users_role
    ON ctxnurse_users (role)
    WHERE deleted_at IS NULL;

-- Index for the Caretex upsert key. Not unique: collisions between tenants
-- in different orgs are valid, and we already scope every upsert by org.
CREATE INDEX IF NOT EXISTS idx_ctxnurse_users_caretx_uid
    ON ctxnurse_users (organization_id, caretx_uid)
    WHERE caretx_uid IS NOT NULL AND deleted_at IS NULL;
