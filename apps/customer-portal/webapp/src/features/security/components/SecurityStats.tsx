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
import { useGetProjectCasesStats } from "@features/dashboard/api/useGetProjectCasesStats";
import { CaseType } from "@features/support/constants/supportConstants";
import {
  SECURITY_STAT_CONFIGS,
  SECURITY_STATS_ENTITY_NAME,
} from "@features/security/constants/securityConstants";
import { SecurityStatKey } from "@features/security/types/security";

type SecurityStatsProps = {
  onStatClick?: (key: SecurityStatKey) => void;
};

/**
 * SecurityStats component displays security-related statistics cards.
 *
 * @returns {JSX.Element} The rendered SecurityStats component.
 */
export default function SecurityStats({ onStatClick }: SecurityStatsProps): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();

  const {
    data: securityReportStats,
    isLoading,
    isError,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.SECURITY_REPORT_ANALYSIS],
    enabled: !!projectId,
  });

  const stats: Partial<Record<SecurityStatKey, number>> = {
    [SecurityStatKey.activeSecurityReports]:
      securityReportStats?.activeCount ?? 0,
    [SecurityStatKey.resolvedSecurityReports]:
      securityReportStats?.resolvedCases?.pastThirtyDays ?? 0,
  };

  return (
    <Box sx={{ mb: 3 }}>
      <ListStatGrid
        isLoading={isLoading}
        isError={isError}
        entityName={SECURITY_STATS_ENTITY_NAME}
        stats={stats}
        configs={SECURITY_STAT_CONFIGS}
        itemSize={{ xs: 12, sm: 4, md: 4 }}
        onStatClick={onStatClick}
      />
    </Box>
  );
}
