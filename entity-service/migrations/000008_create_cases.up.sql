CREATE TYPE case_work_state_enum AS ENUM (
  'ongoing',
  'paused'
);

CREATE TYPE case_priority_enum AS ENUM (
  'catastrophic',
  'critical',
  'high',
  'medium',
  'low'
);

CREATE TYPE case_state_enum AS ENUM (
  'open',
  'work_in_progress',
  'waiting_on_wso2',
  'awaiting_info',
  'reopened',
  'solution_proposed',
  'closed'
);

CREATE TYPE case_issue_type_enum AS ENUM (
  'error',
  'partial_outage',
  'performance_degradation',
  'question',
  'security_or_compliance',
  'total_outage'
);

CREATE SEQUENCE cases_number_seq START 1 INCREMENT 1;
CREATE SEQUENCE cases_internal_id_seq START 1 INCREMENT 1;

CREATE TABLE cases (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  number              VARCHAR UNIQUE NOT NULL DEFAULT LPAD(nextval('cases_number_seq')::TEXT, 9, '0'),
  internal_id         VARCHAR UNIQUE NOT NULL DEFAULT LPAD(nextval('cases_internal_id_seq')::TEXT, 9, '0'),
  created_by          UUID NOT NULL REFERENCES users(id),
  project_id          UUID NOT NULL REFERENCES projects(id),
  deployment_id       UUID NOT NULL REFERENCES deployments(id),
  deployed_product_id UUID NOT NULL REFERENCES deployed_products(id),
  subject             VARCHAR NOT NULL,
  description         TEXT NOT NULL,
  priority            case_priority_enum NOT NULL,
  issue_type          case_issue_type_enum NOT NULL,
  state               case_state_enum NOT NULL,
  created_at          TIMESTAMP DEFAULT NOW(),
  updated_at          TIMESTAMP DEFAULT NOW(),
  closed_at           TIMESTAMP,
  assigned_engineer   UUID NULL REFERENCES users(id),
  work_state          case_work_state_enum NULL,
  parent_case_id      UUID NULL REFERENCES cases(id),
  related_case_id     UUID NULL REFERENCES cases(id),

  CONSTRAINT chk_closed_at_on_closed
    CHECK (state != 'closed' OR closed_at IS NOT NULL),

  CONSTRAINT chk_closed_at_not_on_open
    CHECK (closed_at IS NULL OR state = 'closed'),

  CONSTRAINT chk_work_state_only_on_wip
    CHECK (
      (state = 'work_in_progress' AND work_state IS NOT NULL)
      OR
      (state != 'work_in_progress' AND work_state IS NULL)
    )
);

CREATE OR REPLACE FUNCTION check_case_deployment_belongs_to_project()
RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM deployments
    WHERE id         = NEW.deployment_id
      AND project_id = NEW.project_id
  ) THEN
    RAISE EXCEPTION
      'deployment_id % does not belong to project_id %.', NEW.deployment_id, NEW.project_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_case_deployment_belongs_to_project
  BEFORE INSERT OR UPDATE ON cases
  FOR EACH ROW EXECUTE FUNCTION check_case_deployment_belongs_to_project();

CREATE OR REPLACE FUNCTION check_case_deployed_product_belongs_to_deployment()
RETURNS TRIGGER AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM deployed_products
    WHERE id            = NEW.deployed_product_id
      AND deployment_id = NEW.deployment_id
  ) THEN
    RAISE EXCEPTION
      'deployed_product_id % does not belong to deployment_id %.', NEW.deployed_product_id, NEW.deployment_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_case_deployed_product_belongs_to_deployment
  BEFORE INSERT OR UPDATE ON cases
  FOR EACH ROW EXECUTE FUNCTION check_case_deployed_product_belongs_to_deployment();

CREATE OR REPLACE FUNCTION check_case_catastrophic_priority()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.priority = 'catastrophic' AND NOT EXISTS (
    SELECT 1 FROM projects
    WHERE id                = NEW.project_id
      AND subscription_type = 'managed_cloud_subscription'
  ) THEN
    RAISE EXCEPTION
      'catastrophic priority is only allowed for managed_cloud_subscription projects. project_id % has a different subscription type.', NEW.project_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_case_catastrophic_priority
  BEFORE INSERT OR UPDATE ON cases
  FOR EACH ROW EXECUTE FUNCTION check_case_catastrophic_priority();

CREATE OR REPLACE FUNCTION sync_work_state_on_state_change()
RETURNS TRIGGER AS $$
BEGIN
  IF NEW.state != 'work_in_progress' THEN
    NEW.work_state := NULL;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_sync_work_state_on_state_change
  BEFORE INSERT OR UPDATE ON cases
  FOR EACH ROW EXECUTE FUNCTION sync_work_state_on_state_change();

-- Enable trigram extension for ILIKE '%...%' support
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- FK / equality indexes
CREATE INDEX idx_cases_created_by          ON cases(created_by);
CREATE INDEX idx_cases_project_id          ON cases(project_id);
CREATE INDEX idx_cases_deployment_id       ON cases(deployment_id);
CREATE INDEX idx_cases_deployed_product_id ON cases(deployed_product_id);
CREATE INDEX idx_cases_priority            ON cases(priority);
CREATE INDEX idx_cases_state               ON cases(state);
CREATE INDEX idx_cases_issue_type          ON cases(issue_type);

-- Sort indexes
CREATE INDEX idx_cases_created_at          ON cases(created_at);
CREATE INDEX idx_cases_updated_at          ON cases(updated_at);
CREATE INDEX idx_cases_closed_at           ON cases(closed_at);

-- Composite indexes
CREATE INDEX idx_cases_project_state         ON cases(project_id, state);
CREATE INDEX idx_cases_priority_state        ON cases(priority, state);
CREATE INDEX idx_cases_priority_issue_type   ON cases(priority, issue_type);

-- work_state indexes
CREATE UNIQUE INDEX uidx_cases_one_ongoing_per_engineer
  ON cases (assigned_engineer)
  WHERE work_state = 'ongoing';

CREATE INDEX idx_cases_work_state
  ON cases(work_state)
  WHERE work_state IS NOT NULL;

CREATE INDEX idx_cases_engineer_work_state
  ON cases(assigned_engineer, work_state)
  WHERE work_state IS NOT NULL;

-- Trigram indexes for ILIKE search on subject, number, internal_id
CREATE INDEX idx_cases_subject_trgm  ON cases USING GIN (subject  gin_trgm_ops);
CREATE INDEX idx_cases_number_trgm   ON cases USING GIN (number   gin_trgm_ops);
CREATE INDEX idx_cases_internal_id_trgm  ON cases USING GIN (internal_id  gin_trgm_ops);
