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

-- catalog_item.csm_only marks a catalog item only WSO2 staff may raise, from
-- the CSM portal. Both portals read the same catalog (SearchCatalogs) and the
-- same routing rules, so routing alone can't keep an item away from
-- customers. For a caller that isn't CSM-portal staff (a customer, or staff
-- signed in to the customer portal), entity-service leaves these items out of
-- the catalog search, answers "not found" for their questions, and refuses a
-- service request raised from one (catalog_repo.go, case_service.go).
--
-- Defaults to FALSE, so every existing item stays visible to everyone. Not a
-- ServiceNow field: the sync never writes it.

ALTER TABLE catalog_item ADD COLUMN IF NOT EXISTS csm_only BOOLEAN NOT NULL DEFAULT FALSE;
