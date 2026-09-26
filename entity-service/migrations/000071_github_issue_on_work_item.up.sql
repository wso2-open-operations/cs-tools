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

-- Move the GitHub issue link from "case" up to work_item.
--
-- WHY IT MOVES. 000067 put github_issue_number on "case" because a case was
-- the only thing that could carry one. Reading the integration that actually
-- runs -- servicenow-integration's issue_servicenow.yml -- shows an issue
-- becomes a SERVICE REQUEST or an INCIDENT, not a case:
--
--     [CR]: / [ECR]: title  -> Service Request, catalog "Generic Requests"
--     Type/ServiceRequest   -> Service Request, catalog "General Requests"
--     Type/Incident         -> Incident
--
-- Service requests live in service_request and incidents in incident; neither
-- is in "case". So the column was on the one table the integration never
-- writes to.
--
-- ON work_item RATHER THAN ON EACH EXTENSION TABLE. Three types need it now
-- and the four outbound triggers currently join "case" only because that was
-- the only linkable type. One column on the parent table lets them join
-- work_item once, instead of growing a branch and an index per type.
--
-- The data moves with it: "case" rows that carry a number keep it.

BEGIN;

ALTER TABLE work_item ADD COLUMN IF NOT EXISTS github_issue_number INTEGER;

UPDATE work_item wi
SET github_issue_number = c.github_issue_number
FROM "case" c
WHERE c.id = wi.id AND c.github_issue_number IS NOT NULL;

-- Partial, like the one it replaces: most work items have no linked issue, and
-- every lookup asks only about the ones that do.
CREATE INDEX IF NOT EXISTS idx_work_item_github_issue_number
    ON work_item (github_issue_number) WHERE github_issue_number IS NOT NULL;

-- THE TRIGGER FUNCTIONS MOVE WITH THE COLUMN, IN THIS SAME TRANSACTION.
-- 000069 defined four trigger functions that read "case".github_issue_number.
-- That migration has already been applied everywhere, so editing it in place
-- would leave every existing database running the old bodies -- which start
-- failing the moment the DROP COLUMN below lands. Replacing them here keeps
-- the column move and the readers of that column atomic: no window exists in
-- which a trigger references a column that is gone.

CREATE OR REPLACE FUNCTION trg_github_outbound_cr()
RETURNS TRIGGER AS $$
DECLARE
    gh RECORD;
    ev  TEXT;
    diff JSONB := '{}'::jsonb;
    cr_number TEXT;
    parent_number TEXT;
    parent_id UUID;
    actor TEXT;
    assignee TEXT;
    col TEXT;
    oldv JSONB;
    newv JSONB := to_jsonb(NEW);
