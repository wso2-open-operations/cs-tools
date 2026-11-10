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

-- Dismissed standard change suggestions (GET /standard-change-suggestions). A
-- suggestion is a cluster of one customer's completed change requests, known by
-- its key, the normalised subject its members share; a dismissal hides that
-- cluster for that customer from then on, however many members it gains, until
-- someone restores it (the row is deleted). Who dismissed it and why are kept
-- with it. One row per customer and key.

CREATE TABLE IF NOT EXISTS standard_change_suggestion_dismissal (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id           UUID NOT NULL REFERENCES account(id) ON DELETE CASCADE,
    suggestion_key       VARCHAR(512) NOT NULL,
    reason               VARCHAR(1000) NOT NULL,
    dismissed_by_user_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    dismissed_by         VARCHAR(255) NOT NULL,
    dismissed_on         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT standard_change_suggestion_dismissal_key UNIQUE (account_id, suggestion_key)
);
