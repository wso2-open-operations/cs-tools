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

-- The default SLA policies for product issues (work items of type PRODUCT_ISSUE):
-- one RESOLUTION clock per issue, chosen by the issue's priority label.
--
--   P1  24 hours, continuous ("CS 24x7": wall clock, but computed in Go so it can be
--       updated at an event's own time)
--   P2  24 hours on "CS 12x5 IST" (Monday-Friday 09:00-21:00 Asia/Kolkata)
--   P3  48 hours on "CS 12x5 IST"
--
-- No priority, or one that is not P1/P2/P3, matches none of them: no clock.
-- Only "Waiting on CS" pauses (waits_on = CS); every other open status accrues. A
-- resolved or closed issue stops the clock. Conditions are in the neutral predicate
-- grammar of internal/slacondition (source CSM is parsed as written). The policies are
-- tagged applies_to = 'PRODUCT_ISSUE' (migration 0306), so case and incident
-- resolution never lists them.
--
-- Safe to run again: a policy is inserted only when no row with its name and
-- applies_to exists, and existing rows are never changed.

INSERT INTO sla_policy (
    id, created_on, updated_on, created_by, updated_by, name, is_active, target, duration,
    schedule, retroactive, when_to_cancel, when_to_resume, start_condition, stop_condition,
    pause_condition, source, applies_to)
SELECT gen_random_uuid(), NOW(), NOW(), 'migration-0307', 'migration-0307',
       v.name, TRUE, 'RESOLUTION'::sla_policy_target_enum, v.duration::interval,
       v.schedule, TRUE, 'NEVER'::sla_policy_when_to_cancel_enum,
       'PAUSE_CONDITIONS_ARE_NOT_MET'::sla_policy_when_to_resume_enum,
       v.start_condition, 'stateINRESOLVED,CLOSED', 'waits_on=CS',
       'CSM'::sla_source_enum, 'PRODUCT_ISSUE'
FROM (VALUES
    ('Product Issue P1', '24 hours', 'CS 24x7',       'priority=P1^stateNOT INRESOLVED,CLOSED'),
    ('Product Issue P2', '24 hours', 'CS 12x5 IST',   'priority=P2^stateNOT INRESOLVED,CLOSED'),
    ('Product Issue P3', '48 hours', 'CS 12x5 IST',   'priority=P3^stateNOT INRESOLVED,CLOSED')
) AS v(name, duration, schedule, start_condition)
WHERE NOT EXISTS (
    SELECT 1 FROM sla_policy sp WHERE sp.name = v.name AND sp.applies_to = 'PRODUCT_ISSUE'
);
