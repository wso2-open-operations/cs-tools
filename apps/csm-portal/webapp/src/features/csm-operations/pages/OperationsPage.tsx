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
import { useLocation, useSearchParams } from "react-router";
import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import ChangeRequestsArea from "@features/csm-operations/components/ChangeRequestsArea";
import IncidentsTab from "@features/csm-operations/components/IncidentsTab";
import ProblemsTab from "@features/csm-operations/components/ProblemsTab";
import OutagesTab from "@features/csm-operations/components/OutagesTab";
import RunbooksTab from "@features/csm-operations/components/RunbooksTab";
import { useNavTransition } from "@hooks/useNavTransition";
import { usePathSectionTabs } from "@hooks/useSectionTabs";
import { changeRequestsView } from "@features/csm-operations/utils/standardChanges";

interface PageHeading {
  title: string;
  subtitle: string;
}

/** The heading of every Operations tab but Standard changes. */
const OPERATIONS_HEADING: PageHeading = {
  title: "Operations",
  subtitle: "Service requests, change requests, incidents, problems, and runbooks across customers.",
};

/**
 * The heading of the Standard changes sub-view of Change requests (its
 * catalogue and its proposals alike). A standard change is a templated change
 * request, so this is about change requests and nothing else: the
 * Operations-wide line (service requests, incidents, problems, runbooks) does
 * not describe it.
 */
const STANDARD_CHANGES_HEADING: PageHeading = {
  title: "Standard changes",
  subtitle: "Pre-authorised change requests, raised from templates the CAB has approved.",
};

/**
 * Operations landing — the home for the managed-cloud operational entities,
 * split into Service Requests / Change Requests / Incidents / Problems tabs.
 * Service requests are case-typed, so they list through the shared issues view;
 * the others each have their own search endpoint and listing. Standard changes
 * (templated change requests) are a sub-view of Change requests
 * (`ChangeRequestsArea`), not a tab of their own; there the page heading is
 * about standard changes (`STANDARD_CHANGES_HEADING`) instead of the
 * Operations-wide one, which names entities they have nothing to do with.
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
  const [searchParams] = useSearchParams();
  const changeRequestsSubView = activeTab === "change-requests" ? changeRequestsView(searchParams) : "list";
  const heading = changeRequestsSubView === "standard" ? STANDARD_CHANGES_HEADING : OPERATIONS_HEADING;

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
        <Typography variant="h5">{heading.title}</Typography>
        <Typography variant="body2" color="text.secondary">
          {heading.subtitle}
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

      {activeTab === "change-requests" && <ChangeRequestsArea />}

      {activeTab === "incidents" && <IncidentsTab />}

      {activeTab === "problems" && <ProblemsTab />}

      {activeTab === "outages" && <OutagesTab />}

      {activeTab === "runbooks" && <RunbooksTab />}
    </Box>
  );
}
