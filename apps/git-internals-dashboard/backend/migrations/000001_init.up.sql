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

-- PRIVACY: no title, no labels, no assignees, no opened_by. Ever.
CREATE TABLE issues (
  id                 serial PRIMARY KEY,
  repository_id      integer NOT NULL REFERENCES repositories(id),
  github_number      integer NOT NULL,
  github_node_id     text,
  state              issue_state NOT NULL,
  html_url           text,
  priority           text,          -- derived from Priority/* label at ingest; raw labels discarded
  current_status     text,
  current_status_at  timestamptz,
  github_created_at  timestamptz,
  github_closed_at   timestamptz,
  github_updated_at  timestamptz,
  first_seen_at      timestamptz NOT NULL DEFAULT now(),
  last_synced_at     timestamptz,
  UNIQUE (repository_id, github_number)
);
CREATE INDEX issues_github_node_id_idx  ON issues (github_node_id);
CREATE INDEX issues_state_idx           ON issues (state);
CREATE INDEX issues_repo_state_idx      ON issues (repository_id, state);
CREATE INDEX issues_priority_idx        ON issues (priority);
CREATE INDEX issues_current_status_idx  ON issues (current_status);
CREATE INDEX issues_github_created_idx  ON issues (github_created_at);

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
