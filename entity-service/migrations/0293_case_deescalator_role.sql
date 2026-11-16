-- A role whose holders may de-escalate any case, in addition to the leads of
-- the case's own ABT team (who already may). An admin grants and removes it
-- from the portal's admin area (Escalation access), so who may de-escalate
-- changes by a grant, not a redeploy.
--
-- ON CONFLICT (name), not (id): the ServiceNow sync seeds this table too, so
-- a deployment may already have a row under a different id. Re-running is
-- safe.

BEGIN;
SET LOCAL lock_timeout = '5s';

INSERT INTO role (id, created_on, updated_on, created_by, updated_by, name, description)
VALUES
    (gen_random_uuid(), now(), now(), 'migration', 'migration',
     'case_deescalator', 'May de-escalate any case, besides the leads of the case''s ABT team')
ON CONFLICT (name) DO NOTHING;

COMMIT;
