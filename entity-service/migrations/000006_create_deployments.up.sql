CREATE TYPE deployment_type_enum AS ENUM (
    'primary_production',
    'staging',
    'qa',
    'stress',
    'uat',
    'development'
);

CREATE SEQUENCE deployment_number_seq START 1;

CREATE TABLE IF NOT EXISTS deployments (
    id          UUID                 PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID                 NOT NULL REFERENCES projects(id),
    number      VARCHAR              NOT NULL UNIQUE DEFAULT 'DEP-' || LPAD(NEXTVAL('deployment_number_seq')::TEXT, 5, '0'),
    name        TEXT                 NOT NULL,
    type        deployment_type_enum NOT NULL,
    description TEXT,
    created_by  UUID                 NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ          NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ          NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_deployment_project_name UNIQUE (project_id, name)
);

CREATE INDEX IF NOT EXISTS idx_deployments_project_id   ON deployments (project_id);
CREATE INDEX IF NOT EXISTS idx_deployments_created_by   ON deployments (created_by);
CREATE INDEX IF NOT EXISTS idx_deployments_type         ON deployments (type);
CREATE INDEX IF NOT EXISTS idx_deployments_project_type ON deployments (project_id, type);
