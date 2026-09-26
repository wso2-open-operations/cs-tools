CREATE TABLE IF NOT EXISTS announcement_requests (
  id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kind                      TEXT NOT NULL CHECK (kind IN ('customer', 'eol')),
  state                     TEXT NOT NULL DEFAULT 'draft'
                              CHECK (state IN ('draft', 'pending_approval', 'approved', 'published')),
  subject                   TEXT NOT NULL DEFAULT '',
  description               TEXT NOT NULL DEFAULT '',
  is_security_announcement  BOOLEAN NOT NULL DEFAULT false,
  audience_definition       JSONB NOT NULL DEFAULT '{}'::jsonb,
  resolved_project_ids      JSONB,
  resolved_project_count    INT,
  dry_run_case_id           TEXT,
  dry_run_on                TIMESTAMPTZ,
  dry_run_by                TEXT,
  created_by                TEXT NOT NULL,
  created_on                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_on                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  submitted_by              TEXT,
  submitted_on              TIMESTAMPTZ,
  approved_by               TEXT,
  approved_on               TIMESTAMPTZ,
  published_by              TEXT,
  published_on              TIMESTAMPTZ
);

-- No FK to cases(id): a published announcement fans out into ServiceNow-backed
-- cases in production, which have no local `cases` row at all, and this row's
-- own lifecycle (draft/pending/approved) exists entirely before any case is
-- created. Same reasoning as sla_clocks.case_id / event_publish_failures.entity_id.

CREATE INDEX IF NOT EXISTS idx_announcement_requests_state ON announcement_requests(state);
CREATE INDEX IF NOT EXISTS idx_announcement_requests_created_by ON announcement_requests(created_by);
