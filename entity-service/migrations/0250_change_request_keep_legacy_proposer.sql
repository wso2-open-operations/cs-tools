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

-- Keeps the proposer of a customer's time that has no record (migration 0249) when another writer
-- is about to replace them as the change's last writer.
--
-- A time stored before 0249, or by a writer that does not record its proposer (an older build of
-- this service, the sync), has customer_proposed_for NULL, and for it the proposer is read from the
-- change's last writer (work_item.updated_by) when that writer is a REGISTERED PORTAL_USER contact
-- of the project (proposerFromWriter, change_request_customer_proposal.go). Any later write by
-- somebody else (a staff work note, the hold, a whole-form resend, GitHub, the sync) replaced that
-- writer, and the waiting proposal read "nobody is recorded" from then on.
--
-- The trigger records what that rule says just before it stops saying it: when work_item.updated_by
-- of a change request changes, and the change request is in Customer Approval with an unanswered,
-- finite stored time that differs from the planned start and has no record, and the writer being
-- replaced is a registered PORTAL_USER contact of the project with a "user" row, it writes the
-- record the proposal itself would have written: that user (the earliest "user" row with the
-- address, as the read model names them), the stored time, and when they wrote (OLD.updated_on, the
-- time the read model already showed). It records nothing where the rule names nobody. The record is
-- then read like any other (proposerFromRecord): it counts while it is about the stored time and the
-- contact is still registered.
--
-- It freezes whatever the rule names at that moment, a wrong name included. Every writer in this
-- service stamps work_item.updated_by before it writes the stored time, so the rule's name is the
-- proposer's. A writer that changes customer_updated_on directly while a contact is the last writer
-- (the sync, a manual fix, an import) can make the rule name that contact for a time they did not
-- propose; the old rule showed that until the next save, and this trigger keeps it. Such writers
-- must stamp work_item.updated_by first, as this service does, and must update work_item before
-- change_request in one transaction: a writer that locks change_request first can deadlock with a
-- staff save that this trigger turns into a change_request write (one side is aborted, 40P01).
--
-- It covers every writer from the moment it is applied, which is why it is a trigger and not a
-- one-time update: this build, GitHub, the sync, and an older build that is still running. One gap
-- while an older build runs: once a staff save has recorded a waiting proposal, a later proposal the
-- older build stores on the same change carries no record, is not about the recorded time, and reads
-- "nobody is recorded" (staff can propose a different time). Apply it with 0249 and deploy the build
-- that records proposers straight after.
--
-- Its write to change_request touches only the three record columns, so it moves no state, no
-- planned date and no stored time: the outbox row it leaves (0051) is acknowledged by the change
-- request notices as no turn, and the GitHub queue (0112) ignores it. It runs as the writer, so a
-- role that writes work_item rows of change requests needs SELECT on "user", project_contact,
-- project_contact_group, project_group_role and project_role, and UPDATE on change_request: without
-- them that role's writes of a change request fail.
--
-- TEMPORARY. Once the build that records proposers is deployed and nothing writes
-- customer_updated_on directly (the sync can, while it runs), nothing stores a time without a
-- record, so the set it protects only shrinks. Drop it, in a new migration (DROP TRIGGER IF EXISTS
-- work_item_keep_legacy_customer_proposer ON work_item; DROP FUNCTION IF EXISTS
-- trg_work_item_keep_legacy_customer_proposer();), once this reads 0 on every environment. Run it as
-- a superuser, or after SELECT set_config('app.is_internal', 'true', false): change_request is FORCE
-- ROW LEVEL SECURITY and otherwise reads 0. It counts the rows the trigger would act on:
--
--   SELECT count(*)
--     FROM change_request cr
--     JOIN work_item wi ON wi.id = cr.id
--    WHERE cr.state::text = 'CUSTOMER_APPROVAL'
--      AND cr.customer_proposed_for IS NULL
--      AND cr.customer_updated_on IS NOT NULL
--      AND isfinite(cr.customer_updated_on)
--      AND cr.customer_updated_date_confirmation IS NULL
--      AND cr.customer_updated_on IS DISTINCT FROM cr.start_on
--      AND EXISTS (SELECT 1 FROM "user" u
--                   WHERE LOWER(u.email) = LOWER(btrim(wi.updated_by, E' \t\n\r\f' || chr(11))))
--      AND EXISTS (SELECT 1
--                    FROM project_contact pc
--                    JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
--                    JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
--                    JOIN project_role pr ON pr.id = pgr.project_role_id
--                   WHERE pc.project_id = wi.project_id
--                     AND LOWER(pc.email) = LOWER(btrim(wi.updated_by, E' \t\n\r\f' || chr(11)))
--                     AND pc.state = 'REGISTERED'::project_contact_state_enum
--                     AND pr.role = 'PORTAL_USER'::project_role_enum);
--
-- Until then it carries a second copy, in SQL, of the registered-contact test
-- (callerIsRegisteredPortalContact): change the two together.
--
-- Safe to run again: the function and the trigger replace themselves (CREATE OR REPLACE TRIGGER, so
-- a re-run takes no ACCESS EXCLUSIVE lock). Creating the trigger locks work_item against writes for
-- that statement, so the lock wait is bounded as 0207's is. A run that stops after the function and
-- before the trigger leaves a function nothing calls, and the next run finishes it. Needs migration
-- 0249: it refuses to run without 0249's columns, which the function names (a function without them
-- would fail every write of a change request that changes its writer).
SET lock_timeout = '5s';

