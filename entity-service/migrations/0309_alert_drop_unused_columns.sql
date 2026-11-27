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

-- Drops what the alert table no longer uses: the claim columns (claimed_by, claim_expires_on, processed_on), the alert JSON now held in columns, the fingerprint now computed from them, and the error rows with error_reason, error and content_type. Safe to run again.
-- Run after 0308 and only after the data migration that needs these has finished: dropping a column discards its values, and the error rows are deleted.

-- Error rows are messages the consumer could not normalise before it dead-lettered them; alert is FORCE ROW LEVEL SECURITY, so they are deleted as the internal identity.
SELECT set_config('app.is_internal', 'true', false);
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'alert' AND column_name = 'error_reason') THEN
        DELETE FROM alert WHERE error_reason IS NOT NULL;
    END IF;
END $$;
SELECT set_config('app.is_internal', '', false);

DROP INDEX IF EXISTS alert_unprocessed_idx;
DROP INDEX IF EXISTS alert_unprocessed_fingerprint_idx;
DROP INDEX IF EXISTS alert_error_idx;

-- Every row is now an alert, so the row-kind check becomes NOT NULL on the columns it required.
ALTER TABLE alert
    DROP CONSTRAINT IF EXISTS alert_row_kind,
    DROP COLUMN IF EXISTS claimed_by,
    DROP COLUMN IF EXISTS claim_expires_on,
    DROP COLUMN IF EXISTS processed_on,
    DROP COLUMN IF EXISTS alert,
    DROP COLUMN IF EXISTS fingerprint,
    DROP COLUMN IF EXISTS error_reason,
    DROP COLUMN IF EXISTS error,
    DROP COLUMN IF EXISTS content_type,
    ALTER COLUMN route SET NOT NULL,
    ALTER COLUMN source SET NOT NULL,
    ALTER COLUMN service SET NOT NULL,
    ALTER COLUMN metric_name SET NOT NULL,
    ALTER COLUMN environment SET NOT NULL,
    ALTER COLUMN unique_identifier SET NOT NULL;
