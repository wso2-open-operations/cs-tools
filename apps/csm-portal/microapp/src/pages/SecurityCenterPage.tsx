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

import { useState } from "react";
import { Stack, Tab, Tabs } from "@wso2/oxygen-ui";
import { SecurityReportsTab } from "@components/security-center/SecurityReportsTab";
import { VulnerabilitiesTab } from "@components/security-center/VulnerabilitiesTab";

type SecurityCenterTabId = "security_reports" | "vulnerabilities";

const TABS: { id: SecurityCenterTabId; label: string }[] = [
  { id: "security_reports", label: "Security reports" },
  { id: "vulnerabilities", label: "Vulnerabilities" },
];

// Mirrors the webapp's CsmSecurityCenterPage: Security reports (cases of type
// "security_report_analysis") | Vulnerabilities (a separate, non-case-backed
// entity). "New security report" lives behind SecurityReportsTab's own Fab
// (see NewSecurityReportPage.tsx) — Vulnerabilities stays read-only, there's no
// create flow for that entity in either app.
export default function SecurityCenterPage() {
  const [activeTab, setActiveTab] = useState<SecurityCenterTabId>("security_reports");

  return (
    <Stack gap={2}>
      <Tabs variant="scrollable" value={activeTab} onChange={(_, value: SecurityCenterTabId) => setActiveTab(value)}>
        {TABS.map((tab) => (
          <Tab key={tab.id} label={tab.label} value={tab.id} disableRipple />
        ))}
      </Tabs>

      {activeTab === "security_reports" && <SecurityReportsTab />}
      {activeTab === "vulnerabilities" && <VulnerabilitiesTab />}
    </Stack>
  );
}
