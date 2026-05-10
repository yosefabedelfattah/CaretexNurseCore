-- 0001_init.up.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "citext";

-- Organizations
CREATE TABLE IF NOT EXISTS ctxnurse_organizations (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(200) NOT NULL,
    code        VARCHAR(64) NOT NULL UNIQUE,
    status      VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    created_by  UUID,
    updated_by  UUID
);

-- Departments
CREATE TABLE IF NOT EXISTS ctxnurse_departments (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES ctxnurse_organizations(id),
    name            VARCHAR(120) NOT NULL,
    code            VARCHAR(32) NOT NULL,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    created_by      UUID,
    updated_by      UUID
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_ctxnurse_departments_org_code
    ON ctxnurse_departments (organization_id, code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ctxnurse_departments_org ON ctxnurse_departments (organization_id);

-- Users
CREATE TABLE IF NOT EXISTS ctxnurse_users (
    id                       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id          UUID NOT NULL REFERENCES ctxnurse_organizations(id),
    email                    CITEXT NOT NULL,
    password_hash            TEXT NOT NULL,
    full_name                VARCHAR(200) NOT NULL,
    status                   VARCHAR(32) NOT NULL DEFAULT 'active',
    last_login_at            TIMESTAMPTZ,
    preferred_department_id  UUID REFERENCES ctxnurse_departments(id),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at               TIMESTAMPTZ,
    created_by               UUID,
    updated_by               UUID
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_ctxnurse_users_email
    ON ctxnurse_users (lower(email)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ctxnurse_users_org ON ctxnurse_users (organization_id);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_users_pref_dept ON ctxnurse_users (preferred_department_id);

-- Roles
CREATE TABLE IF NOT EXISTS ctxnurse_roles (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(64) NOT NULL UNIQUE,
    description VARCHAR(255),
    permissions JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    created_by  UUID,
    updated_by  UUID
);

-- User <-> Role
CREATE TABLE IF NOT EXISTS ctxnurse_user_roles (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES ctxnurse_users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES ctxnurse_roles(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    created_by  UUID,
    updated_by  UUID
);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_user_roles_user ON ctxnurse_user_roles (user_id);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_user_roles_role ON ctxnurse_user_roles (role_id);

-- Refresh tokens
CREATE TABLE IF NOT EXISTS ctxnurse_refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES ctxnurse_users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(128) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    user_agent  VARCHAR(255),
    ip_address  VARCHAR(64),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    created_by  UUID,
    updated_by  UUID
);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_refresh_tokens_user ON ctxnurse_refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_refresh_tokens_expires ON ctxnurse_refresh_tokens (expires_at);

-- Resident status catalog
CREATE TABLE IF NOT EXISTS ctxnurse_resident_status_codes (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES ctxnurse_organizations(id),
    code            VARCHAR(64) NOT NULL,
    name_he         VARCHAR(120) NOT NULL,
    name_en         VARCHAR(120) NOT NULL DEFAULT '',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    created_by      UUID,
    updated_by      UUID
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_ctxnurse_status_codes_org_code
    ON ctxnurse_resident_status_codes (organization_id, code) WHERE deleted_at IS NULL;

-- Resident attribute catalog
CREATE TABLE IF NOT EXISTS ctxnurse_resident_attributes (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES ctxnurse_organizations(id),
    code            VARCHAR(64) NOT NULL,
    name_he         VARCHAR(160) NOT NULL,
    name_en         VARCHAR(160) NOT NULL DEFAULT '',
    category        VARCHAR(64) NOT NULL DEFAULT '',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    created_by      UUID,
    updated_by      UUID
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_ctxnurse_attributes_org_code
    ON ctxnurse_resident_attributes (organization_id, code) WHERE deleted_at IS NULL;

-- Residents
CREATE TABLE IF NOT EXISTS ctxnurse_residents (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id     UUID NOT NULL REFERENCES ctxnurse_organizations(id),
    department_id       UUID REFERENCES ctxnurse_departments(id),
    mrn                 VARCHAR(64) NOT NULL,
    first_name          VARCHAR(120) NOT NULL,
    last_name           VARCHAR(120) NOT NULL,
    dob                 TIMESTAMPTZ,
    gender              VARCHAR(16),
    room_number         VARCHAR(32),
    phone               VARCHAR(32),
    email               VARCHAR(255),
    photo_url           VARCHAR(512),
    notes               TEXT,
    has_treatment_plan  BOOLEAN NOT NULL DEFAULT FALSE,
    caretx_id           VARCHAR(64),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    created_by          UUID,
    updated_by          UUID
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_ctxnurse_residents_org_mrn
    ON ctxnurse_residents (organization_id, mrn) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_ctxnurse_residents_org ON ctxnurse_residents (organization_id);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_residents_dept ON ctxnurse_residents (department_id);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_residents_room ON ctxnurse_residents (room_number);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_residents_has_plan ON ctxnurse_residents (has_treatment_plan);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_residents_name ON ctxnurse_residents (last_name, first_name);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_residents_caretx ON ctxnurse_residents (caretx_id);

-- Resident <-> Status (M2M)
CREATE TABLE IF NOT EXISTS ctxnurse_resident_statuses (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resident_id UUID NOT NULL REFERENCES ctxnurse_residents(id) ON DELETE CASCADE,
    status_id   UUID NOT NULL REFERENCES ctxnurse_resident_status_codes(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    created_by  UUID,
    updated_by  UUID
);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_resident_statuses_resident ON ctxnurse_resident_statuses (resident_id);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_resident_statuses_status ON ctxnurse_resident_statuses (status_id);

-- Resident <-> Attribute (M2M)
CREATE TABLE IF NOT EXISTS ctxnurse_resident_attribute_assignments (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resident_id  UUID NOT NULL REFERENCES ctxnurse_residents(id) ON DELETE CASCADE,
    attribute_id UUID NOT NULL REFERENCES ctxnurse_resident_attributes(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    created_by   UUID,
    updated_by   UUID
);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_resident_attrs_resident
    ON ctxnurse_resident_attribute_assignments (resident_id);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_resident_attrs_attribute
    ON ctxnurse_resident_attribute_assignments (attribute_id);

-- Treatment plan stub
CREATE TABLE IF NOT EXISTS ctxnurse_treatment_plans (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES ctxnurse_organizations(id),
    resident_id     UUID NOT NULL REFERENCES ctxnurse_residents(id) ON DELETE CASCADE,
    title           VARCHAR(200) NOT NULL,
    notes           TEXT,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    created_by      UUID,
    updated_by      UUID
);
CREATE INDEX IF NOT EXISTS idx_ctxnurse_treatment_plans_resident
    ON ctxnurse_treatment_plans (resident_id);
