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
import ProductVulnerabilitiesTab from "@features/csm-security-center/components/ProductVulnerabilitiesTab";
import { useNavTransition } from "@hooks/useNavTransition";
import { usePathSectionTabs } from "@hooks/useSectionTabs";

/**
 * Security Center landing — the home for the customer-security entities, split
 * into Security Reports (SRA) / Vulnerabilities tabs. The active tab is a real
 * path segment (`/security-center/:tab`) so the vulnerability detail page can
 * link back to the right tab, and the selection survives a refresh or share.
 * See `usePathSectionTabs` and the `security-center` routes in App.tsx.
 *
 * Which tabs exist comes from the navigation tree, so a deployment can restrict
 * one through `CSM_PORTAL_FEATURE_OVERRIDES` without touching this page.
 *
 * The tab switch itself lives in the sidebar now (Security Center's
 * submenu), not an in-page strip — `usePathSectionTabs` is kept only for its
 * enabled/WIP-aware fallback reading the same real path segment
 * (`/security-center/:tab`) the sidebar's submenu links navigate to.
 */
export default function CsmSecurityCenterPage(): JSX.Element {
  const { activeKey: activeTab } = usePathSectionTabs(
    "security-center",
    "/security-center",
  );
  const navigate = useNavTransition();
  // Set by a dashboard widget's click-through, since this page has no
  // dashboard context of its own. The security-reports tab renders its own
  // Back button instead (via CsmIssuesView, which reads this same state) —
  // skip here to avoid a duplicate.
  const backState = useLocation().state as { from?: string } | undefined;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {backState?.from && activeTab !== "security-reports" && (
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
        <Typography variant="h5">Security Center</Typography>
        <Typography variant="body2" color="text.secondary">
          Security reports and vulnerability posture across customer deployments.
        </Typography>
      </Box>

      {activeTab === "security-reports" && (
        <CsmIssuesView
          entityNoun="security reports"
          lockedFilters={{ caseTypes: ["security_report_analysis"] }}
          hideTypeFilter
          hideSeverityColumn
          detailBasePath="/security-center/security-reports"
          enableColumnCustomization
          columnsViewId="security-reports"
          actions={
            <Button
              variant="contained"
              color="primary"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => navigate("/security-center/reports/new")}
            >
              New security report
            </Button>
          }
        />
      )}

      {activeTab === "vulnerabilities" && <ProductVulnerabilitiesTab />}
    </Box>
  );
}
