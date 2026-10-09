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
 * Display label for each portal role key `GET /roles/grantable` can report.
 * Keyed by the same stable vocabulary `usePortalAccess`/the backend's own
 * `AccessGuard` use ("cs_engineer", "escalator", ...) — never the real
 * identity-provider role name or id behind it, which the backend never
 * exposes to this frontend at all.
 *
 * A key with no entry here still renders (falls back to the raw key in
 * `AddUserDialog.tsx`) rather than being silently dropped, so a newly
 * configured role is visible immediately even before this map is updated to
 * give it a proper label.
 */
export const GRANTABLE_ROLE_LABELS: Record<string, string> = {
  viewer: "Viewer",
  escalator: "Escalator",
  attachment_downloader: "Attachment Downloader",
  usage_metrics_viewer: "Usage Metrics Viewer",
  cs_engineer: "CS Engineer",
  admin: "Admin",
  timecard_approver: "Time Card Approver",
  dashboard_designer: "Dashboard Designer",
  sales_solutions: "Sales Solutions",
  worknote_creator: "Work Note Creator",
  announcement_creator: "Announcement Creator",
};

/** Falls back to the raw key, title-cased on underscores, for a role this
 * map doesn't yet have a proper label for. */
export function grantableRoleLabel(key: string): string {
  if (GRANTABLE_ROLE_LABELS[key]) return GRANTABLE_ROLE_LABELS[key];
  return key
    .split("_")
    .map((part) => (part ? part[0].toUpperCase() + part.slice(1) : part))
    .join(" ");
}
