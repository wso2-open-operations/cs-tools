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

-- Records an incident's assignment group changes in event_outbox, so the
-- incident report drainer can publish incident.special_ops_alert when the new
-- group is a Special Ops team -- however the group changed (the specialist
-- handoff today, any other writer later). ServiceNow's "Incident Special Ops
-- Escalation Notification Flow" triggers the same way: assignment group
-- CHANGES TO a Special Ops group.
--
-- The group lives on work_item, not incident, so migration 0181's
-- incident_outbox trigger never sees it. This trigger fires only for INCIDENT
-- rows whose group actually changed, and writes an 'incident' row -- the
-- entity type that drainer already claims -- with changes
-- {"assignment_group_id": {"from", "to"}} and a snapshot of who changed it
-- and when. The drainer acknowledges a change to any other group as a no-op.

SET lock_timeout = '5s';

BEGIN;

CREATE OR REPLACE FUNCTION trg_incident_assignment_group_outbox() RETURNS trigger AS $$
BEGIN
    INSERT INTO event_outbox (entity_type, entity_id, changes, snapshot)
    VALUES (
        'incident',
        NEW.id,
        jsonb_build_object('assignment_group_id',
            jsonb_build_object('from', to_jsonb(OLD.assignment_group_id), 'to', to_jsonb(NEW.assignment_group_id))),
        jsonb_build_object('updated_by', NEW.updated_by, 'updated_on', NEW.updated_on)
    );
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS incident_assignment_group_outbox ON work_item;
CREATE TRIGGER incident_assignment_group_outbox
    AFTER UPDATE OF assignment_group_id ON work_item
    FOR EACH ROW
    WHEN (NEW.type = 'INCIDENT' AND OLD.assignment_group_id IS DISTINCT FROM NEW.assignment_group_id)
    EXECUTE FUNCTION trg_incident_assignment_group_outbox();

COMMIT;

RESET lock_timeout;
