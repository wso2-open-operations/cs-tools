-- Computed query-hour position per project, owned by entity-service.
--
-- Deliberately NOT columns on `project`: that table belongs to
-- csm-sync-service, which mirrors ServiceNow into it. `consumed_duration`,
-- `total_query_duration` and `remaining_query_duration` there are all SN's
-- values, and anything Go wrote into them would be overwritten by the next
-- sync run. So the port keeps its own derived row and never writes `project`.
--
-- project_id is a plain UUID, not a FK to project(id), for the same reason
-- sla_clocks.case_id is a plain TEXT: the projects table is owned by another
-- service's migration series, and a cross-series FK would couple entity-service's
-- migration order to it.
CREATE TABLE IF NOT EXISTS project_query_hours (
  project_id            UUID PRIMARY KEY,
  entitlement_minutes   INTEGER NOT NULL DEFAULT 0,
  consumed_minutes      INTEGER NOT NULL DEFAULT 0,
  billable_minutes      INTEGER NOT NULL DEFAULT 0,
  non_billable_minutes  INTEGER NOT NULL DEFAULT 0,
  -- 0 = under 75%, 1 = >=75%, 2 = >=90%, 3 = >=100%. Mirrors ServiceNow's
  -- u_query_hour_state values so a cutover comparison is a straight equality
  -- check. Unlike SN's `Set Project Query Hour State`, this is recomputed in
  -- BOTH directions: a recalled or corrected time card lowers it again.
  query_hour_state      SMALLINT NOT NULL DEFAULT 0,
  -- The last state successfully pushed to Choreo, and when. NULL means never
  -- pushed. Kept separate from query_hour_state so a failed push retries on
  -- the next recompute instead of being silently lost.
  last_pushed_state     SMALLINT,
  last_pushed_at        TIMESTAMPTZ,
  computed_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT project_query_hours_state_range CHECK (query_hour_state BETWEEN 0 AND 3),
  CONSTRAINT project_query_hours_pushed_state_range
    CHECK (last_pushed_state IS NULL OR last_pushed_state BETWEEN 0 AND 3)
);

-- The scheduled sweep asks "which projects are stalest?", so it orders by this.
CREATE INDEX IF NOT EXISTS idx_project_query_hours_computed_at
  ON project_query_hours(computed_at);

-- "Which projects still owe Choreo a push?" — the retry path.
CREATE INDEX IF NOT EXISTS idx_project_query_hours_push_pending
  ON project_query_hours(project_id)
  WHERE last_pushed_state IS DISTINCT FROM query_hour_state;