BEGIN
    -- Parent case -> its issue number -> the account's repository. All three
    -- must be present; any one missing means there is nowhere to push.
    SELECT agr.owner, agr.repository, parent_wi.github_issue_number
      INTO gh
      FROM work_item cr_wi
      -- The parent carries the issue. It is a service request far more often
      -- than a case (759 change requests hang off one, against 301 off a
      -- case), so this joins work_item rather than "case".
      JOIN work_item parent_wi ON parent_wi.id = cr_wi.parent_id
      JOIN account_github_repo agr ON agr.account_id = parent_wi.account_id
     WHERE cr_wi.id = NEW.id
       AND parent_wi.github_issue_number IS NOT NULL
       AND agr.is_active;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    IF TG_OP = 'INSERT' THEN
        ev := 'cr_created';
    ELSE
        ev := 'cr_updated';
        oldv := to_jsonb(OLD);
        -- Only the four columns ServiceNow's trigger watched: state, assigned
        -- to, planned start, planned end. Diffing every column would enqueue
        -- pushes for changes the integration being replaced never sent, onto
        -- an issue a customer can read.
        -- start_on / end_on ARE the planned dates -- change_request_repo.go
        -- reads them into PlannedStartOn. An earlier version of this list named
        -- planned_start_date and planned_end_date, which are not columns on
        -- this table: `newv -> col` returns NULL for a missing key rather than
        -- erroring, so both sides always compared equal and a planned-date
        -- change never enqueued anything. ServiceNow watched those two fields,
        -- so a quarter of the CR updates it sent were silently absent here.
        FOR col IN SELECT unnest(ARRAY['state','start_on','end_on']) LOOP
            IF oldv -> col IS DISTINCT FROM newv -> col THEN
                diff := diff || jsonb_build_object(col,
                    jsonb_build_object('from', oldv -> col, 'to', newv -> col));
            END IF;
        END LOOP;
        -- A sync pass that rewrote the row with identical values is not news
        -- worth putting on someone's issue.
        IF diff = '{}'::jsonb THEN
            RETURN NULL;
        END IF;
    END IF;

    -- The client_payload of "servicenow-cr-update", field for field. The
    -- workflow in the target repository reads these names, so they are not
    -- ours to rename.
    SELECT wi.number, wi.updated_by, u.name, p_wi.number, cr_wi.parent_id
      INTO cr_number, actor, assignee, parent_number, parent_id
      FROM work_item wi
      LEFT JOIN "user" u ON u.id = wi.assigned_to_id
      LEFT JOIN work_item cr_wi ON cr_wi.id = wi.id
      LEFT JOIN work_item p_wi  ON p_wi.id = cr_wi.parent_id
     WHERE wi.id = NEW.id;

    INSERT INTO github_outbound_queue (event, work_item_id, owner, repository, issue_number, payload)
    VALUES (ev, NEW.id, gh.owner, gh.repository, gh.github_issue_number,
            -- EXACTLY THE NINE PROPERTIES ServiceNow sent, no more: GitHub
            -- rejects a client_payload with more than ten, and an eleventh
            -- here cost every CR dispatch a 422. servicenow-cr-update carries
            -- no sn_user -- only servicenow-case-update and -note do -- and
            -- the action is decided here rather than shipped as a diff.
            jsonb_build_object(
                'github_issue_number', gh.github_issue_number,
                'cr_number',     cr_number,
                'cr_sys_id',     NEW.id,
                'cr_state',      NEW.state,
                'assigned_to',   assignee,
                'case_sys_id',   parent_id,
                'planned_start', NEW.start_on,
                'planned_end',   NEW.end_on,
                'action',        CASE
                                     WHEN TG_OP = 'INSERT'        THEN 'created'
                                     WHEN diff ? 'state'          THEN 'state_changed'
                                     ELSE 'dates_updated'
                                 END));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION trg_github_outbound_assignment()
RETURNS TRIGGER AS $$
DECLARE
    gh RECORD;
    to_name TEXT;
BEGIN
    IF NEW.type NOT IN ('CHANGE_REQUEST', 'CASE', 'SERVICE_REQUEST', 'INCIDENT')
       OR OLD.assigned_to_id IS NOT DISTINCT FROM NEW.assigned_to_id THEN
        RETURN NULL;
    END IF;

    SELECT agr.owner, agr.repository, linked_wi.github_issue_number
      INTO gh
      FROM work_item linked_wi
      JOIN account_github_repo agr ON agr.account_id = linked_wi.account_id
     -- A change request reaches its issue through its parent; anything else
     -- carries the issue number itself.
     WHERE linked_wi.id = CASE WHEN NEW.type = 'CHANGE_REQUEST' THEN NEW.parent_id ELSE NEW.id END
       AND linked_wi.github_issue_number IS NOT NULL
       AND agr.is_active;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT COALESCE(NULLIF(TRIM(u.name), ''),
                    NULLIF(TRIM(COALESCE(u.first_name, '') || ' ' || COALESCE(u.last_name, '')), ''))
      INTO to_name FROM "user" u WHERE u.id = NEW.assigned_to_id;

    IF NEW.type <> 'CHANGE_REQUEST' THEN
        -- UNASSIGNMENT IS NOT AN EVENT. ServiceNow's flow computed
        --     action = isClosed ? 'closed' : (isAssigned ? 'assigned' : '')
        -- so clearing the assignee produced no action and dispatched nothing.
        -- Sending action='assigned' with a null assigned_to would have the
        -- workflow post "Assigned to:" with nobody after it.
        IF NEW.assigned_to_id IS NULL THEN
            RETURN NULL;
        END IF;

        -- client_payload of "servicenow-case-update", action=assigned.
        INSERT INTO github_outbound_queue (event, work_item_id, owner, repository, issue_number, payload)
        VALUES ('case_assigned', NEW.id, gh.owner, gh.repository, gh.github_issue_number,
                jsonb_build_object(
                    'action',              'assigned',
                    'github_issue_number', gh.github_issue_number,
                    'case_number',         NEW.number,
                    'case_sys_id',         NEW.id,
                    'resolution_notes',    NULL,
                    'assigned_to',         to_name,
                    'sn_user',             NEW.updated_by));
    ELSE
        -- A change request's assignment travels on "servicenow-cr-update".
        INSERT INTO github_outbound_queue (event, work_item_id, owner, repository, issue_number, payload)
        VALUES ('cr_updated', NEW.id, gh.owner, gh.repository, gh.github_issue_number,
                jsonb_build_object(
                    'github_issue_number', gh.github_issue_number,
                    'cr_number',     NEW.number,
                    'cr_sys_id',     NEW.id,
                    'cr_state',      (SELECT state FROM change_request WHERE id = NEW.id),
                    'assigned_to',   to_name,
                    'case_sys_id',   NEW.parent_id,
                    'planned_start', (SELECT start_on FROM change_request WHERE id = NEW.id),
                    'planned_end',   (SELECT end_on   FROM change_request WHERE id = NEW.id),
                    'action',        'state_changed'));
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION trg_github_outbound_case()
RETURNS TRIGGER AS $$
DECLARE
    gh RECORD;
    case_number TEXT;
    actor TEXT;
    assignee TEXT;
