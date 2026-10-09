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

-- Two roles whose holders are emailed when a case is escalated to EL4 / EL5.
-- ServiceNow named these people in system properties
-- (x_wso2_customer_0.escalation.el4.cco_email / .cro_email and
-- .el5.ceo_email); here they are whoever is granted the role, so changing who
-- an EL4 or EL5 escalation reaches is a grant, not a redeploy. EL3's CRE head
-- needs no role: it is the existing team position cre_head.
--
-- Escalation is cumulative: an EL5 escalation also reaches every EL4 holder.
-- Grant case_escalation_el4 to the CCO and CRO and case_escalation_el5 to the
-- CEO. There is no role-assignment UI yet, so a grant is an INSERT into
-- user_role (see migration 0156). Grant them only to internal staff: the
-- escalation email is internal, and entity-service mails only @wso2.com
-- addresses whatever the role holds.
--
-- ON CONFLICT (name), not (id): the ServiceNow sync seeds this table too, so
-- a deployment may already have a row under a different id. Re-running is
-- safe.

BEGIN;
SET LOCAL lock_timeout = '5s';

INSERT INTO role (id, created_on, updated_on, created_by, updated_by, name, description)
VALUES
    (gen_random_uuid(), now(), now(), 'migration', 'migration',
     'case_escalation_el4', 'Emailed when a case is escalated to EL4 or above (ServiceNow: CCO / CRO)'),
    (gen_random_uuid(), now(), now(), 'migration', 'migration',
     'case_escalation_el5', 'Emailed when a case is escalated to EL5 (ServiceNow: CEO)')
ON CONFLICT (name) DO NOTHING;

COMMIT;
