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
 * Whether the customer-onboarding status column on a project's Contacts tab
 * is on for this deployment.
 *
 * Read from the runtime `window.config.CSM_MIGRATION_ONBOARDING_STATUS_ENABLED`
 * key. The same flag name gates the backend route it depends on
 * (`GET /projects/{id}/onboarding-steps`); both must be on for the column to
 * work, and off is the default on both sides so a deployment gets nothing new
 * until it opts in. Only `true` — the boolean, or the string `"true"` for
 * platforms that inject config as strings — turns it on; any other value is
 * off, and off means the column is not rendered and no request is made.
 *
 * Read at call time (not module load, unlike `apiConfig.ts`) so a test can set
 * `window.config` per case; the value is fixed for a real page load anyway.
 */
export function isOnboardingStatusEnabled(): boolean {
  const raw = window.config?.CSM_MIGRATION_ONBOARDING_STATUS_ENABLED;
  return !(raw === false || (typeof raw === "string" && raw.trim().toLowerCase() === "false"));
}
