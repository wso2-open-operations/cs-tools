CREATE TABLE IF NOT EXISTS scheduled_task_run (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_name          TEXT NOT NULL,
  period_key         TIMESTAMPTZ NOT NULL,
  attempt_count      INT NOT NULL DEFAULT 1,
  last_error         TEXT,
  next_retry_at      TIMESTAMPTZ,
  first_attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_attempted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  succeeded_at       TIMESTAMPTZ,
  superseded_at      TIMESTAMPTZ,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (task_name, period_key)
);

-- task_name is a plain TEXT, not a foreign key or enum: which sub-crons
-- exist is a registry defined entirely inside operations/csm-scheduled-tasks,
-- not something this service tracks (same reasoning as sla_clocks.clock_type).

-- No status column: status is always derivable from which timestamp is set
-- (succeeded_at / superseded_at / next_retry_at) — same choice sla_clocks
-- makes, and each timestamp is independently useful on its own. See
-- domain.ScheduledTaskRun's doc comment for the derivation.

-- By construction (see the "supersede" step in ScheduledTaskRunRepository.Attempt)
-- there is at most one open (not succeeded, not superseded) row per
-- task_name at any time. This partial index serves both that supersede
-- step's UPDATE and the monitoring GET /scheduled-tasks/attempts?status=failed
-- endpoint.
CREATE INDEX IF NOT EXISTS idx_scheduled_task_run_open
  ON scheduled_task_run(task_name)
  WHERE succeeded_at IS NULL AND superseded_at IS NULL;
