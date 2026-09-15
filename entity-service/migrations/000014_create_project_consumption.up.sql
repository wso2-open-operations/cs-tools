-- Product-consumption provisioning state for a project.
--
-- Mirrors the ServiceNow fields this data has always lived in on the
-- customer_project record (u_product_consumption_choreo_application_status and
-- friends), but as a separate 1:1 table rather than columns on `projects`:
--
--   * the rows are optional — only projects whose customer has started the
--     license-download flow ever get one, so widening `projects` would add six
--     always-NULL columns to the hottest table in the schema;
--   * it keeps the credential columns in one place that can be GRANTed, audited
--     and rotated independently of general project reads.
--
-- `status` is a smallint rather than an enum type on purpose. It is the
-- provisioning state machine's step number (1 pending, 2 application created,
-- 3 subscribed, 4 credentials generated, 5 secret keys generated) and it is
-- wire-compatible with the numeric `status` the customer portal and the Choreo
-- subscription operation already exchange. Introducing a named enum here would
-- mean translating in both directions for no gain, and the ordering is
-- meaningful — the flow resumes at whatever step it left off, so `status >= n`
-- comparisons are the point.
--
-- The three credential columns are BYTEA, not TEXT: they hold AEAD ciphertext
-- produced by internal/crypto, never the raw values. See that package for the
-- key handling and for how to migrate to an external KMS without a schema
-- change.
-- project_id is TEXT, not UUID, to match `projects.id` as migration 000003
-- declares it. Note that 000006 declares deployments.project_id as UUID with a
-- foreign key to that same TEXT column, which PostgreSQL cannot create — the
-- committed migration chain does not currently apply end to end, and nothing
-- applies it at startup (internal/db/migrate.go is a stub). This table follows
-- 000003 because that is the definition of `projects` in this repository; if
-- the live schema turns out to use UUID, this column changes with it.
CREATE TABLE IF NOT EXISTS project_consumption (
    project_id           TEXT        PRIMARY KEY REFERENCES projects (id) ON DELETE CASCADE,
    status               SMALLINT    NOT NULL DEFAULT 1,
    choreo_application_id TEXT       NULL,
    consumer_key         TEXT        NULL,
    consumer_secret      BYTEA       NULL,
    primary_secret_key   BYTEA       NULL,
    secondary_secret_key BYTEA       NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_project_consumption_status
        CHECK (status BETWEEN 1 AND 5),

    -- Each step's artefacts must be present once the state machine claims to
    -- have passed that step. Without these a partially-failed PATCH could
    -- advance `status` past a step whose output was never stored, and the next
    -- license download would resume from the wrong place — creating a second
    -- Choreo application for the same customer.
    CONSTRAINT chk_project_consumption_application_id
        CHECK (status < 2 OR choreo_application_id IS NOT NULL),
    CONSTRAINT chk_project_consumption_credentials
        CHECK (status < 4 OR (consumer_key IS NOT NULL AND consumer_secret IS NOT NULL)),
    CONSTRAINT chk_project_consumption_secret_keys
        CHECK (status < 5 OR (primary_secret_key IS NOT NULL AND secondary_secret_key IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_project_consumption_status
    ON project_consumption (status);
