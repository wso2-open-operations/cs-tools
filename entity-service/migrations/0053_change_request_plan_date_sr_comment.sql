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

-- Records a plan-start-date change as a comment on the change request's parent.
--
-- Port of the ServiceNow flow "[CR] Fields Changes Comments on the SR":
--
--   trigger  change_request updated, State is Customer Approval,
--            Customer Updated changes
--   loop     For Each changed field
--   gate     If field name is u_customer_updated
--   action   Update Task Record
--              Record : Change Request Record > Parent
--              Table  : task
--              Field  : Additional comments
--              Value  : "Plan start date change update from <prev> to <current>"
--
-- The loop and the gate exist only because Flow Designer hands you every
-- changed field and makes you find the one you meant. A trigger is told
-- directly, so both collapse into the WHERE below.
--
-- TABLE MAPPING. ServiceNow's `task` is the base table change_request extends;
-- work_item is its equivalent here, and `parent` is work_item.parent_id. So the
-- comment lands on the parent work item -- the service request the change was
-- raised under -- which is exactly where the original puts it.
--
-- TYPE = COMMENT, not WORK_NOTE: "Additional comments" is ServiceNow's
-- customer-visible field; work notes are the internal one.
--
-- AFTER, not BEFORE: this writes a different row than the one being updated,
-- so there is nothing to modify in place and no reason to run before the write
-- is settled.
--
-- A change request with no parent records nothing. That is a real state -- not
-- every CR is raised under a service request -- and the original's Update
-- Record would have had nothing to target either.
-- Wrapped in a transaction deliberately, so a psql -f run applying this file
-- alone can't leave the table with no trigger at all mid-way through.
BEGIN;

CREATE OR REPLACE FUNCTION trg_change_request_plan_date_comment() RETURNS trigger AS $$
DECLARE
    parent UUID;
    actor  TEXT;
BEGIN
    IF NEW.state IS DISTINCT FROM 'CUSTOMER_APPROVAL'
       OR NEW.customer_updated_on IS NOT DISTINCT FROM OLD.customer_updated_on THEN
        RETURN NULL;
    END IF;

    SELECT wi.parent_id, wi.updated_by INTO parent, actor
    FROM work_item wi WHERE wi.id = NEW.id;

    IF parent IS NULL THEN
        RETURN NULL;
    END IF;

    INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
    VALUES (
        gen_random_uuid(), NOW(), COALESCE(actor, 'system'), 'COMMENT', parent,
        'Plan start date change update from ' ||
        COALESCE(to_char(OLD.customer_updated_on, 'YYYY-MM-DD HH24:MI'), '(not set)') ||
        ' to ' ||
        COALESCE(to_char(NEW.customer_updated_on, 'YYYY-MM-DD HH24:MI'), '(not set)')
    );

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS change_request_plan_date_comment ON change_request;
CREATE TRIGGER change_request_plan_date_comment
    AFTER UPDATE ON change_request
    FOR EACH ROW EXECUTE FUNCTION trg_change_request_plan_date_comment();

COMMIT;
