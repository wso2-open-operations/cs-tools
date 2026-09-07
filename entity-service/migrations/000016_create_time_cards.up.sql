BEGIN;

-- Native time-card entity. Until now time cards were served only through the
-- ServiceNow passthrough (sn_time_card_service.go); this is their Postgres home.
-- A time card logs effort against a case, in five effort buckets, moving through
-- submitted -> approved/rejected. is_billable is the per-card flag the "TimeCard
-- billable/non-billable" case-type rules key off.
--
-- NOTE ON FK TYPES: users.id and projects.id are TEXT (see 000001, 000003), so
-- the user/project references here are TEXT. cases.id is UUID, so case_id is UUID.
-- (The cases table declares created_by/project_id as UUID against those TEXT PKs,
-- which is a pre-existing inconsistency; this table references each PK with its
-- actual type so the constraints apply.)
CREATE TYPE time_card_state_enum AS ENUM (
  'pending', 'submitted', 'approved', 'rejected', 'processed', 'recalled'
);

CREATE TABLE time_cards (
  id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  case_id                    UUID NOT NULL REFERENCES cases(id),
  project_id                 TEXT NOT NULL REFERENCES projects(id),
  submitter                  TEXT NOT NULL REFERENCES users(id),
  work_date                  DATE NOT NULL,
  state                      time_card_state_enum NOT NULL DEFAULT 'submitted',
  is_billable                BOOLEAN NOT NULL DEFAULT FALSE,
  issue_complexity           TEXT NULL,
  work_log_comment           TEXT NULL,
  lead_comment               TEXT NULL,
  rejection_reason           TEXT NULL,
  -- effort buckets, in minutes; non-negative
  time_analyzing             INTEGER NOT NULL DEFAULT 0 CHECK (time_analyzing >= 0),
  time_setting_up            INTEGER NOT NULL DEFAULT 0 CHECK (time_setting_up >= 0),
  time_reproducing_debugging INTEGER NOT NULL DEFAULT 0 CHECK (time_reproducing_debugging >= 0),
  time_providing_solution    INTEGER NOT NULL DEFAULT 0 CHECK (time_providing_solution >= 0),
  time_patching              INTEGER NOT NULL DEFAULT 0 CHECK (time_patching >= 0),
  approved_by                TEXT NULL REFERENCES users(id),
  created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- rejection_reason only makes sense on a rejected card
  CONSTRAINT chk_time_card_rejection
    CHECK (rejection_reason IS NULL OR state = 'rejected'),
  -- approved_by only set once approved
  CONSTRAINT chk_time_card_approved_by
    CHECK (approved_by IS NULL OR state = 'approved')
);

-- Eligible approvers (SN's approver_list): many candidate approvers per card,
-- distinct from approved_by (the one who actually approved).
CREATE TABLE time_card_approvers (
  time_card_id UUID NOT NULL REFERENCES time_cards(id) ON DELETE CASCADE,
  approver_id  TEXT NOT NULL REFERENCES users(id),
  PRIMARY KEY (time_card_id, approver_id)
);

CREATE INDEX idx_time_cards_case_id        ON time_cards(case_id);
CREATE INDEX idx_time_cards_project_id     ON time_cards(project_id);
CREATE INDEX idx_time_cards_submitter      ON time_cards(submitter);
CREATE INDEX idx_time_cards_state          ON time_cards(state);
CREATE INDEX idx_time_cards_work_date      ON time_cards(work_date);
CREATE INDEX idx_time_cards_approved_by    ON time_cards(approved_by) WHERE approved_by IS NOT NULL;
CREATE INDEX idx_time_card_approvers_appr  ON time_card_approvers(approver_id);

COMMIT;
