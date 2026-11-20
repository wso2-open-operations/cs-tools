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

-- case_tag_automation: the case tags the system added by itself, so the system
-- never removes a tag a person added. One row per (case, tag label). The
-- s_dip rule (internal/service/product_issue_case_tags.go) inserts a row when it
-- adds s_dip and deletes it when it removes the tag; a tag already on the case
-- when the rule wants it is left unrecorded and stays manual. The row is only
-- the system's claim: the rule also reads the case's current tags, so a row
-- left behind by a person removing the tag is harmless.
-- Additive and idempotent: safe to re-run. Its row-level security is
-- 100030_case_tag_automation_rls.sql.

CREATE TABLE IF NOT EXISTS case_tag_automation (
    case_id   UUID        NOT NULL REFERENCES work_item(id) ON DELETE CASCADE,
    tag_label TEXT        NOT NULL CHECK (btrim(tag_label) <> ''),
    added_on  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (case_id, tag_label)
);

-- Not audited: the row is the rule's bookkeeping; the tag change itself is on
-- work_item_tag and the rule writes a work note for each one. Recorded as
-- excluded so the new-table warning (0295) does not name it.
DO $$
BEGIN
    IF to_regclass(format('%I.%I', current_schema(), 'case_tag_automation')) IS NOT NULL
       AND NOT EXISTS (SELECT 1 FROM audit_config
                        WHERE schema_name = current_schema() AND table_name = 'case_tag_automation') THEN
        PERFORM exclude_from_audit(current_schema(), 'case_tag_automation',
            'the s_dip rule''s record of its own tag adds; tag changes are on work_item_tag and in a work note');
    END IF;
END $$;