BEGIN
    IF NEW.state IS NOT DISTINCT FROM OLD.state OR NEW.state <> 'CLOSED' THEN
        RETURN NULL;
    END IF;

    -- The issue number is on work_item now, not on this table, so the same
    -- function serves "case", service_request and anything else that closes.
    SELECT agr.owner, agr.repository, linked_wi.github_issue_number
      INTO gh
      FROM work_item linked_wi
      JOIN account_github_repo agr ON agr.account_id = linked_wi.account_id
     WHERE linked_wi.id = NEW.id
       AND linked_wi.github_issue_number IS NOT NULL
       AND agr.is_active;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT wi.number, wi.updated_by,
           COALESCE(NULLIF(TRIM(u.name), ''),
                    NULLIF(TRIM(COALESCE(u.first_name,'')||' '||COALESCE(u.last_name,'')), ''))
      INTO case_number, actor, assignee
      FROM work_item wi LEFT JOIN "user" u ON u.id = wi.assigned_to_id
     WHERE wi.id = NEW.id;

    -- client_payload of "servicenow-case-update", action=closed.
    INSERT INTO github_outbound_queue (event, work_item_id, owner, repository, issue_number, payload)
    VALUES ('case_closed', NEW.id, gh.owner, gh.repository, gh.github_issue_number,
            jsonb_build_object(
                'action',              'closed',
                'github_issue_number', gh.github_issue_number,
                'case_number',         case_number,
                'case_sys_id',         NEW.id,
                'resolution_notes',    NEW.close_notes,
                'assigned_to',         assignee,
                'sn_user',             actor));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION trg_github_outbound_comment()
RETURNS TRIGGER AS $$
DECLARE
    gh RECORD;
BEGIN
    IF NEW.type IS DISTINCT FROM 'COMMENT' THEN
        RETURN NULL;
    END IF;

    SELECT agr.owner, agr.repository, linked_wi.github_issue_number
      INTO gh
      FROM work_item linked_wi
      JOIN account_github_repo agr ON agr.account_id = linked_wi.account_id
     WHERE linked_wi.id = NEW.work_item_id
       AND linked_wi.github_issue_number IS NOT NULL
       AND agr.is_active;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    INSERT INTO github_outbound_queue (event, work_item_id, owner, repository, issue_number, payload)
    VALUES ('comment_added', NEW.work_item_id, gh.owner, gh.repository, gh.github_issue_number,
            -- client_payload of "servicenow-note". note_text goes RAW: the
            -- workflow strips ServiceNow's [code] markers and HTML itself, and
            -- doing it here would change what it receives.
            jsonb_build_object(
                'issue_number', gh.github_issue_number,
                'note_text',    NEW.content,
                'note_type',    NEW.type,
                'case_number',  (SELECT number FROM work_item WHERE id = NEW.work_item_id),
                'case_sys_id',  NEW.work_item_id,
                'sn_user',      NEW.created_by));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS service_request_github_outbound ON service_request;
CREATE TRIGGER service_request_github_outbound
    AFTER UPDATE OF state ON service_request
    FOR EACH ROW EXECUTE FUNCTION trg_github_outbound_case();

DROP INDEX IF EXISTS idx_case_github_issue_number;
ALTER TABLE "case" DROP COLUMN IF EXISTS github_issue_number;

COMMIT;
