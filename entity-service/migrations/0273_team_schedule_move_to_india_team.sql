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

-- "Move to India Team": the India region shift as a move to the Phoenix team.
--
-- Phoenix is the India team: its engineers' ordinary day is the India region
-- shift (CRE_REGULAR_IND). Somebody from another team put on that shift is
-- working on the India team for those days, so it is offered the way the
-- Americas cover already is (ALLO_BR): a tag that moves them to Phoenix for a
-- span, worked there as the India region shift. The day picker then lists it
-- under "Move to another team" instead of among the standing hours.
--
-- Data only: no table, column, type or constraint is created or changed.
-- Nobody is moved by this script, and days already on the India region shift
-- are left as they are.
--
-- Re-runnable: the tag is added only when a team with key 'phoenix' exists
-- and the code is new. One transaction with a short lock timeout, as the
-- other schedule migrations.

BEGIN;
SET LOCAL lock_timeout = '5s';

INSERT INTO team_schedule_absence_kind
    (code, short_code, label, bucket, colour_token, sort_order, is_active, family,
     moves_to_team_key, works_rota_there, shows_as_shift_code, created_by, updated_by)
SELECT 'ALLO_IND', 'IND', 'India team rotation', 'ALLOCATION', 'IND', 72, TRUE, 'CRE',
       t.key, TRUE, 'CRE_REGULAR_IND', 'migration', 'migration'
  FROM team t
 WHERE t.key = 'phoenix'
   AND EXISTS (SELECT 1 FROM team_schedule_shift s WHERE s.code = 'CRE_REGULAR_IND')
ON CONFLICT (code) DO NOTHING;

COMMIT;
