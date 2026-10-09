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

-- Product change, requested directly: a security announcement should still
-- be visible to a project's ordinary portal users (PORTAL_USER/LEAD_USER)
-- when that project has NO security contact at all -- only when a real
-- security contact exists should visibility narrow to that role alone.
-- Confirmed live: several real projects (checked against this database
-- copy) have registered PORTAL_USER contacts but no SECURITY_CONTACT at
-- all, and would otherwise see zero security announcements ever, for as
-- long as that stays true.
--
-- IMPORTANT, discovered while making this change, and it changed this
-- migration's own design: the policy this alters does NOT match what
-- entity-service's own migration 100001 last defined. That migration's
-- announcement_visibility policy calls an announcement_is_security(id,
-- type) FUNCTION (deriving security-or-not from announcement_type plus a
-- "Security Announcement" work_item_tag), but that function does not
-- exist in this database at all -- the policy actually deployed here
-- (confirmed via \d announcement / pg_policies) instead read a plain
-- is_security_announcement BOOLEAN column directly, a shape
-- entity-service's own migrations/ never creates (announcement's real
-- schema is built by a separate service in staging/production -- see this
-- repo's own CLAUDE.md). Separately, and directly relevant to which of
-- those two signals this migration should build on: there is an
-- in-flight, not-yet-merged plan (PR #1974) to remove the
-- is_security_announcement boolean entirely in favor of announcement_type
-- alone. Checked live against this database copy before deciding: right
-- now, announcement_type is 0% reliable for this purpose (zero rows have
-- announcement_type = 'SECURITY' at all; every one of the 48 real
-- security announcements here is only identifiable via the tag), so this
-- migration does NOT special-case or depend on the boolean column, but
-- also does NOT switch to announcement_type alone -- either would either
-- entrench a column being deprecated or immediately stop restricting every
-- currently-known real security announcement. Instead it revives migration
-- 100001's own two-signal function (which was written but, per the above,
-- never actually reached this database), computing security status live
-- from announcement_type OR the tag on every read rather than trusting
-- either column's own bookkeeping. Once announcement_type becomes reliable
-- (PR #1974 and/or a real backfill), this function is the one place that
-- needs simplifying back down to a plain column check -- not the policy
-- itself.
--
-- One transaction: this file already had DROP POLICY IF EXISTS in front of
-- its one CREATE POLICY, but no BEGIN/COMMIT wrapper around the two
-- statements -- added for atomicity with the rest of this migration series.
BEGIN;

CREATE OR REPLACE FUNCTION announcement_is_security(ann_id UUID, ann_type announcement_type_enum)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
  SELECT ann_type = 'SECURITY'
    OR EXISTS (
      SELECT 1
      FROM work_item_tag wit
      JOIN tag t ON t.id = wit.tag_id
      WHERE wit.work_item_id = ann_id
        AND LOWER(t.name) = LOWER('Security Announcement')
    )
$$;

-- project_has_security_contact centralizes the new "does this project have
-- anyone who can see security announcements at all" check -- written the
-- same way is_project_member is (migration 100002): a shared, STABLE
-- function rather than a per-policy copy of the same join chain, so this
-- rule can't drift if it's ever needed elsewhere.
CREATE OR REPLACE FUNCTION project_has_security_contact(target_project_id UUID)
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
  SELECT EXISTS (
    SELECT 1
    FROM project_contact pc
    JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
    JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
    JOIN project_role pr ON pr.id = pgr.project_role_id
    WHERE pc.project_id = target_project_id
      AND pr.role = 'SECURITY_CONTACT'
      -- A deactivated contact can no longer see anything, so it must not
      -- count as "the project has a security contact" and keep the
      -- fallback off. Same predicate the rest of entity-service uses.
      AND (pc.state IS NULL OR pc.state <> 'DEACTIVATED'::project_contact_state_enum)
  )
$$;

DROP POLICY IF EXISTS announcement_visibility ON announcement;

CREATE POLICY announcement_visibility ON announcement
  FOR SELECT
  USING (
    current_setting('app.is_internal', true) = 'true'
    OR EXISTS (
      SELECT 1
      FROM work_item wi
      JOIN project_contact pc ON pc.project_id = wi.project_id
      JOIN project_contact_group pcg ON pcg.project_contact_id = pc.id
      JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
      JOIN project_role pr ON pr.id = pgr.project_role_id
      WHERE wi.id = announcement.id
        -- LOWER(...) both sides, and NULLIF guarding the bare column, not a
        -- plain pc.email = current_setting(...) comparison -- migration
        -- 100001's original policy (which this DROP POLICY/CREATE POLICY
        -- replaces) used exactly this form, with its own supporting
        -- expression index on LOWER(email); dropping the LOWER() here would
        -- silently deny a legitimate viewer whose stored project_contact
        -- email differs only in case from what the JWT carries, and would
        -- stop using that index. NULLIF matches is_project_member's own
        -- guard (migration 100002): current_setting(..., true) returns ''
        -- (not NULL) once a transaction-local set_config reverts, and a
        -- bare '' could in principle match a project_contact row with a
        -- blank email.
        AND LOWER(pc.email) = LOWER(NULLIF(current_setting('app.viewer_email', true), ''))
        AND (
          (NOT announcement_is_security(announcement.id, announcement.announcement_type) AND pr.role IN ('PORTAL_USER', 'LEAD_USER'))
          OR (announcement_is_security(announcement.id, announcement.announcement_type) AND pr.role = 'SECURITY_CONTACT')
          -- New fallback: a security announcement in a project with no
          -- security contact at all is visible to ordinary portal users
          -- instead of being invisible to everyone but internal callers.
          OR (
            announcement_is_security(announcement.id, announcement.announcement_type)
            AND pr.role IN ('PORTAL_USER', 'LEAD_USER')
            AND NOT project_has_security_contact(wi.project_id)
          )
        )
    )
  );

COMMIT;
