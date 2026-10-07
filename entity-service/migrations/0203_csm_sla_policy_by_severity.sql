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

-- Seeds one deterministic, source='CSM' sla_policy row per (severity,
-- clock_type) pair already defined in sla_duration_policy (migration 0192),
-- replacing internal/service/sla_policy_resolver.go's old name/pattern
-- matching against the real ServiceNow-synced sla_policy rows plus its
-- "which support plan is this case on" guess (resolveCasePlan). That lookup
-- silently found nothing for every LOW-severity (S4/"Query") case -- the
-- real synced rows for it are named "QuerySLA", "Onboarding Case Customer
-- Query Response", etc, none matching the resolver's assumed
-- "<prefix> - <label> (<plan>)" convention under either plan label or the
-- loose pattern fallback -- so RegisterCaseClocks silently registered
-- nothing for every such case.
--
-- Named deterministically ("<severity> - <TARGET> (CSM)", e.g.
-- "S4 - RESPONSE (CSM)") rather than mimicking ServiceNow's own naming
-- convention, since these rows are never meant to be found by guessing --
-- the resolver now builds this exact name from the severity it already has
-- and looks it up once. Durations are drawn directly from
-- sla_duration_policy rather than re-stated here, so the two tables (and the
-- two SLA engines that each read one of them -- this one, and
-- csm-notification-service's own Redis-based tracker) can never disagree on
-- a duration. Mirrors migration 0136_csm_p0_sla_policies.sql's own
-- precedent (seeding CSM rows for a severity the sync doesn't cover),
-- generalized to every severity instead of just P0/Catastrophic.
INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, is_active, target, duration, source)
SELECT gen_random_uuid(), NOW(), NOW(), 'migration-0203', 'migration-0203',
       dp.severity::TEXT || ' - ' || UPPER(dp.clock_type) || ' (CSM)',
       TRUE,
       UPPER(dp.clock_type)::sla_policy_target_enum,
       dp.duration,
       'CSM'::sla_source_enum
FROM sla_duration_policy dp
WHERE NOT EXISTS (
    SELECT 1 FROM sla_policy sp
    WHERE sp.name = dp.severity::TEXT || ' - ' || UPPER(dp.clock_type) || ' (CSM)'
);
