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

-- The alert processor (internal/alertprocessor) folds stored alerts into incidents from the alert.created events the consumer publishes; incident_id marks an alert done. Safe to run again.

-- The processor reads a fingerprint's alerts by its five columns (source, service, metric_name, environment, unique_identifier) to find the incident they fold into.
CREATE INDEX IF NOT EXISTS alert_identity_idx ON alert (source, service, metric_name, environment, unique_identifier, received_on);
