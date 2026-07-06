CREATE TYPE subscription_type_enum AS ENUM (
    'development_support',
    'managed_cloud_subscription',
    'evaluation_subscription',
    'subscription',
    'cloud_evaluation_support',
    'internal',
    'platformer_subscription',
    'cloud_support',
    'professional_services'
);

CREATE TYPE closure_status_enum AS ENUM (
    'notify',
    'closed',
    'open',
    'read_only',
    'restricted',
    'suspended'
);

CREATE TABLE IF NOT EXISTS projects (
    id                TEXT                   PRIMARY KEY,
    account_id        TEXT                   NOT NULL,
    sf_id             TEXT                   NOT NULL UNIQUE,
    name              TEXT                   NOT NULL,
    key               TEXT                   NOT NULL UNIQUE,
    subscription_type subscription_type_enum NOT NULL,
    closure_status    closure_status_enum    NULL,
    start_date        DATE                   NOT NULL,
    end_date          DATE                   NOT NULL,
    created_at        TIMESTAMPTZ            NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ            NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_project_dates   CHECK (end_date > start_date),
    CONSTRAINT fk_projects_account FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_projects_account_id        ON projects (account_id);
CREATE INDEX IF NOT EXISTS idx_projects_subscription_type ON projects (subscription_type);
CREATE INDEX IF NOT EXISTS idx_projects_start_date        ON projects (start_date);
CREATE INDEX IF NOT EXISTS idx_projects_end_date          ON projects (end_date);
CREATE INDEX IF NOT EXISTS idx_projects_account_dates     ON projects (account_id, start_date, end_date);
