CREATE TYPE issue_state AS ENUM ('OPEN', 'CLOSED');

CREATE TABLE projects (
  id                 serial PRIMARY KEY,
  github_project_id  text NOT NULL UNIQUE,
  title              text,
  enabled            boolean NOT NULL DEFAULT true,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE repositories (
  id              serial PRIMARY KEY,
  owner           text NOT NULL,
  name            text NOT NULL,
  github_node_id  text,
  html_url        text,
  sla_project_id  integer REFERENCES projects(id),
  issue_query     text NOT NULL,
  enabled         boolean NOT NULL DEFAULT true,
  last_synced_at  timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (owner, name)
);

-- The only issue-body-derived columns persisted are title, abt_team, and
-- opened_by, the last restricted to a @wso2.com address by the CHECK below.
-- Nothing else from the issue body — no labels, no assignees — is ever
-- persisted.
CREATE TABLE issues (
  id                 serial PRIMARY KEY,
  repository_id      integer NOT NULL REFERENCES repositories(id),
  github_number      integer NOT NULL,
  github_node_id     text,
  state              issue_state NOT NULL,
  html_url           text,
  title              text,
  abt_team           text,
  opened_by          text,
  priority           text,          -- derived from Priority/* label at ingest; raw labels discarded
  current_status     text,
  current_status_at  timestamptz,
  github_created_at  timestamptz,
  github_closed_at   timestamptz,
  github_updated_at  timestamptz,
  first_seen_at      timestamptz NOT NULL DEFAULT now(),
  last_synced_at     timestamptz,
  UNIQUE (repository_id, github_number),
  CONSTRAINT issues_opened_by_wso2_chk
    CHECK (opened_by IS NULL OR opened_by ~ '^[a-z0-9._%+-]+@wso2\.com$')
);
CREATE INDEX issues_github_node_id_idx  ON issues (github_node_id);
CREATE INDEX issues_state_idx           ON issues (state);
CREATE INDEX issues_repo_state_idx      ON issues (repository_id, state);
CREATE INDEX issues_priority_idx        ON issues (priority);
CREATE INDEX issues_current_status_idx  ON issues (current_status);
CREATE INDEX issues_github_created_idx  ON issues (github_created_at);
CREATE INDEX issues_github_updated_idx  ON issues (github_updated_at);
CREATE INDEX issues_abt_team_idx        ON issues (abt_team);

-- PRIVACY: no actor, no raw payload.
CREATE TABLE issue_status_events (
  id               serial PRIMARY KEY,
  issue_id         integer NOT NULL REFERENCES issues(id),
  project_id       integer NOT NULL REFERENCES projects(id),
  previous_status  text,
  status           text,
  occurred_at      timestamptz NOT NULL,
  source           text NOT NULL,
  dedupe_key       text NOT NULL UNIQUE,
  ingested_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX issue_status_events_issue_occurred_idx ON issue_status_events (issue_id, occurred_at);
CREATE INDEX issue_status_events_project_idx        ON issue_status_events (project_id);

CREATE TABLE issue_sla (
  issue_id          integer PRIMARY KEY REFERENCES issues(id),
  priority          text,
  budget_hours      double precision,
  consumed_hours    double precision,
  remaining_hours   double precision,
  pct_consumed      double precision,
  sla_state         text,
  sla_running       boolean,
  -- A sticky "was this ever violated" signal, independent of sla_state
  -- (which reports TERMINAL once an issue resolves, masking a real past
  -- VIOLATED). Every write path ORs its new value in rather than
  -- overwriting, so it can only ever go false -> true.
  breached_ever     boolean NOT NULL DEFAULT false,
  computed_at       timestamptz NOT NULL,
  computed_through  timestamptz NOT NULL
);
CREATE INDEX issue_sla_state_idx        ON issue_sla (sla_state);
CREATE INDEX issue_sla_pct_consumed_idx ON issue_sla (pct_consumed);

CREATE TABLE sla_snapshots (
  id               serial PRIMARY KEY,
  snapshot_date    date NOT NULL,
  issue_id         integer NOT NULL REFERENCES issues(id),
  repository_id    integer NOT NULL REFERENCES repositories(id),
  priority         text,
  current_status   text,
  budget_hours     double precision,
  consumed_hours   double precision,
  remaining_hours  double precision,
  pct_consumed     double precision,
  sla_state        text,
  sla_running      boolean,
  UNIQUE (snapshot_date, issue_id)
);
CREATE INDEX sla_snapshots_date_idx       ON sla_snapshots (snapshot_date);
CREATE INDEX sla_snapshots_repo_date_idx  ON sla_snapshots (repository_id, snapshot_date);
CREATE INDEX sla_snapshots_state_date_idx ON sla_snapshots (sla_state, snapshot_date);

CREATE TABLE sync_runs (
  id                serial PRIMARY KEY,
  repository_id     integer REFERENCES repositories(id),
  kind              text,
  since_ts          timestamptz,
  started_at        timestamptz NOT NULL DEFAULT now(),
  finished_at       timestamptz,
  status            text,
  issues_processed  integer DEFAULT 0,
  error             text
);

-- Statuses seen on a board that taxonomy.statuses doesn't recognize (a
-- renamed or newly added column) — surfaced via GET /metrics/overview so
-- they get noticed and classified instead of silently pausing or accruing
-- the SLA clock unnoticed. Replaced wholesale each recompute tick (see
-- internal/jobs.RunTickOnce), not accumulated: a status that's since been
-- added to the taxonomy disappears from here.
CREATE TABLE unknown_statuses (
  status            text PRIMARY KEY,
  occurrence_count  integer NOT NULL,
  first_seen_at     timestamptz NOT NULL DEFAULT now(),
  last_seen_at      timestamptz NOT NULL DEFAULT now()
);
