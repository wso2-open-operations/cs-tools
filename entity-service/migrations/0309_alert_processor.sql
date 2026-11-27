-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- The alert processor's columns on alert (migration 0306). Safe to run again. The consumer writes alert all the time,
-- so wait at most 5 seconds for its lock.
SET lock_timeout = '5s';

ALTER TABLE alert
    -- NEW until the processor decides; ERROR on a message that could not be normalised. See alert_status_check.
    ADD COLUMN IF NOT EXISTS status             TEXT        NOT NULL DEFAULT 'NEW',
    -- Failed processing attempts, the last error, and when the next attempt is due.
    ADD COLUMN IF NOT EXISTS attempts           INTEGER     NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_error         TEXT,
    ADD COLUMN IF NOT EXISTS next_attempt_on    TIMESTAMPTZ,
    -- The alert mute rule that held a MUTED alert back.
    ADD COLUMN IF NOT EXISTS mute_rule_id       UUID,
    -- In hybrid (shadow) mode: what the processor would have done, and the open incident it would have joined.
    ADD COLUMN IF NOT EXISTS shadow_decision    TEXT,
    ADD COLUMN IF NOT EXISTS shadow_incident_id UUID;

-- Error rows stored before this migration read ERROR too. The processor never takes an error row whatever its status.
UPDATE alert SET status = 'ERROR' WHERE error_reason IS NOT NULL AND status = 'NEW';

DO $$
DECLARE c record;
BEGIN
    FOR c IN SELECT * FROM (VALUES
        ('alert_status_check',
         'CHECK (status IN (''NEW'', ''ERROR'', ''PROCESSED'', ''MUTED'', ''SHADOWED'', ''FAILED''))'),
        -- Only an error row may be ERROR. An error row written by a consumer older than this migration reads NEW,
        -- so the reverse is not enforced.
        ('alert_status_error_check', 'CHECK (status <> ''ERROR'' OR error_reason IS NOT NULL)'),
        ('alert_shadow_decision_check',
         'CHECK (shadow_decision IN (''WOULD_CREATE'', ''WOULD_ATTACH'', ''WOULD_MUTE'', ''WOULD_IGNORE''))'),
        ('alert_attempts_check', 'CHECK (attempts >= 0)')
    ) AS t(name, def) LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_constraint
                        WHERE conrelid = 'alert'::regclass AND conname = c.name) THEN
            EXECUTE format('ALTER TABLE alert ADD CONSTRAINT %I %s', c.name, c.def);
        END IF;
    END LOOP;
END $$;

-- The processor claims the oldest waiting alert of the priority tier first ('high' sorts after 'default'). Same
-- predicate as the claim query in alert_processor_repo.go.
CREATE INDEX IF NOT EXISTS alert_pending_idx ON alert (tier DESC, id) WHERE processed_on IS NULL AND error_reason IS NULL;

-- Not audited: the queue's own working table, written by the consumer and the processor only, and its rows already
-- record who decided what. Skipped on a database without the audit log (migration 0295).
DO $$
BEGIN
    IF to_regprocedure('exclude_from_audit(text, text, text)') IS NOT NULL THEN
        PERFORM exclude_from_audit(current_schema(), 'alert',
            'SRE alert queue table: rows are written by the queue consumer and the alert processor only, and carry their own outcome');
    END IF;
END $$;

COMMENT ON COLUMN alert.status IS
    'NEW: waiting for the alert processor. ERROR: the message could not be normalised (error_reason, error); never processed. '
    'PROCESSED: linked to its incident (incident_id), or a resolving alert with no open incident. MUTED: held back by mute_rule_id. '
    'SHADOWED: hybrid mode recorded shadow_decision and raised nothing. FAILED: gave up after repeated errors (last_error).';
