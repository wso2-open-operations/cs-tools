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

import { useCallback, useMemo, useState, type JSX } from "react";
import { useParams, useSearchParams } from "react-router";
import { useModifierAwareNavigate } from "@hooks/useModifierAwareNavigate";
import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import useGetProjectFilters from "@api/useGetProjectFilters";
import SecurityStats from "@features/security/components/SecurityStats";
import TabBar from "@components/tab-bar/TabBar";
import ProductVulnerabilitiesTable from "@features/security/components/ProductVulnerabilitiesTable";
import SecurityReportAnalysis from "@features/security/components/SecurityReportAnalysis";
import { SECURITY_PAGE_TABS } from "@features/security/constants/securityConstants";
import { SecurityStatKey, SecurityTabId } from "@features/security/types/security";
import { parseSecurityTabQueryParam } from "@features/security/utils/securityPage";
import { getProjectPermissions } from "@utils/permission";
import { CaseStatus } from "@features/support/constants/supportConstants";
import { getLast30DaysUtcRange } from "@features/support/utils/support";

const SECURITY_STAT_FILTER_INFO: Record<SecurityStatKey, { title: string; subtitle: string }> = {
  [SecurityStatKey.activeSecurityReports]: {
    title: "Outstanding Security Reports",
    subtitle: "Security reports without closed state",
  },
  [SecurityStatKey.resolvedSecurityReports]: {
    title: "Resolved Security Reports (Last 30d)",
    subtitle: "Security reports in closed state",
  },
  [SecurityStatKey.totalVulnerabilities]: {
    title: "All Security Reports",
    subtitle: "All security reports",
  },
};

const SecurityPage = (): JSX.Element => {
  const navigate = useModifierAwareNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const { data: projectDetails } = useGetProjectDetails(projectId || "");
  const { data: projectFeatures, isLoading: isProjectFeaturesLoading } =
    useGetProjectFeatures(projectId || "");
  const { data: filterMetadata } = useGetProjectFilters(projectId || "");
  const [fixedStatusIds, setFixedStatusIds] = useState<number[] | undefined>(undefined);
  const [activeStatKey, setActiveStatKey] = useState<SecurityStatKey | undefined>(undefined);
  const [fixedClosedDateRange, setFixedClosedDateRange] = useState<
    { closedStartDate: string; closedEndDate: string } | undefined
  >(undefined);

  const isStatFiltered = fixedStatusIds !== undefined;

  const clearStatFilter = () => {
    setFixedStatusIds(undefined);
    setActiveStatKey(undefined);
    setFixedClosedDateRange(undefined);
  };

  const tabParam = searchParams.get("tab");
  const rawActiveTab = parseSecurityTabQueryParam(tabParam);
  const areFeaturePermissionsReady =
    !isProjectFeaturesLoading && projectFeatures !== undefined;

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

  const handleStatCardClick = useCallback(
    (key: SecurityStatKey) => {
      if (!filterMetadata?.caseStates) return;
      const getStateId = (label: string): number | null => {
        const s = filterMetadata.caseStates!.find((st) => st.label === label);
        return s != null ? Number(s.id) : null;
      };
      let ids: number[] | undefined;
      switch (key) {
        case SecurityStatKey.activeSecurityReports: {
          const closedId = getStateId(CaseStatus.CLOSED);
          ids = filterMetadata.caseStates.map((s) => Number(s.id)).filter((id) => id !== closedId);
          setFixedClosedDateRange(undefined);
          break;
        }
        case SecurityStatKey.resolvedSecurityReports: {
          const id = getStateId(CaseStatus.CLOSED);
          ids = id != null ? [id] : undefined;
          setFixedClosedDateRange(getLast30DaysUtcRange());
          break;
        }
        default:
          ids = undefined;
          setFixedClosedDateRange(undefined);
      }
      setFixedStatusIds(ids);
      setActiveStatKey(key);
      handleTabChange(SecurityTabId.VULNERABILITIES);
    },
    [filterMetadata, handleTabChange],
  );

  const tabs = useMemo(
    () =>
      areFeaturePermissionsReady
        ? SECURITY_PAGE_TABS.filter((tab) =>
            tab.id === SecurityTabId.VULNERABILITIES
              ? getProjectPermissions(projectDetails?.type?.label, {
                  projectFeatures,
                }).hasSecurityReportAnalysis
              : true,
          )
        : SECURITY_PAGE_TABS,
    [areFeaturePermissionsReady, projectDetails?.type?.label, projectFeatures],
  );
  const activeTab = useMemo(() => {
    const hasRawActive = tabs.some((tab) => tab.id === rawActiveTab);
    if (hasRawActive) {
      return rawActiveTab;
    }
    return tabs[0]?.id ?? SecurityTabId.VULNERABILITIES;
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
        return <SecurityReportAnalysis fixedStatusIds={fixedStatusIds} />;
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
      {isStatFiltered ? (
        <Box sx={{ mb: 3 }}>
          <Button
            startIcon={<ArrowLeft size={16} />}
            onClick={clearStatFilter}
            variant="text"
            sx={{ mb: 1 }}
          >
            Back
          </Button>
          <Typography variant="h5" color="text.primary" sx={{ mb: 0.5 }}>
            {activeStatKey ? SECURITY_STAT_FILTER_INFO[activeStatKey].title : ""}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {activeStatKey ? SECURITY_STAT_FILTER_INFO[activeStatKey].subtitle : ""}
          </Typography>
        </Box>
      ) : (
        <SecurityStats onStatClick={handleStatCardClick} />
      )}
      {!isStatFiltered && (
        <TabBar
          tabs={tabs}
          activeTab={activeTab}
          onTabChange={handleTabChange}
        />
      )}
      <Box>
        {isStatFiltered ? (
          <SecurityReportAnalysis
            fixedStatusIds={fixedStatusIds}
            fixedClosedDateRange={fixedClosedDateRange}
          />
        ) : (
          renderTabContent()
        )}
      </Box>
    </Box>
  );
};

export default SecurityPage;
