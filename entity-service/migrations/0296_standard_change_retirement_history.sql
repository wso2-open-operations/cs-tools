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

-- A standard change template's retirement is recorded in its history, and a
-- retired template carries no review flag.
--
-- standard_change_template_event (0287) gains the action RETIRED and a
-- proposal_id: the retirement proposal whose approval retired the template
-- (the history links to it; its note is that proposal's rationale). A proposal
-- is deleted only as a never-submitted draft, which retires nothing, so ON
-- DELETE SET NULL never loses a retirement's link in practice.
--
-- Templates retired before this migration get their RETIRED entry from the
-- approved retirement proposal, dated when it was decided and attributed to
-- the member whose approval completed it (or "migration" when that user is
-- gone). Their review flag is cleared: a retired template is never used again,
-- so there is nothing left to review, and the flag could never be cleared
-- (clear-review refuses a retired template). No REVIEW_CLEARED entry is
-- written for it; the RETIRED entry says why the flag went.
--
-- standard_change_template (100026) and standard_change_template_event
-- (100031) are row-level-secured with FORCE: a migrating role without
-- BYPASSRLS that is not flagged internal sees no row, and the INSERT and the
-- UPDATE would write nothing without an error. So the session is flagged
-- internal and cleared afterwards, as 0288 and 0295 do.
--
-- Safe to re-run: the constraint is dropped and recreated, the column is added
-- IF NOT EXISTS, a template that already has a RETIRED entry gets no second
-- one, and a cleared flag stays cleared.

BEGIN;
SET LOCAL lock_timeout = '5s';

SELECT set_config('app.is_internal', 'true', false);

ALTER TABLE standard_change_template_event DROP CONSTRAINT IF EXISTS standard_change_template_event_action_check;
ALTER TABLE standard_change_template_event ADD CONSTRAINT standard_change_template_event_action_check
    CHECK (action IN ('SUSPENDED', 'RESUMED', 'REVIEW_FLAGGED', 'REVIEW_CLEARED', 'RETIRED'));

ALTER TABLE standard_change_template_event
    ADD COLUMN IF NOT EXISTS proposal_id UUID REFERENCES standard_change_proposal(id) ON DELETE SET NULL;

INSERT INTO standard_change_template_event (template_id, action, note, proposal_id, actor, created_on)
SELECT p.template_id, 'RETIRED', NULLIF(btrim(COALESCE(p.rationale, '')), ''), p.id,
       COALESCE(u.email, 'migration'), COALESCE(p.decided_on, NOW())
  FROM standard_change_proposal p
  JOIN standard_change_template t ON t.id = p.template_id
  LEFT JOIN "user" u ON u.id = p.decided_by_user_id
 WHERE p.kind = 'RETIRE' AND p.state = 'APPROVED' AND t.state = 'RETIRED'
   AND NOT EXISTS (SELECT 1 FROM standard_change_template_event e
                    WHERE e.template_id = p.template_id AND e.action = 'RETIRED');

UPDATE standard_change_template
   SET review_required = false, review_required_reason = NULL, review_required_on = NULL,
       review_required_change_request_id = NULL
 WHERE state = 'RETIRED' AND review_required
   -- 0254's check refuses any write to a template with no customer; such a
   -- template keeps its flag, which no page shows on a retired template.
   AND account_id IS NOT NULL;

SELECT set_config('app.is_internal', '', false);

COMMIT;
