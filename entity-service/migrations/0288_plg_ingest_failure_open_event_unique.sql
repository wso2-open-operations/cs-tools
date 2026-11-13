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

-- At most one open failure per event id, so concurrent records of the same
-- failing event cannot both land.

-- Earlier duplicates: keep the oldest open row of each event, resolve the rest.
UPDATE plg_ingest_failure f
SET    resolved_on = NOW()
WHERE  f.resolved_on IS NULL AND f.event_id IS NOT NULL
  AND  EXISTS (SELECT 1 FROM plg_ingest_failure o
               WHERE  o.event_id = f.event_id AND o.resolved_on IS NULL
                 AND  (o.created_at, o.id) < (f.created_at, f.id));

CREATE UNIQUE INDEX IF NOT EXISTS uq_plg_ingest_failure_open_event
    ON plg_ingest_failure (event_id)
    WHERE event_id IS NOT NULL AND resolved_on IS NULL;
