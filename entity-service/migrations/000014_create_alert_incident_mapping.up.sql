-- alert_incident_mapping records which monitoring alerts contributed to
-- which CSM incident: multiple alerts (e.g. a firing event and a later
-- resolved event for the same underlying condition) are grouped onto one
-- incident rather than each creating its own. CSM-native data only — no
-- ServiceNow equivalent, always backed by Postgres regardless of
-- DATA_SOURCE (same reasoning as sla_clocks/scheduled_task_run).
CREATE TABLE IF NOT EXISTS alert_incident_mapping (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  alert_number      TEXT NOT NULL UNIQUE,
  source            TEXT NOT NULL,
  unique_identifier TEXT,
  service           TEXT,
  metric_name       TEXT,
  alert_status      TEXT NOT NULL,
  incident_id       TEXT NOT NULL,
  incident_number   TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Correlation lookup: given the alert source and its correlation key
-- (unique_identifier), find every alert already grouped onto an incident
-- for that same underlying condition.
CREATE INDEX IF NOT EXISTS idx_alert_incident_mapping_correlation
  ON alert_incident_mapping (source, unique_identifier);

-- Reverse lookup: given an incident, find every alert that contributed to it.
CREATE INDEX IF NOT EXISTS idx_alert_incident_mapping_incident_id
  ON alert_incident_mapping (incident_id);
