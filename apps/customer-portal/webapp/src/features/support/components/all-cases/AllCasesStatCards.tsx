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

import type { AllCasesStatCardsProps } from "@features/support/types/supportComponents";
import { Box } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import SupportStatGrid from "@components/stat-grid/SupportStatGrid";
import {
  ALL_CASES_STAT_CONFIGS,
  getAllCasesFlattenedStats,
} from "@features/support/constants/supportConstants";

/**
 * AllCasesStatCards component to display 4 stat cards for case statistics.
 *
 * @param {AllCasesStatCardsProps} props - Loading state and stats data.
 * @returns {JSX.Element} The rendered stat cards grid.
 */
export default function AllCasesStatCards({
  isLoading,
  isError,
  stats,
  statEntityName = "case",
}: AllCasesStatCardsProps): JSX.Element {
  const flattenedStats = getAllCasesFlattenedStats(stats);

  return (
    <Box sx={{ mb: 3 }}>
      <SupportStatGrid
        isLoading={isLoading}
        isError={isError}
        entityName={statEntityName}
        configs={ALL_CASES_STAT_CONFIGS}
        stats={flattenedStats}
      />
    </Box>
  );
}
