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

-- The product-consumption provisioning flow's final step generates two
-- subscription secret keys and stores them on the project alongside the
-- Choreo application's OAuth2 credentials. The project table mirrors the
-- ServiceNow record, which carries both, but had no columns for them — so
-- the dual-write silently discarded step 5's artefacts and every read
-- reported the project as having no secret keys.
--
-- TEXT rather than VARCHAR(255) like client_secret: these hold base64-encoded
-- AES-256-GCM ciphertext, whose length follows the plaintext, and a secret
-- long enough to overflow the column should not fail the write.
ALTER TABLE project
    ADD COLUMN IF NOT EXISTS primary_secret_key TEXT,
    ADD COLUMN IF NOT EXISTS secondary_secret_key TEXT;

COMMENT ON COLUMN project.primary_secret_key IS
    'Base64-encoded AES-256-GCM ciphertext of the primary subscription secret key. Never stored or returned in the clear.';
COMMENT ON COLUMN project.secondary_secret_key IS
    'Base64-encoded AES-256-GCM ciphertext of the secondary subscription secret key. Never stored or returned in the clear.';
