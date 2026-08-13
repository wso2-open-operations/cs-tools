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

import { Box, Tab, Tabs } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import ConversationsTab from "@features/csm-projects/components/ConversationsTab";
import { usePathTabs } from "@hooks/useSectionTabs";

type WorkItemSubTab =
  | "cases"
  | "service-requests"
  | "security-reports"
  | "engagements"
  | "conversations";

// Every known sub-tab id, for `usePathTabs` to validate an incoming URL
// segment against.
const WORK_ITEM_SUB_TAB_IDS: readonly WorkItemSubTab[] = [
  "cases",
  "service-requests",
  "security-reports",
  "engagements",
  "conversations",
];

interface WorkItemsTabProps {
  projectId: string;
  /** Resolved base path for this sub-tab strip, e.g.
   * `/customers/projects/:id/work-items` — supplied by the parent page,
   * which owns the `:id` segment (same convention as `usePathTabs` itself). */
  basePath: string;
}

/**
 * A project's work items, categorized into per-type sub-tabs rather than one
 * mixed list with a type filter — Cases / Service requests / Security reports
 * / Engagements each lock `CsmIssuesView` to their own case type (mirroring
 * the props each type's own standalone page already uses — `CsmCasesPage`,
 * `OperationsPage`, `CsmSecurityCenterPage`, `CsmEngagementsPage` — so a row's
 * severity column, detail route, and engagement-type filter all match what a
 * user sees on that type's dedicated page). Conversations is the project's
 * chat sessions (`ConversationsTab`), included here as its own sub-tab rather
 * than as a separate top-level project tab.
 */
export default function WorkItemsTab({
  projectId,
  basePath,
}: WorkItemsTabProps): JSX.Element {
  // Sub-tab is a real URL path segment (`${basePath}/<subTab>`, i.e.
  // `/customers/projects/:id/work-items/:tab?`) so a link to a specific
  // sub-tab is shareable/bookmarkable, same convention as the parent page's
  // own top-level tabs.
  const { activeTab: subTab, setActiveTab: setSubTab } =
    usePathTabs<WorkItemSubTab>(basePath, WORK_ITEM_SUB_TAB_IDS, "cases");

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Tabs value={subTab} onChange={(_, v) => setSubTab(v as WorkItemSubTab)}>
        <Tab value="cases" label="Cases" />
        <Tab value="service-requests" label="Service requests" />
        <Tab value="security-reports" label="Security reports" />
        <Tab value="engagements" label="Engagements" />
        <Tab value="conversations" label="Chats" />
      </Tabs>

      {subTab === "cases" && (
        <CsmIssuesView
          entityNoun="cases"
          lockedFilters={{ projects: [projectId], caseTypes: ["case"] }}
          hideProjectFilter
          hideTypeFilter
        />
      )}

      {subTab === "service-requests" && (
        <CsmIssuesView
          entityNoun="service requests"
          lockedFilters={{ projects: [projectId], caseTypes: ["service_request"] }}
          hideProjectFilter
          hideTypeFilter
          detailBasePath="/operations/service-requests"
        />
      )}

      {subTab === "security-reports" && (
        <CsmIssuesView
          entityNoun="security reports"
          lockedFilters={{ projects: [projectId], caseTypes: ["security_report_analysis"] }}
          hideProjectFilter
          hideTypeFilter
          hideSeverityColumn
          detailBasePath="/security-center/security-reports"
        />
      )}

      {subTab === "engagements" && (
        <CsmIssuesView
          entityNoun="engagements"
          lockedFilters={{ projects: [projectId], caseTypes: ["engagement"] }}
          hideProjectFilter
          hideTypeFilter
          showEngagementTypeFilter
          hideSeverityColumn
          detailBasePath="/engagements"
        />
      )}

      {subTab === "conversations" && <ConversationsTab projectId={projectId} />}
    </Box>
  );
}
