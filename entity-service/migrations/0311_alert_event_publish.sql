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

-- The alert row is the outbox of alert.created (migrations 0306, 0309). A waiting alert with no published_on is still
-- to publish; the relay stamps published_on when it takes the alert, and clears it again when the publish or the
-- decision fails. Safe to run again. The consumer writes alert all the time, so wait at most 5 seconds for its lock.
SET lock_timeout = '5s';

ALTER TABLE alert ADD COLUMN IF NOT EXISTS published_on TIMESTAMPTZ;

-- What the relay publishes next: waiting alerts not yet published, priority tier first, then oldest.
CREATE INDEX IF NOT EXISTS alert_unpublished_idx ON alert (tier DESC, id)
    WHERE processed_on IS NULL AND error_reason IS NULL AND published_on IS NULL;

-- Published and still undecided: the relay's backpressure count and the stale sweep.
CREATE INDEX IF NOT EXISTS alert_in_flight_idx ON alert (published_on)
    WHERE processed_on IS NULL AND error_reason IS NULL AND published_on IS NOT NULL;

COMMENT ON COLUMN alert.published_on IS
    'When the alert relay took this waiting alert to publish alert.created. NULL with processed_on NULL: still to publish '
    '(new, or a failed attempt waiting for next_attempt_on). Cleared again when the publish or the decision fails.';
