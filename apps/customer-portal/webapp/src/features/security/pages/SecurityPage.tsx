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

import { useCallback, useMemo, type JSX } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { Box } from "@wso2/oxygen-ui";
import useGetProjectDetails from "@api/useGetProjectDetails";
import SecurityStats from "@features/security/components/SecurityStats";
import TabBar from "@components/tab-bar/TabBar";
import ProductVulnerabilitiesTable from "@features/security/components/ProductVulnerabilitiesTable";
import SecurityReportAnalysis from "@features/security/components/SecurityReportAnalysis";
import { SECURITY_PAGE_TABS } from "@features/security/constants/securityConstants";
import { SecurityTabId } from "@features/security/types/security";
import { parseSecurityTabQueryParam } from "@features/security/utils/securityPage";
import { getProjectPermissions } from "@utils/permission";

const SecurityPage = (): JSX.Element => {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const { data: projectDetails } = useGetProjectDetails(projectId || "");

  const tabParam = searchParams.get("tab");
  const rawActiveTab = parseSecurityTabQueryParam(tabParam);
  const permissions = getProjectPermissions(projectDetails?.type?.label, {
    hasPdpSubscription: projectDetails?.hasPdpSubscription,
  });

  const handleTabChange = useCallback(
    (tabId: string) => {
      const nextParams = new URLSearchParams(window.location.search);
      nextParams.set("tab", tabId);
      setSearchParams(nextParams, { replace: true });
    },
    [setSearchParams],
  );

  const handleVulnerabilityClick = useCallback(
    (vulnerability: { id: string }) => {
      navigate(`/projects/${projectId}/security-center/${vulnerability.id}`);
    },
    [navigate, projectId],
  );

  const tabs = useMemo(
    () =>
      SECURITY_PAGE_TABS.filter((tab) =>
        tab.id === SecurityTabId.VULNERABILITIES
          ? permissions.hasSecurityReportAnalysis
          : true,
      ),
    [permissions.hasSecurityReportAnalysis],
  );
  const activeTab = useMemo(() => {
    const hasRawActive = tabs.some((tab) => tab.id === rawActiveTab);
    if (hasRawActive) {
      return rawActiveTab;
    }
    return tabs[0]?.id ?? SecurityTabId.COMPONENTS;
  }, [rawActiveTab, tabs]);

  const renderTabContent = () => {
    switch (activeTab) {
      case SecurityTabId.COMPONENTS:
        return (
          <ProductVulnerabilitiesTable
            onVulnerabilityClick={handleVulnerabilityClick}
          />
        );
      case SecurityTabId.VULNERABILITIES:
        return <SecurityReportAnalysis />;
      default:
        return (
          <ProductVulnerabilitiesTable
            onVulnerabilityClick={handleVulnerabilityClick}
          />
        );
    }
  };

  return (
    <Box>
      <SecurityStats />

      <Box sx={{ mt: 3 }}>
        <TabBar
          tabs={tabs}
          activeTab={activeTab}
          onTabChange={handleTabChange}
        />

        <Box>
          {/* Using the switch case result here */}
          {renderTabContent()}
        </Box>
      </Box>
    </Box>
  );
};

export default SecurityPage;
