// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

/**
 * Whether attachments use entity-service's stored-attachment flow (SFTPGo
 * shares for upload and download, a streamed route for comment images) in
 * this deployment.
 *
 * Read from the runtime `window.config.CSM_PORTAL_ENTITY_ATTACHMENTS_ENABLED`
 * key. On is the default: a missing key or any other value keeps the flow on.
 * Only `false` — the boolean, or the string `"false"` for platforms that inject
 * config as strings — turns it off and restores the older ServiceNow flow.
 *
 * Read at call time (not module load) so a test can set `window.config` per
 * case; the value is fixed for a real page load anyway.
 */
export function isEntityAttachmentsEnabled(): boolean {
  const raw = window.config?.CSM_PORTAL_ENTITY_ATTACHMENTS_ENABLED;
  return !(raw === false || (typeof raw === "string" && raw.trim().toLowerCase() === "false"));
}
