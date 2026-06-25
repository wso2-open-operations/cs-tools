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

import { Box, Button, Chip, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import { useNavigate } from "react-router";
import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";

type SecurityCenterTabId = "security_reports" | "vulnerabilities";

/**
 * Security Center landing — the home for the customer-security entities, split
 * into Security Reports (SRA) / Vulnerabilities tabs. Only the security-report
 * create entry point is live; the report list and the Vulnerabilities tab are
 * awaiting their backend endpoints, so they render "coming soon" placeholders.
 * Replaces the previous blanket coming-soon page.
 */
export default function CsmSecurityCenterPage(): JSX.Element {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] =
    useState<SecurityCenterTabId>("security_reports");

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box>
        <Typography variant="h5">Security Center</Typography>
        <Typography variant="body2" color="text.secondary">
          Security reports and vulnerability posture across customer deployments.
        </Typography>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={activeTab}
          onChange={(_, v) => setActiveTab(v as SecurityCenterTabId)}
        >
          <Tab value="security_reports" label="Security reports" />
          <Tab value="vulnerabilities" label="Vulnerabilities" />
        </Tabs>
      </Box>

      {activeTab === "security_reports" && (
        <CsmIssuesView
          entityNoun="security reports"
          lockedFilters={{ caseTypes: ["security_report_analysis"] }}
          hideTypeFilter
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

      {activeTab === "vulnerabilities" && (
        <TabPanel
          description="Vulnerability posture across customer deployments, with response-time tracking."
          comingSoon
        />
      )}
    </Box>
  );
}

interface TabPanelProps {
  description: string;
  action?: ReactNode;
  comingSoon?: boolean;
  children?: ReactNode;
}

function TabPanel({
  description,
  action,
  comingSoon,
  children,
}: TabPanelProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1.5,
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="body2" color="text.secondary">
            {description}
          </Typography>
          {comingSoon && (
            <Chip
              size="small"
              label="Coming soon"
              color="warning"
              variant="outlined"
            />
          )}
        </Box>
        {action}
      </Box>
      {children}
    </Box>
  );
}
