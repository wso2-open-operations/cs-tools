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

-- Six WORKAROUND/RESOLUTION policy rows have stop_condition = 'stateINCLOSED'
-- only -- missing the "^ORfix_eta_sharedISNOTEMPTY" branch every one of
-- their siblings carries, so sharing a fix ETA with the customer never
-- completes these clocks the way it does for every other severity/track.
-- Confirmed live: case CS0451994 (06b405a1-3b58-5390-3e1e-088aa4e45a48, P0,
-- Open Source) had its workaround clock correctly PAUSE when a workaround
-- was provided, but never complete once the fix ETA was subsequently shared
-- -- its own policy ("P0 - Workaround (Open Source)") simply has no clause
-- to react to it. The same gap exists for P1 Workaround (Open Source), and
-- for all four Open Source Resolution severities (P0-P3) -- queried
-- directly against sla_policy: every Managed Services row, every other
-- Open Source Workaround severity (P2/P3), and even the unrelated
-- "Query - Resolution (Open Source)" row already carry the clause; only
-- these six never got it.
--
-- Fixed by appending the same clause every sibling policy already has,
-- verbatim, changing nothing else about these rows.
--
-- NOTE: applied directly to staging (not through `make migrate`), so it is
-- not recorded in csm_migration_applied_migration there. Not applied to
-- production as part of this change.
--
-- Backup of the six touched rows, pre-change, kept in
-- sla_policy_backup_0269, never dropped.

CREATE TABLE sla_policy_backup_0269 AS
SELECT id, name, stop_condition
FROM sla_policy
WHERE id IN (
    'd85baefd-7b84-4ccd-805d-46b4ca9c46ed', -- P0 - Workaround (Open Source)
    '33c4353b-475f-d590-a0a2-9cd3846d43ac', -- P1 - Workaround (Open Source)
    'af4e95d3-1bde-5610-a002-c9d3604bcb08', -- P0 - Resolution (Open Source)
    '7e9d1898-1b20-0550-6a67-caa1604bcb8f', -- P1 - Resolution (Open Source)
    '726dc150-1b64-0550-6a67-caa1604bcbad', -- P2 - Resolution (Open Source)
    'ce9e8d50-1b64-0550-6a67-caa1604bcb87'  -- P3 - Resolution (Open Source)
);

UPDATE sla_policy SET stop_condition = 'stateINCLOSED^ORfix_eta_sharedISNOTEMPTY'
WHERE id = 'd85baefd-7b84-4ccd-805d-46b4ca9c46ed'; -- P0 - Workaround (Open Source)

UPDATE sla_policy SET stop_condition = 'stateINCLOSED^ORfix_eta_sharedISNOTEMPTY'
WHERE id = '33c4353b-475f-d590-a0a2-9cd3846d43ac'; -- P1 - Workaround (Open Source)

UPDATE sla_policy SET stop_condition = 'stateINCLOSED^ORfix_eta_sharedISNOTEMPTY'
WHERE id = 'af4e95d3-1bde-5610-a002-c9d3604bcb08'; -- P0 - Resolution (Open Source)

UPDATE sla_policy SET stop_condition = 'stateINCLOSED^ORfix_eta_sharedISNOTEMPTY'
WHERE id = '7e9d1898-1b20-0550-6a67-caa1604bcb8f'; -- P1 - Resolution (Open Source)

UPDATE sla_policy SET stop_condition = 'stateINCLOSED^ORfix_eta_sharedISNOTEMPTY'
WHERE id = '726dc150-1b64-0550-6a67-caa1604bcbad'; -- P2 - Resolution (Open Source)

UPDATE sla_policy SET stop_condition = 'stateINCLOSED^ORfix_eta_sharedISNOTEMPTY'
WHERE id = 'ce9e8d50-1b64-0550-6a67-caa1604bcb87'; -- P3 - Resolution (Open Source)
