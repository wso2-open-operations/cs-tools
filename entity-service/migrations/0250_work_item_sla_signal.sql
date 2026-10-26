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

-- The one supporting table outside the "sla_" namespace: a derived-signal
-- store, one row per work item, kept current by existing comment/status-
-- change code paths (case_service.go/sn_case_service.go's own
-- isSupportEngineerAuthor/isSupportEngineerAuthorSN -- already computed
-- today for the old SLA engine's CompleteResponseClock, reused here rather
-- than reimplemented) immediately before the new SLA engine evaluates any
-- condition against that work item.
--
-- This exists specifically to let a condition (SLA SPEC.md R4) react to
-- something that is, in reality, a related-record event -- "a support
-- engineer replied," "the customer replied" -- while R6 and R18-R21 still
-- hold exactly as written ("conditions evaluate only against... fields of
-- the work item itself... related-record lookups are not supported"): a
-- derived signal IS a field of the work item once it lands here, computed
-- once by application code rather than read through a join at condition-
-- evaluation time. See sla_field_catalog_entry's own 'derived_signal'
-- storage_kind (migration 0245).
--
-- Not an extension of work_item itself (no ALTER TABLE on that table): a
-- work item without this row simply has no derived-signal state yet, and
-- every column here is nullable for exactly that reason -- a condition
-- referencing one of these fields before this table has ever been written
-- for that work item reads NULL, which any predicate using 'is_not_null'/
-- 'changed_to' already handles as "not yet true," no special-casing needed.
BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS work_item_sla_signal (
    work_item_id                  UUID PRIMARY KEY REFERENCES work_item(id),
    last_comment_author_role      VARCHAR(20),
    last_comment_on               TIMESTAMPTZ,
    is_awaiting_customer_response BOOLEAN,
    updated_on                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT work_item_sla_signal_author_role_check
        CHECK (last_comment_author_role IS NULL
               OR last_comment_author_role IN ('SUPPORT_ENGINEER', 'CUSTOMER', 'SYSTEM'))
);

COMMENT ON TABLE work_item_sla_signal IS
    'Derived, application-maintained signal fields the SLA field registry exposes as ordinary work-item fields (R6, R16, R18-R21) -- not a raw sync of any upstream table. Written by the same comment/status-change code paths that already compute these facts for other purposes.';

COMMIT;