DO $$
BEGIN
    -- 'change_request'::regclass resolves the table the way the function's UPDATE will.
    IF (SELECT count(*) FROM pg_attribute
         WHERE attrelid = 'change_request'::regclass AND NOT attisdropped
           AND attname IN ('customer_proposed_by_user_id', 'customer_proposed_for', 'customer_proposed_on')) <> 3 THEN
        RAISE EXCEPTION 'migration 0250 needs migration 0249 (change_request.customer_proposed_by_user_id, customer_proposed_for, customer_proposed_on): apply 0249 first';
    END IF;
END
$$;

CREATE OR REPLACE FUNCTION trg_work_item_keep_legacy_customer_proposer() RETURNS trigger AS $$
DECLARE
    -- Trimmed of the ASCII whitespace Go's strings.TrimSpace trims for the rule (TrimSpace also
    -- trims Unicode spaces: a writer padded with one is not recorded).
    prior TEXT := NULLIF(btrim(OLD.updated_by, E' \t\n\r\f' || chr(11)), '');
BEGIN
    IF prior IS NULL OR OLD.project_id IS NULL THEN
        RETURN NULL;
    END IF;
    -- state::text: the comparison reads the same whether the column is this schema's enum or the
    -- sync tool's own shape of it (as 0249 does).
    UPDATE change_request cr
       SET customer_proposed_by_user_id = (
               SELECT u.id FROM "user" u
                WHERE LOWER(u.email) = LOWER(prior)
                ORDER BY u.created_on ASC, u.id ASC
                LIMIT 1),
           customer_proposed_for = cr.customer_updated_on,
           customer_proposed_on  = OLD.updated_on
     WHERE cr.id = OLD.id
       AND cr.customer_proposed_for IS NULL
       AND cr.state::text = 'CUSTOMER_APPROVAL'
       AND cr.customer_updated_on IS NOT NULL
       AND isfinite(cr.customer_updated_on)
       AND cr.customer_updated_date_confirmation IS NULL
       AND cr.customer_updated_on IS DISTINCT FROM cr.start_on
       AND EXISTS (SELECT 1 FROM "user" u WHERE LOWER(u.email) = LOWER(prior))
       -- The test of the customer's own answer (callerIsRegisteredPortalContact), on the
       -- change's project as it stood before this write.
       AND EXISTS (
               SELECT 1
                 FROM project_contact pc
                 JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
                 JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
                 JOIN project_role pr ON pr.id = pgr.project_role_id
                WHERE pc.project_id = OLD.project_id
                  AND LOWER(pc.email) = LOWER(prior)
                  AND pc.state = 'REGISTERED'::project_contact_state_enum
                  AND pr.role = 'PORTAL_USER'::project_role_enum);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER work_item_keep_legacy_customer_proposer
    AFTER UPDATE OF updated_by ON work_item
    FOR EACH ROW
    WHEN (OLD.type::text = 'CHANGE_REQUEST' AND OLD.updated_by IS DISTINCT FROM NEW.updated_by)
    EXECUTE FUNCTION trg_work_item_keep_legacy_customer_proposer();

RESET lock_timeout;
