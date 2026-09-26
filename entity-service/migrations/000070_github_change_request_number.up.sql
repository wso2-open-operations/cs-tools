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

-- Numbering for change requests raised from GitHub.
--
-- A SEPARATE PREFIX, NOT A SHARED SEQUENCE. Existing numbers look like
-- CHG0040007 and are assigned by ServiceNow, not generated here -- there is no
-- sequence behind them in this database. Drawing native numbers from the same
-- space would mean guessing a high-water mark and hoping the sync never
-- catches up, and the failure mode is a unique-violation on a live insert.
--
-- CHG-GH-000001 cannot collide with CHG0040007 by construction, and a reader
-- can tell at a glance where a record came from. If ServiceNow's numbering is
-- ever retired, this becomes the only scheme and the prefix can be revisited
-- then.
CREATE SEQUENCE IF NOT EXISTS github_change_request_number_seq START 1;

-- Format is fixed here rather than in Go so every caller agrees, including
-- anything written later in SQL.
CREATE OR REPLACE FUNCTION next_github_change_request_number()
RETURNS TEXT AS $$
BEGIN
    RETURN 'CHG-GH-' || LPAD(nextval('github_change_request_number_seq')::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;
