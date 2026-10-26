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

-- Notification rules (SLA SPEC.md R46-R53): admin config saying which
-- threshold-crossing events trigger which action, kept entirely separate
-- from the clock itself (R47: "Handling an event never affects the
-- clock"). entity-service owns this config and the outcome log;
-- csm-notification-service is the one that actually evaluates a rule,
-- resolves recipients and sends -- it holds no SLA clock state of its own
-- (see this change's own plan file, "Single engine").
--
-- definition_id/threshold_percentage are both nullable -- NULL means "every
-- definition" / "any threshold" respectively (R48: "a rule says which
-- events it acts on, by definition and threshold"), so one rule can cover a
-- broad class of crossings without being duplicated per definition.
--
-- recipients is JSONB, not three separate columns or a child table: R49's
-- three recipient kinds (assignee, team, named users/groups) are resolved
-- fresh when the event is handled, never stored as a frozen list, so the
-- column is just the admin's own selection of which kinds/names to resolve
-- -- not a materialized recipient list.
BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS sla_notification_rule (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    definition_id        UUID REFERENCES sla_definition(id),
    threshold_percentage INTEGER,
    channel              VARCHAR(20) NOT NULL,
    -- {"assignee": bool, "team": bool, "users": [...], "groups": [...]} (R49).
    recipients           JSONB NOT NULL,
    -- Required iff channel = 'webhook' (R50).
    webhook_url          TEXT,
    is_active            BOOLEAN NOT NULL DEFAULT TRUE,
    created_on           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_on           TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by           VARCHAR(255) NOT NULL,
    updated_by           VARCHAR(255) NOT NULL,
    CONSTRAINT sla_notification_rule_channel_check
        CHECK (channel IN ('email', 'webhook')),
    CONSTRAINT sla_notification_rule_webhook_url_check
        CHECK (channel <> 'webhook' OR webhook_url IS NOT NULL)
);

COMMENT ON TABLE sla_notification_rule IS
    'Admin-configured action for a threshold-crossing event (R46-R51). Recipients are resolved fresh when the event is handled (R49), never frozen here.';

CREATE INDEX IF NOT EXISTS sla_notification_rule_definition_id_idx
    ON sla_notification_rule (definition_id) WHERE definition_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS sla_notification_action_log (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id              UUID NOT NULL REFERENCES sla_notification_rule(id),
    instance_id          UUID NOT NULL REFERENCES sla_instance(id),
    threshold_percentage INTEGER NOT NULL,
    channel              VARCHAR(20) NOT NULL,
    status               VARCHAR(20) NOT NULL,
    attempted_on         TIMESTAMPTZ NOT NULL,
    error                TEXT,
    retry_count          INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT sla_notification_action_log_status_check
        CHECK (status IN ('sent', 'failed'))
);

COMMENT ON TABLE sla_notification_action_log IS
    'Outcome of each sla_notification_rule firing (R53), written by csm-notification-service against the instance/threshold that triggered it.';

CREATE INDEX IF NOT EXISTS sla_notification_action_log_instance_id_idx
    ON sla_notification_action_log (instance_id);

COMMIT;
