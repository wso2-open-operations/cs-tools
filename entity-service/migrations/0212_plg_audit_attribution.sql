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

-- Attribution for the PLG writes that had none, and the schema change that lets
-- a detach be recorded at all.
--
-- plg_org_platform already carries four actor columns and plg_lifecycle_history
-- records before/after with a reason, so the pairing axes were covered. Four
-- tables were not: a playbook template could be authored, edited and re-tasked,
-- a playbook detached from a pairing, and an organisation's CS owner
-- reassigned, with nothing recording who did any of it. The playbook routes are
-- the admin-only ones, which makes them the least attributable and the most
-- privileged at the same time.
--
-- Every column added here is nullable with no default, so those parts are
-- catalogue-only: Postgres adds the attribute without rewriting the table, and
-- rows written before it keep NULL rather than being credited to somebody who
-- did not do the work. NULL means "written before attribution existed", which is
-- a truthful answer where a backfilled guess would not be.
--
-- UUID REFERENCES "user" (id) ON DELETE SET NULL matches every actor column
-- already in 0129: an offboarded engineer's row is removed without taking the
-- playbook or the organisation with it, and the attribution degrades to unknown
-- rather than blocking the delete.
--
-- THE DETACH IS DIFFERENT IN KIND and is why this migration is not purely
-- additive. A column cannot record a deletion on a row that is gone, so
-- DetachRun stops deleting and starts setting detached_on/detached_by -- and two
-- things have to move with it or behaviour that works today breaks. Both are at
-- the end of this file.

BEGIN;

-- Attribution for the PLG writes that had none.
--
-- plg_org_platform already carries four actor columns and plg_lifecycle_history
-- records before/after with a reason, so the pairing axes were covered. These
-- four tables were not: a playbook template could be authored, edited and
-- re-tasked, and a pairing's CS owner reassigned, with nothing recording who did
-- it. The playbook routes are the admin-only ones, which makes them the least
-- attributable and the most privileged at the same time.
--
-- Every column is nullable with no default, so this is a catalogue-only change:
-- Postgres adds the attribute without rewriting the table, and rows written
-- before it keep NULL rather than being credited to somebody who did not do the
-- work. NULL here means "written before attribution existed", and that is a
-- truthful answer where a backfilled guess would not be.
--
-- UUID REFERENCES "user" (id) ON DELETE SET NULL matches every actor column
-- already in 0129: an offboarded engineer's row is removed without taking the
-- playbook or the organisation with it, and the attribution degrades to unknown
-- rather than blocking the delete.

