-- Durable buffer for inbound alerts this service could not (yet) hand off
-- to csm-integration-service as a CSM incident. A row here must survive
-- this service's own restarts and a regional failover -- that durability
-- requirement is the entire reason this table exists in a dedicated
-- database, not an in-process queue.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS alert_buffer (
  -- The DEFAULT is a fallback only: internal/handler.CreateAlert now
  -- generates this id client-side (internal/idgen) and always supplies it
  -- explicitly on INSERT, because it must be embedded as the dedup tag in
  -- the row's own CreateIncidentRequest.Subject before the row is
  -- persisted (see internal/csmclient.DedupTag). Kept as a DEFAULT anyway
  -- so the column still self-populates for any future caller that inserts
  -- without supplying one.
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  payload         JSONB NOT NULL,
  status          TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'delivered', 'escalated', 'failed')),
  retry_count     INT NOT NULL DEFAULT 0,
  last_attempt_at TIMESTAMPTZ,
  last_error      TEXT,
  incident_id     TEXT,
  escalated_at    TIMESTAMPTZ
);

-- Backs the worker's periodic scan for pending rows, oldest first.
CREATE INDEX IF NOT EXISTS idx_alert_buffer_status_received_at
  ON alert_buffer (status, received_at)
  WHERE status = 'pending';
