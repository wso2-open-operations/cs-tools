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

-- Work waiting to be pushed to GitHub: the outbound half of the change-request
-- sync, replacing ServiceNow's [GitHub Integration] flows.
--
-- A SEPARATE QUEUE FROM THE NOTIFICATION OUTBOX, ON PURPOSE. That one claims a
-- row at read time and never retries, because a lost email beats a duplicate
-- one. This calls someone else's service, where the trade-off inverts: GitHub
-- returns 502s and rate limits that succeed on the next attempt, so a row here
-- is retried with backoff and only abandoned after a bounded number of tries.
--
-- THE GATE IS THE PARENT CASE'S ISSUE NUMBER, matching ServiceNow's own flow:
-- "Change Request Created where Parent is not empty", then look up the case by
-- that parent and read the issue number off it. A change request has no issue
-- of its own -- it reaches GitHub only through the case it belongs to, and one
-- with no parent, or whose parent is not linked, is not our business.
CREATE TABLE IF NOT EXISTS github_outbound_queue (
    id BIGSERIAL PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- What happened, which decides what gets pushed.
    event VARCHAR(40) NOT NULL,
    -- The record this is about: a change request for the CR flows, the case
    -- itself for the case closure and assignment ones. Everything we push is a
    -- comment on the linked issue.
    --
    -- work_item, not change_request: both kinds of row live there, and a case
    -- id is not present in the change_request extension table -- pointing this
    -- at change_request made every case event fail its foreign key at enqueue
    -- time, inside the trigger, which takes the caller's UPDATE down with it.
    work_item_id UUID NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    -- Resolved AT ENQUEUE TIME rather than on delivery: if the case is
    -- re-linked afterwards, this row still belongs to the issue the change was
    -- actually about.
    owner VARCHAR(100) NOT NULL,
    repository VARCHAR(200) NOT NULL,
    issue_number INTEGER NOT NULL,
    -- Event-specific detail: the comment body, the columns that changed.
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Retry state.
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Bounded, and never the raw upstream body: an error from GitHub can carry
    -- content we are not allowed to store or log.
    last_error VARCHAR(500),
    delivered_on TIMESTAMPTZ,

    CONSTRAINT chk_github_outbound_status
        CHECK (status IN ('PENDING', 'DELIVERED', 'FAILED'))
);

-- The worker's only query: what is due now. Partial, so the index stays the
-- size of the backlog rather than the size of the history.
CREATE INDEX IF NOT EXISTS idx_github_outbound_due
    ON github_outbound_queue (next_attempt_on)
    WHERE status = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_github_outbound_cr
    ON github_outbound_queue (work_item_id);

-- Enqueue a change-request event, but only when there is an issue to push to.
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
    SELECT agr.owner, agr.repository, c.github_issue_number
      INTO gh
      FROM work_item cr_wi
      JOIN work_item case_wi ON case_wi.id = cr_wi.parent_id
      JOIN "case" c          ON c.id = case_wi.id
      JOIN account_github_repo agr ON agr.account_id = case_wi.account_id
     WHERE cr_wi.id = NEW.id
       AND c.github_issue_number IS NOT NULL
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

DROP TRIGGER IF EXISTS change_request_github_outbound ON change_request;
CREATE TRIGGER change_request_github_outbound
    AFTER INSERT OR UPDATE ON change_request
    FOR EACH ROW EXECUTE FUNCTION trg_github_outbound_cr();

-- Enqueue a comment, resolving the change request it belongs to. A comment on
-- a work item that is not a change request, or on one with no linked issue,
-- enqueues nothing.
-- Assignment, for both records that can carry it.
--
-- For a change request this is the fourth field "SN CR Updates -> GitHub"
-- watched. For a case it is the 'assigned' half of "SN Case Updates ->
-- GitHub", which detects the change through the audit log; we have the OLD
-- row, so a direct comparison does the same job without a second query.
--
-- It lives on work_item rather than on either extension table, so one trigger
-- serves both.
--
-- THE NAMES ARE RESOLVED HERE, NOT AT DELIVERY. assigned_to_id is a UUID, and
-- the comment this becomes is posted on an issue a customer reads: "Assigned
-- to: 4f3a...  ->  9b2c..." is noise to them and a leaked internal identifier
-- to us. ServiceNow sent the display name, so we resolve it the same way it
-- resolves owner and repository -- at enqueue time, against the row as it
-- stood. An email address is never the fallback for the same reason.
CREATE OR REPLACE FUNCTION trg_github_outbound_assignment()
RETURNS TRIGGER AS $$
DECLARE
    gh RECORD;
    to_name TEXT;