-- 1. Playbook templates. Authored once, edited by any admin thereafter -- not
--    only by whoever wrote it -- so the two are separate columns rather than one
--    "last touched by".
ALTER TABLE plg_playbook
    ADD COLUMN IF NOT EXISTS authored_by UUID REFERENCES "user" (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS updated_by  UUID REFERENCES "user" (id) ON DELETE SET NULL;

COMMENT ON COLUMN plg_playbook.authored_by IS
    'Who created this playbook template. NULL for rows written before attribution existed.';
COMMENT ON COLUMN plg_playbook.updated_by IS
    'Who last edited it, which need not be the author -- any admin may edit any playbook.';

-- 2. Playbook tasks. PUT /plg/playbooks/{id}/tasks is a soft replace: existing
--    rows are deactivated and re-upserted by (playbook_id, code) rather than
--    deleted, so a row survives its edits and both columns stay meaningful.
ALTER TABLE plg_playbook_task
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES "user" (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS updated_by UUID REFERENCES "user" (id) ON DELETE SET NULL;

COMMENT ON COLUMN plg_playbook_task.created_by IS
    'Who first added this task to its playbook. NULL for rows written before attribution existed.';
COMMENT ON COLUMN plg_playbook_task.updated_by IS
    'Who last changed it, including the deactivate/re-upsert a task replacement performs.';

-- 3. Playbook runs: attaching and detaching a playbook from a pairing.
--
--    added_by already records the attach. The detach did not, and could not:
--    DetachRun is a hard DELETE, so there was no row left to write it on. These
--    two columns only become meaningful once that delete becomes an update --
--    the application change is tracked separately, and until it lands
--    detached_by stays NULL on every row.
--
--    Named detached_* rather than deleted_* because the playbook is being taken
--    off a pairing, not removed from existence: the template is untouched and
--    the same playbook can be attached again.
ALTER TABLE plg_playbook_run
    ADD COLUMN IF NOT EXISTS detached_on TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS detached_by UUID REFERENCES "user" (id) ON DELETE SET NULL;

COMMENT ON COLUMN plg_playbook_run.detached_on IS
    'When this run was taken off the pairing. NULL means still attached.';
COMMENT ON COLUMN plg_playbook_run.detached_by IS
    'Who detached it. Paired with detached_on; both NULL while the run is attached.';

-- Partial, because every read of an attached run filters on exactly this and the
-- detached rows are the minority that accumulate.
CREATE INDEX IF NOT EXISTS idx_plg_playbook_run_attached
    ON plg_playbook_run (org_platform_id)
    WHERE detached_on IS NULL;

-- 4. The CS owner of an organisation.
--
--    plg_cs_owner records WHO OWNS the account; nothing recorded who assigned
--    them. That is a worklist handover -- it changes whose queue the customer
--    appears in -- and it was the one pairing-level change with no trail at all.
ALTER TABLE plg_organization
    ADD COLUMN IF NOT EXISTS owner_updated_by UUID REFERENCES "user" (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS owner_updated_on TIMESTAMPTZ;

COMMENT ON COLUMN plg_organization.owner_updated_by IS
    'Who last set plg_cs_owner. Distinct from plg_cs_owner, which is who the work belongs to.';
COMMENT ON COLUMN plg_organization.owner_updated_on IS
    'When plg_cs_owner was last set. NULL if it has never been changed through the portal.';

-- 1. Uniqueness over attached rows only.
--
-- A pairing may hold one attached copy of a playbook at a time, and any number
-- of detached ones behind it -- attach, detach, attach again is a legitimate
-- history, not a conflict.
ALTER TABLE plg_playbook_run DROP CONSTRAINT IF EXISTS uq_plg_playbook_run;

CREATE UNIQUE INDEX IF NOT EXISTS uq_plg_playbook_run_attached
    ON plg_playbook_run (org_platform_id, playbook_id)
    WHERE detached_on IS NULL;

-- 2. Both views, re-declared with the filter. Bodies are 0129's, unchanged
--    apart from the WHERE each one gains.

CREATE OR REPLACE VIEW plg_playbook_run_v AS
SELECT r.id                                    AS playbook_run_id,
       r.org_platform_id,
       r.playbook_id,
       r.added_by,
       r.created_at,
       pb.name                                 AS playbook_name,
       pb.description                          AS playbook_description,
       pb.lifecycle_stage                      AS playbook_stage,
       pb.playbook_type,
       op.organization_id,
       o.organization_name,
       o.plg_cs_owner,
       op.product_id,
       pr.code                                 AS product_code,
       pr.name                                 AS product_name,
       op.lifecycle_stage                      AS current_stage,
       COUNT(t.id)                             AS task_total,
       COUNT(t.id) FILTER (WHERE t.is_completed) AS task_completed,
       CASE
           WHEN bool_or(t.code = 'CLOSE_PLAYBOOK'    AND t.is_completed) THEN 'CLOSED'
           WHEN bool_or(t.code = 'INITIATE_PLAYBOOK' AND t.is_completed) THEN 'ACTIVE'
           ELSE 'NOT_STARTED'
       END                                     AS run_status,
       (ARRAY_AGG(t.code ORDER BY t.sequence_no)
          FILTER (WHERE NOT t.is_completed))[1] AS next_task_code,
       (ARRAY_AGG(t.name ORDER BY t.sequence_no)
          FILTER (WHERE NOT t.is_completed))[1] AS next_task_name
FROM   plg_playbook_run r
JOIN   plg_playbook pb     ON pb.id = r.playbook_id
JOIN   plg_org_platform op ON op.id = r.org_platform_id
JOIN   plg_organization o  ON o.id = op.organization_id
JOIN   plg_product pr      ON pr.id = op.product_id
LEFT   JOIN plg_playbook_run_task t ON t.playbook_run_id = r.id
-- A detached run is history, not a run. Every reader of this view asks what
-- is attached to a pairing now, so the filter belongs here rather than in
-- each of them.
WHERE  r.detached_on IS NULL
GROUP  BY r.id, r.org_platform_id, r.playbook_id, r.added_by, r.created_at,
          pb.name, pb.description, pb.lifecycle_stage, pb.playbook_type,
          op.organization_id, o.organization_name, o.plg_cs_owner,
          op.product_id, pr.code, pr.name, op.lifecycle_stage;

CREATE OR REPLACE VIEW plg_work_queue_v AS
SELECT op.id                       AS org_platform_id,
       op.organization_id,
       o.organization_name,

       -- Three owner columns, for the reason given at the top of this section.
       o.plg_cs_owner,
       ow.email                    AS plg_cs_owner_email,
       ow.display_name             AS plg_cs_owner_name,

       op.product_id,
       pr.code                     AS product_code,
       pr.name                     AS product_name,
       pr.display_order            AS product_display_order,
       op.lifecycle_stage,
       ls.name                     AS lifecycle_stage_name,
       ls.display_order            AS lifecycle_stage_order,
       op.acknowledged_on,
       op.acknowledged_by,
       ack.email                   AS acknowledged_by_email,
       ack.display_name            AS acknowledged_by_name,

       -- The second axis, surfaced on every queue row. An at-risk pairing is
       -- the most valuable thing in the queue and the list has to be able to
       -- say so without a second query.
       op.health_state,
       op.health_entered_on,

       COALESCE(runs.run_total, 0)   AS run_total,
       COALESCE(runs.run_active, 0)  AS run_active,
       COALESCE(runs.run_closed, 0)  AS run_closed,
       COALESCE(runs.task_total, 0)     AS task_total,
       COALESCE(runs.task_completed, 0) AS task_completed,
       avail.available_total,

       -- Why this pairing is in the queue, most advanced state first. A pairing
       -- with one run under way and another untouched reads as IN_PROGRESS:
       -- something is moving, which is the more useful thing to know.
       --
       -- There is no "finished" reason, deliberately. Everything closed with
       -- nothing left to attach is the EXIT condition — see the WHERE clause at
       -- the bottom — so a pairing with nothing left to do leaves the queue
       -- rather than sitting in it looking like work.
       CASE
           WHEN COALESCE(runs.run_active, 0) > 0                           THEN 'IN_PROGRESS'
           WHEN COALESCE(runs.run_total, 0) > COALESCE(runs.run_closed, 0) THEN 'NOT_STARTED'
           ELSE 'NO_PLAYBOOK'
       END AS reason,

       runs.next_task_code,
       runs.next_task_name,
       runs.playbook_id,
       runs.playbook_name,
       op.registered_on
FROM   plg_org_platform op
JOIN   plg_organization o     ON o.id  = op.organization_id
JOIN   plg_product pr         ON pr.id = op.product_id
JOIN   plg_lifecycle_stage ls ON ls.stage = op.lifecycle_stage
LEFT   JOIN plg_user_v ow     ON ow.id = o.plg_cs_owner
LEFT   JOIN plg_user_v ack    ON ack.id = op.acknowledged_by

-- What is running on this pairing, and what the engineer would pick up next.
LEFT   JOIN LATERAL (
           SELECT COUNT(*)                                                  AS run_total,
                  COUNT(*) FILTER (WHERE rv.run_status = 'ACTIVE')          AS run_active,
                  COUNT(*) FILTER (WHERE rv.run_status = 'CLOSED')          AS run_closed,
                  COALESCE(SUM(rv.task_total), 0)                           AS task_total,
                  COALESCE(SUM(rv.task_completed), 0)                       AS task_completed,
                  (ARRAY_AGG(rv.next_task_code ORDER BY rv.created_at)
                     FILTER (WHERE rv.run_status = 'ACTIVE'))[1]            AS next_task_code,
                  (ARRAY_AGG(rv.next_task_name ORDER BY rv.created_at)
                     FILTER (WHERE rv.run_status = 'ACTIVE'))[1]            AS next_task_name,
                  (ARRAY_AGG(rv.playbook_id ORDER BY rv.created_at)
                     FILTER (WHERE rv.run_status = 'ACTIVE'))[1]            AS playbook_id,
                  (ARRAY_AGG(rv.playbook_name ORDER BY rv.created_at)
                     FILTER (WHERE rv.run_status = 'ACTIVE'))[1]            AS playbook_name
           FROM   plg_playbook_run_v rv
           WHERE  rv.org_platform_id = op.id
       ) runs ON TRUE

-- How many playbooks this pairing could still add, at its current stage AND of
-- the kind its health calls for. Zero with nothing attached is the case an
-- author has to fix, not an engineer.
--
-- The type filter is what makes health drive the queue. A COMMERCIAL pairing
-- that has finished its progressive and sustaining playbooks counts zero and
-- leaves; mark it AT_RISK and the recovery playbooks at COMMERCIAL start
-- counting, so it returns on its own. Nothing special-cases that.
LEFT   JOIN LATERAL (
           SELECT COUNT(*) AS available_total
           FROM   plg_playbook pb
           WHERE  pb.product_id      = op.product_id
             AND  pb.lifecycle_stage = op.lifecycle_stage
             AND  pb.active
             AND  pb.playbook_type   = ANY(
                      plg_applicable_playbook_types(op.health_state))
             -- detached_on IS NULL, so a playbook taken off a pairing becomes
             -- attachable again rather than being barred for ever by the row
             -- that records it was once removed.
             AND  NOT EXISTS (SELECT 1 FROM plg_playbook_run r
                              WHERE r.org_platform_id = op.id AND r.playbook_id = pb.id
                                  AND r.detached_on IS NULL)
       ) avail ON TRUE

-- Who is on the list.
--
-- The question is simply: is there anything left to DO?
--
--   acknowledged          nobody works a registration they have not claimed
--   not ABANDONED         terminal; there is no work on a customer who left
--   something to do       a run going, a run untouched, or a playbook that
--                         could still be attached for this health
--
-- The third clause is the exit condition. A pairing with everything closed and
-- nothing attachable is finished — it leaves rather than sitting on the list
-- looking like work.
--
-- "Nothing attached yet" is NOT an exit: no runs and an available playbook
-- counts as work, and shows as NO_PLAYBOOK. That is the state every freshly
-- acknowledged registration is in, and it is exactly what an engineer opens the
-- queue to find.
WHERE  op.acknowledged_on IS NOT NULL
  AND  op.lifecycle_stage <> 'ABANDONED'
  AND  ( COALESCE(runs.run_active, 0) > 0
      OR COALESCE(runs.run_total, 0) > COALESCE(runs.run_closed, 0)
      OR COALESCE(avail.available_total, 0) > 0 );

COMMIT;
