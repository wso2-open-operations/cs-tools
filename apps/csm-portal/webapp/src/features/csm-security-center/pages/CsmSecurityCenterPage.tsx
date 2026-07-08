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

import { Box, Button, Tab, Tabs, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import {  useSearchParams } from "react-router";
import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import ProductVulnerabilitiesTab from "@features/csm-security-center/components/ProductVulnerabilitiesTab";
import { useNavTransition } from "@hooks/useNavTransition";

type SecurityCenterTabId = "security_reports" | "vulnerabilities";

const TAB_IDS: SecurityCenterTabId[] = ["security_reports", "vulnerabilities"];

/**
 * Security Center landing — the home for the customer-security entities, split
 * into Security Reports (SRA) / Vulnerabilities tabs. The active tab lives in
 * the URL (`?tab=`) so the vulnerability detail page can link back to the right
 * tab, and the selection survives a refresh or share.
 */
export default function CsmSecurityCenterPage(): JSX.Element {
  const navigate = useNavTransition();
  const [searchParams, setSearchParams] = useSearchParams();
  const tabParam = searchParams.get("tab") as SecurityCenterTabId | null;
  const activeTab: SecurityCenterTabId =
    tabParam && TAB_IDS.includes(tabParam) ? tabParam : "security_reports";

  const handleTabChange = (_: unknown, next: SecurityCenterTabId): void => {
    setSearchParams((prev) => {
      prev.set("tab", next);
      return prev;
    });
  };

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
          onChange={handleTabChange}
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

      {activeTab === "vulnerabilities" && <ProductVulnerabilitiesTab />}
    </Box>
  );
}
