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

-- onboarding_step records the outcome of each step the customer onboarding
-- flow performs for one Salesforce project membership (Project_Contact__c):
-- identity creation, CSM DB upsert, invitation email, and first-access
-- registration. One row per membership per step, holding the latest outcome;
-- retries update the row and bump attempt_count. The Salesforce ids and the
-- invited email are stored even when no project / contact row resolves, so a
-- failed onboarding is visible to the CSM Portal with its reason.

DO $$ BEGIN
    CREATE TYPE onboarding_step_enum AS ENUM (
        'IDENTITY',      -- Asgardeo create-if-absent
        'DATABASE',      -- CSM DB upsert (user, account_contact, project_contact, roles)
        'EMAIL',         -- invitation email
        'REGISTRATION'   -- first access: contact lockout flag cleared, membership REGISTERED
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE onboarding_step_status_enum AS ENUM ('SUCCEEDED', 'FAILED', 'SKIPPED');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS onboarding_step (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    -- Salesforce Project_Contact__c Id: the key carried by the onboarding event.
    membership_sf_id VARCHAR(100) NOT NULL,
    -- Salesforce Contact Id of the invited person.
    contact_sf_id VARCHAR(100),
    -- Invited address, kept even when no user / contact row resolves.
    email VARCHAR(255) NOT NULL,
    project_id UUID REFERENCES project(id) ON DELETE SET NULL,
    project_contact_id UUID REFERENCES project_contact(id) ON DELETE SET NULL,
    step onboarding_step_enum NOT NULL,
    status onboarding_step_status_enum NOT NULL,
    attempt_count INT NOT NULL DEFAULT 1,
    last_error TEXT,
    -- Salesforce event operation this row reflects: CREATED, UPDATED or RESTORED.
    event_type VARCHAR(20) NOT NULL,
    -- Salesforce LastModifiedDate of the membership this row reflects; an event
    -- that is not newer than this value is a duplicate and is ignored.
    event_modified_on TIMESTAMPTZ NOT NULL,
    UNIQUE (membership_sf_id, step)
);

CREATE INDEX IF NOT EXISTS idx_onboarding_step_project_id ON onboarding_step (project_id);
CREATE INDEX IF NOT EXISTS idx_onboarding_step_project_contact_id ON onboarding_step (project_contact_id);
CREATE INDEX IF NOT EXISTS idx_onboarding_step_failed
    ON onboarding_step (updated_on DESC) WHERE status = 'FAILED';
