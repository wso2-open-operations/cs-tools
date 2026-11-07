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
 * Whether the SLA impact timeline (state, fix ETA, workaround and other
 * SLA-relevant events) is shown beneath the SLA table on a case's SLA tab.
 *
 * Read from the runtime `window.config.CSM_PORTAL_SLA_TIMELINE_ENABLED` key.
 * On by default: an absent key, or any value other than an explicit `false`
 * (the boolean, or the string `"false"` for platforms that inject config as
 * strings), shows it. Only the timeline is affected, never the SLA table.
 *
 * Read at call time (not module load) so a test can set `window.config` per
 * case; the value is fixed for a real page load anyway.
 */
export function isSlaTimelineEnabled(): boolean {
  const raw = window.config?.CSM_PORTAL_SLA_TIMELINE_ENABLED;
  return !(raw === false || (typeof raw === "string" && raw.trim().toLowerCase() === "false"));
}
