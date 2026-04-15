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

import { Box } from "@wso2/oxygen-ui";
import { useParams } from "react-router";
import { type JSX } from "react";
import ListStatGrid from "@components/list-view/ListStatGrid";
import { usePostProductVulnerabilitiesSearch } from "@features/security/api/usePostProductVulnerabilitiesSearch";
import { useGetProjectCasesStats } from "@features/dashboard/api/useGetProjectCasesStats";
import { CaseType } from "@features/support/constants/supportConstants";
import { SECURITY_STAT_CONFIGS } from "@features/security/constants/securityConstants";

/**
 * SecurityStats component displays security-related statistics cards.
 *
 * @returns {JSX.Element} The rendered SecurityStats component.
 */
export default function SecurityStats(): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();

  const {
    data: vulnerabilitiesData,
    isLoading: isVulnerabilitiesLoading,
    isError: isVulnerabilitiesError,
  } = usePostProductVulnerabilitiesSearch({
    pagination: {
      offset: 0,
      limit: 10,
    },
  });

  const {
    data: securityReportStats,
    isLoading: isSecurityReportLoading,
    isError: isSecurityReportError,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.SECURITY_REPORT_ANALYSIS],
    enabled: !!projectId,
  });

  const stats = {
    totalVulnerabilities: vulnerabilitiesData?.totalRecords ?? 0,
    activeSecurityReports: securityReportStats?.activeCount ?? 0,
    resolvedSecurityReports:
      securityReportStats?.resolvedCases?.pastThirtyDays ?? 0,
  };

  const isLoading = isVulnerabilitiesLoading || isSecurityReportLoading;
  const isError = isVulnerabilitiesError || isSecurityReportError;

  return (
    <Box>
      <ListStatGrid
        isLoading={isLoading}
        isError={isError}
        entityName="security"
        stats={stats}
        configs={SECURITY_STAT_CONFIGS}
      />
    </Box>
  );
}
