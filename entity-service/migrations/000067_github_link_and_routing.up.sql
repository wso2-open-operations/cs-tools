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

-- How a change request reaches its GitHub issue, and which repository to use.
--
-- BOTH OF THESE ARE TAKEN FROM THE SERVICENOW FLOW, not inferred. Reading
-- "[GitHub Integration] SN CR Created -> GitHub" settled two things that a
-- first guess got wrong:
--
--  1. THE ISSUE IS LINKED TO THE CASE, NOT THE CHANGE REQUEST. The flow
--     triggers on "Change Request Created where Parent is not empty", looks up
--     sn_customerservice_case by the CR's parent, and reads the issue number
--     off the CASE. A change request reaches GitHub only through its parent.
--
--  2. ROUTING IS BY ACCOUNT, NOT PRODUCT. The flow reads
--     github.dispatch.config keyed by the case's account name, and each entry
--     carries its own owner, repo AND credential -- so different customers use
--     different repositories and different tokens.

-- The issue a case is linked to. On the case rather than the work item because
-- only cases carry this upstream; a change request has no issue of its own.
ALTER TABLE "case" ADD COLUMN IF NOT EXISTS github_issue_number INTEGER;

-- Partial: most cases have no linked issue, and the outbound path only ever
-- asks about the ones that do.
CREATE INDEX IF NOT EXISTS idx_case_github_issue_number
    ON "case" (github_issue_number) WHERE github_issue_number IS NOT NULL;

-- Which repository an account's issues live in.
--
-- Replaces github.dispatch.config, a JSON blob in a system property. A table
-- gets foreign keys, one row per account, and can be edited without a deploy.
CREATE TABLE IF NOT EXISTS account_github_repo (
    id UUID PRIMARY KEY,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,

    account_id UUID NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    -- Spelled as GitHub spells them. Case matters for display; lookups
    -- lower-case both sides, since GitHub routes case-insensitively.
    owner VARCHAR(100) NOT NULL,
    repository VARCHAR(200) NOT NULL,

    -- The credential this account's repository uses. A NAME, not a secret --
    -- it resolves to an entry in the platform secret store. ServiceNow kept a
    -- credential sys_id here and decrypted a PAT inline; a reference keeps the
    -- token out of the database entirely.
    credential_ref VARCHAR(100),

    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    -- One account, one repository. The flow's config was a map keyed by
    -- account name, so it could not express two either.
    CONSTRAINT uq_account_github_repo_account UNIQUE (account_id),
    -- And a repository belongs to one account, which is what makes the inbound
    -- lookup unambiguous and removes the need for a separate allow-list.
    CONSTRAINT uq_account_github_repo_repo UNIQUE (owner, repository)
);

CREATE INDEX IF NOT EXISTS idx_account_github_repo_lookup
    ON account_github_repo (lower(owner), lower(repository));
