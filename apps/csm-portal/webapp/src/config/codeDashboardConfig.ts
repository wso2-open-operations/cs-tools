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
 * Whether the code approach to dashboard generation (the model writes the
 * dashboard as JavaScript: `/dashboard/my/new-code`) is on for this
 * deployment. It is phase 2 and off by default: phase 1 ships the JSON
 * approach only.
 *
 * Read from the runtime `window.config.DASHBOARD_GENERATE_CODE_ENABLED` key.
 * The backend (`POST /dashboards/generate-code`) and csm-ai-integration-service
 * (`POST /ai/dashboards/generate-code`) are gated by the same name, so one
 * value turns the feature on or off everywhere. Only `true`, or the string
 * `"true"` for platforms that inject config as strings, turns it on; any
 * other value (`"1"`, `"TRUE"`, `"yes"`, unset) is off. Off, the page's
 * route sends to the 404 page and its button is not rendered; on, both are
 * further limited to the users `canGenerateAdvancedDashboards` allows.
 *
 * Read at call time (not module load) so a test can set `window.config` per
 * case; the value is fixed for a real page load anyway.
 */
export function isCodeDashboardGenerationEnabled(): boolean {
  const raw = window.config?.DASHBOARD_GENERATE_CODE_ENABLED;
  return raw === true || (typeof raw === "string" && raw.trim() === "true");
}
