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
-- Closure state columns on project default to OPEN.
--
-- wso2_closure_state, end_date_closure_state, invoice_due_date_closure_state
-- and compliance_violation_closure_state were created nullable with no
-- default (0014). A project inserted without them (the Salesforce project
-- ingest) kept NULL, and the Account Closure Process lists candidates with
-- wso2_closure_state = 'OPEN', so such a project was never evaluated: no
-- reminder, no suspension. This migration backfills the NULLs and sets the
-- column DEFAULT so any insert that omits the columns gets OPEN.
--
-- Backfill order matters. The three sub-states (end date, invoice due date,
-- compliance violation) go NULL -> OPEN first. wso2_closure_state is then
-- derived from them where it is NULL, with the same rule the application uses
-- (wso2ClosureStateRule in project_repo.go): SUSPENDED when the end date state
-- is CLOSED or SUSPENDED, the invoice state starts with SUSPENDED, or the
-- compliance state is SUSPENDED; else RESTRICTED when the end date state is
-- RESTRICTED or the invoice state starts with RESTRICTED; else OPEN. A
-- constant OPEN would wrongly reopen a project whose sub-states say it is
-- closed or suspended. Keep the CASE below in step with that constant.
--
-- The columns stay nullable on purpose: existing readers already treat NULL
-- as "not restricted" (project_repo.go, deployed_product_repo.go), and a
-- NOT NULL would make any writer that sets an explicit NULL fail.
--
-- Idempotent: every UPDATE touches only rows whose column IS NULL (a non-NULL
-- value is never changed), and SET DEFAULT can be repeated. updated_on and
-- updated_by are left alone, so the backfill does not look like a user edit.
--
-- Locking and size: the project table has about 2200 rows. Each UPDATE is one
-- statement that takes row locks only on rows it changes, in well under a
-- second. SET DEFAULT is a catalog-only change (no table rewrite). Each
-- ALTER TABLE needs an ACCESS EXCLUSIVE lock for an instant, so the file runs
-- with a short session lock_timeout: it gives up after 5s instead of queueing
-- every later read and write of project behind it.
--
-- Applied with plain autocommit, like every file here: each statement is its
-- own transaction. If one fails (for example on lock_timeout), the earlier
-- statements stay applied; that is safe because every statement is idempotent,
-- so re-run the file when the table is quiet.

SET lock_timeout = '5s';

UPDATE project
SET end_date_closure_state = 'OPEN'
WHERE end_date_closure_state IS NULL;

UPDATE project
SET invoice_due_date_closure_state = 'OPEN'
WHERE invoice_due_date_closure_state IS NULL;

UPDATE project
SET compliance_violation_closure_state = 'OPEN'
WHERE compliance_violation_closure_state IS NULL;

UPDATE project
SET wso2_closure_state = (CASE
	WHEN end_date_closure_state::TEXT IN ('CLOSED', 'SUSPENDED')
	  OR invoice_due_date_closure_state::TEXT LIKE 'SUSPENDED%'
	  OR compliance_violation_closure_state::TEXT = 'SUSPENDED' THEN 'SUSPENDED'
	WHEN end_date_closure_state::TEXT = 'RESTRICTED'
	  OR invoice_due_date_closure_state::TEXT LIKE 'RESTRICTED%' THEN 'RESTRICTED'
	ELSE 'OPEN' END)::wso2_closure_state_enum
WHERE wso2_closure_state IS NULL;

ALTER TABLE project
    ALTER COLUMN wso2_closure_state SET DEFAULT 'OPEN',
    ALTER COLUMN end_date_closure_state SET DEFAULT 'OPEN',
    ALTER COLUMN invoice_due_date_closure_state SET DEFAULT 'OPEN',
    ALTER COLUMN compliance_violation_closure_state SET DEFAULT 'OPEN';

RESET lock_timeout;
