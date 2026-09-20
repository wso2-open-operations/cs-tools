-- Finding 3: a sticky "was this ever violated" signal, independent of
-- sla_state (which reports TERMINAL once an issue resolves, masking a real
-- past VIOLATED). Backfilled from sla_snapshots history so existing breaches
-- aren't lost on day one; every write path afterward ORs its new value in
-- rather than overwriting, so it can only ever go false -> true.
ALTER TABLE issue_sla ADD COLUMN breached_ever boolean NOT NULL DEFAULT false;

UPDATE issue_sla SET breached_ever = true
WHERE EXISTS (
  SELECT 1 FROM sla_snapshots s
  WHERE s.issue_id = issue_sla.issue_id AND s.pct_consumed >= 1.0
);

-- Finding 4: statuses seen on a board that taxonomy.statuses doesn't
-- recognize (a renamed or newly added column) — surfaced via
-- GET /metrics/overview so they get noticed and classified instead of
-- silently pausing or accruing the SLA clock unnoticed. Replaced wholesale
-- each recompute tick (see internal/jobs.RunTickOnce), not accumulated:
-- a status that's since been added to the taxonomy disappears from here.
CREATE TABLE unknown_statuses (
  status            text PRIMARY KEY,
  occurrence_count  integer NOT NULL,
  first_seen_at     timestamptz NOT NULL DEFAULT now(),
  last_seen_at      timestamptz NOT NULL DEFAULT now()
);
