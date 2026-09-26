-- Put the GitHub issue link back on "case".
--
-- LOSSY BY NATURE, and deliberately not disguised: work_item rows that are not
-- cases cannot be represented on "case" at all, so a service request or
-- incident linked to an issue loses that link on the way down. There is
-- nowhere for it to go.

BEGIN;

ALTER TABLE "case" ADD COLUMN IF NOT EXISTS github_issue_number INTEGER;

UPDATE "case" c
SET github_issue_number = wi.github_issue_number
FROM work_item wi
WHERE wi.id = c.id AND wi.github_issue_number IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_case_github_issue_number
    ON "case" (github_issue_number) WHERE github_issue_number IS NOT NULL;

-- Restore the trigger functions exactly as 000069 defined them, and drop the
-- service_request trigger this migration added: they read
-- "case".github_issue_number, which is put back above, and must be back in
-- place before work_item's own column disappears below.
DROP TRIGGER IF EXISTS service_request_github_outbound ON service_request;

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

DROP INDEX IF EXISTS idx_work_item_github_issue_number;
ALTER TABLE work_item DROP COLUMN IF EXISTS github_issue_number;

COMMIT;