BEGIN
    IF NEW.type NOT IN ('CHANGE_REQUEST', 'CASE')
       OR OLD.assigned_to_id IS NOT DISTINCT FROM NEW.assigned_to_id THEN
        RETURN NULL;
    END IF;

    SELECT agr.owner, agr.repository, c.github_issue_number
      INTO gh
      FROM work_item case_wi
      JOIN "case" c ON c.id = case_wi.id
      JOIN account_github_repo agr ON agr.account_id = case_wi.account_id
     -- A change request reaches its issue through the parent case; a case IS
     -- the record that carries the issue number.
     WHERE case_wi.id = CASE WHEN NEW.type = 'CASE' THEN NEW.id ELSE NEW.parent_id END
       AND c.github_issue_number IS NOT NULL
       AND agr.is_active;

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT COALESCE(NULLIF(TRIM(u.name), ''),
                    NULLIF(TRIM(COALESCE(u.first_name, '') || ' ' || COALESCE(u.last_name, '')), ''))
      INTO to_name FROM "user" u WHERE u.id = NEW.assigned_to_id;

    IF NEW.type = 'CASE' THEN
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

DROP TRIGGER IF EXISTS work_item_assignment_github_outbound ON work_item;
CREATE TRIGGER work_item_assignment_github_outbound
    AFTER UPDATE OF assigned_to_id ON work_item
    FOR EACH ROW EXECUTE FUNCTION trg_github_outbound_assignment();

-- Case closure, the other half of "SN Case Updates -> GitHub".
--
-- That flow reads the state's DISPLAY value and asks whether it is "closed",
-- deliberately avoiding hardcoded integer codes so one flow works across
-- instances. Our state is already a named enum, so the comparison is direct.
-- Closure carries the resolution notes, which the flow sends alongside
-- action='closed'.
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

    SELECT agr.owner, agr.repository
      INTO gh
      FROM work_item case_wi
      JOIN account_github_repo agr ON agr.account_id = case_wi.account_id
     WHERE case_wi.id = NEW.id
       AND agr.is_active;

    IF NOT FOUND OR NEW.github_issue_number IS NULL THEN
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
    VALUES ('case_closed', NEW.id, gh.owner, gh.repository, NEW.github_issue_number,
            jsonb_build_object(
                'action',              'closed',
                'github_issue_number', NEW.github_issue_number,
                'case_number',         case_number,
                'case_sys_id',         NEW.id,
                'resolution_notes',    NEW.close_notes,
                'assigned_to',         assignee,
                'sn_user',             actor));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS case_github_outbound ON "case";
CREATE TRIGGER case_github_outbound
    AFTER UPDATE OF state ON "case"
    FOR EACH ROW EXECUTE FUNCTION trg_github_outbound_case();

-- Comments sync from the CASE, not the change request.
--
-- ServiceNow's "SN Comment to GitHub" triggers on
-- "Case Updated where (Additional comments changes)" -- the case's own
-- customer-visible journal. A comment on a change request was never sent, and
-- an earlier version of this trigger had it the wrong way round.
--
-- Only COMMENT type: ServiceNow hardcodes note_type to 'additional_comments',
-- which is the customer-visible field. Work notes live in a separate journal
-- and are never dispatched -- they are internal, and a GitHub issue is read
-- outside WSO2.
CREATE OR REPLACE FUNCTION trg_github_outbound_comment()
RETURNS TRIGGER AS $$
DECLARE
    gh RECORD;
BEGIN
    IF NEW.type IS DISTINCT FROM 'COMMENT' THEN
        RETURN NULL;
    END IF;

    SELECT agr.owner, agr.repository, c.github_issue_number
      INTO gh
      FROM "case" c
      JOIN work_item case_wi ON case_wi.id = c.id
      JOIN account_github_repo agr ON agr.account_id = case_wi.account_id
     WHERE c.id = NEW.work_item_id
       AND c.github_issue_number IS NOT NULL
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

DROP TRIGGER IF EXISTS comment_github_outbound ON comment;
CREATE TRIGGER comment_github_outbound
    AFTER INSERT ON comment
    FOR EACH ROW EXECUTE FUNCTION trg_github_outbound_comment();
