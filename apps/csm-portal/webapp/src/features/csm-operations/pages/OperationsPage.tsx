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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Plus } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { useLocation } from "react-router";
import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import ChangeRequestsTab from "@features/csm-operations/components/ChangeRequestsTab";
import IncidentsTab from "@features/csm-operations/components/IncidentsTab";
import ProblemsTab from "@features/csm-operations/components/ProblemsTab";
import { useNavTransition } from "@hooks/useNavTransition";
import { usePathSectionTabs } from "@hooks/useSectionTabs";

/**
 * Operations landing — the home for the managed-cloud operational entities,
 * split into Service Requests / Change Requests / Incidents / Problems tabs.
 * Service requests are case-typed, so they list through the shared issues view;
 * the others each have their own search endpoint and listing.
 *
 * Which tabs exist comes from the navigation tree, so a deployment can restrict
 * one through `CSM_PORTAL_FEATURE_OVERRIDES` without touching this page.
 *
 * The tab switch itself lives in the sidebar now (Operations' submenu), not
 * an in-page strip — `usePathSectionTabs` is kept only for its
 * enabled/WIP-aware fallback (an invalid or restricted `:tab` still resolves
 * to the first usable one) reading the same real path segment
 * (`/operations/:tab`) the sidebar's submenu links navigate to.
 */
export default function OperationsPage(): JSX.Element {
  const { activeKey: activeTab } = usePathSectionTabs("operations", "/operations");
  const navigate = useNavTransition();
  // Set by a dashboard widget's click-through (see DashboardWidgetTile /
  // widgetListConfig.tsx), since this page has no dashboard context of its
  // own. The service_requests tab renders its own Back button instead (via
  // CsmIssuesView, which reads this same state) — skip here to avoid a
  // duplicate.
  const backState = useLocation().state as { from?: string } | undefined;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {backState?.from && activeTab !== "service-requests" && (
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate(backState.from as string)}
          sx={{ alignSelf: "flex-start" }}
        >
          Back
        </Button>
      )}

      <Box>
        <Typography variant="h5">Operations</Typography>
        <Typography variant="body2" color="text.secondary">
          Service requests, change requests, incidents, and problems across customers.
        </Typography>
      </Box>

      {activeTab === "service-requests" && (
        <CsmIssuesView
          entityNoun="service requests"
          lockedFilters={{ caseTypes: ["service_request"] }}
          hideTypeFilter
          detailBasePath="/operations/service-requests"
          enableColumnCustomization
          columnsViewId="service-requests"
          actions={
            <Button
              variant="contained"
              color="primary"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => navigate("/operations/service-requests/new")}
            >
              Create service request
            </Button>
          }
        />
      )}

      {activeTab === "change-requests" && <ChangeRequestsTab />}

      {activeTab === "incidents" && <IncidentsTab />}

      {activeTab === "problems" && <ProblemsTab />}
    </Box>
  );
}
