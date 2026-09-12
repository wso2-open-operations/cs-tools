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
 * Topic id -> Markdown source, for every Help doc. `csmNavItems.ts`'s `help`
 * node stays the single source of truth for *which topics exist and their
 * order* (sidebar, Quick-nav, route guarding); this map only resolves an id to
 * its content, so it must have an entry for every `help.<id>` child declared
 * there — `helpContent.test.ts` asserts that.
 *
 * Each source is pulled in via Vite's `?raw` suffix, so it's bundled as a
 * plain string at build time rather than fetched at runtime — there is no
 * backend endpoint behind this content.
 */

import overviewMd from "../content/overview.md?raw";
import workspaceBasicsMd from "../content/workspace-basics.md?raw";
import dashboardMd from "../content/dashboard.md?raw";
import supportMd from "../content/support.md?raw";
import operationsMd from "../content/operations.md?raw";
import engagementsMd from "../content/engagements.md?raw";
import securityCenterMd from "../content/security-center.md?raw";
import updatesMd from "../content/updates.md?raw";
import timeCardsMd from "../content/time-cards.md?raw";
import announcementsMd from "../content/announcements.md?raw";
import customersMd from "../content/customers.md?raw";
import peopleAccessMd from "../content/people-access.md?raw";
import settingsMd from "../content/settings.md?raw";

/**
 * Keyed by the topic's *bare* id (the segment after `help.` in its
 * `CsmNavNode.id`, e.g. `"overview"` for `"help.overview"` / `/help/overview`)
 * — the same value `HelpTopicPage` reads out of its `:topicId` route param.
 */
export const HELP_TOPIC_CONTENT: Record<string, string> = {
  overview: overviewMd,
  "workspace-basics": workspaceBasicsMd,
  dashboard: dashboardMd,
  support: supportMd,
  operations: operationsMd,
  engagements: engagementsMd,
  "security-center": securityCenterMd,
  updates: updatesMd,
  "time-cards": timeCardsMd,
  announcements: announcementsMd,
  customers: customersMd,
  "people-access": peopleAccessMd,
  settings: settingsMd,
};
