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
import { type JSX, useEffect } from "react";
import { useLocation, useParams, useSearchParams } from "react-router";
import SectionTabs from "@components/section-tabs/SectionTabs";
import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import ChangeRequestsTab from "@features/csm-operations/components/ChangeRequestsTab";
import IncidentsTab from "@features/csm-operations/components/IncidentsTab";
import ProblemsTab from "@features/csm-operations/components/ProblemsTab";
import { useNavTransition } from "@hooks/useNavTransition";
import { usePathSectionTabs } from "@hooks/useSectionTabs";

const BASE_PATH = "/operations";

/**
 * Operations landing — the home for the managed-cloud operational entities,
 * split into Service Requests / Change Requests / Incidents / Problems tabs.
 * Service requests are case-typed, so they list through the shared issues view;
 * the others each have their own search endpoint and listing.
 *
 * Which tabs exist comes from the navigation tree, so a deployment can restrict
 * one through `CSM_PORTAL_FEATURE_OVERRIDES` without touching this page.
 */
export default function OperationsPage(): JSX.Element {
  // Active tab lives in the URL path (`/operations/:tab`) so the
  // change-request detail page can link back to the right tab, and the tab
  // survives a refresh / share.
  const tabs = usePathSectionTabs("operations", BASE_PATH);
  const activeTab = tabs.activeKey;
  const navigate = useNavTransition();
  // Set by a dashboard widget's click-through (see DashboardWidgetTile /
  // widgetListConfig.tsx), since this page has no dashboard context of its
  // own. The service_requests tab renders its own Back button instead (via
  // CsmIssuesView, which reads this same state) — skip here to avoid a
  // duplicate.
  const backState = useLocation().state as { from?: string } | undefined;

  // Legacy `?tab=` links (this page's URL shape before the path-segment
  // migration) still land here on the tab-less "operations" route — send them
  // on to the equivalent `/operations/<key>` path so a bookmark or an old
  // share link doesn't silently lose the selection. Only fires when there is
  // no path-segment tab already, so it can't fight a real navigation.
  const { tab: routeTab } = useParams<{ tab?: string }>();
  const [searchParams] = useSearchParams();
  const legacyTab = routeTab ? null : searchParams.get("tab");
  useEffect(() => {
    if (legacyTab) navigate(`${BASE_PATH}/${legacyTab}`, { replace: true });
  }, [legacyTab, navigate]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {backState?.from && activeTab !== "service_requests" && (
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

      <SectionTabs {...tabs} ariaLabel="Operations tabs" />

      {activeTab === "service_requests" && (
        <CsmIssuesView
          entityNoun="service requests"
          lockedFilters={{ caseTypes: ["service_request"] }}
          hideTypeFilter
          detailBasePath="/operations/service-requests"
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

      {activeTab === "change_requests" && <ChangeRequestsTab />}

      {activeTab === "incidents" && <IncidentsTab />}

      {activeTab === "problems" && <ProblemsTab />}
    </Box>
  );
}
