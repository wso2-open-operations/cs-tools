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

import type { CasesOverviewStatCardProps } from "@features/support/types/supportComponents";
import { Box } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import ListStatGrid from "@components/list-view/ListStatGrid";
import { SUPPORT_STAT_CONFIGS } from "@features/support/constants/supportConstants";

/**
 * CasesOverviewStatCard component to display a grid of support statistics.
 *
 * @param {CasesOverviewStatCardProps} props - The props for the component.
 * @returns {JSX.Element} The rendered CasesOverviewStatCard component.
 */
export default function CasesOverviewStatCard({
  isLoading,
  isError,
  stats,
}: CasesOverviewStatCardProps): JSX.Element {
  return (
    <Box>
      <ListStatGrid
        isLoading={isLoading}
        isError={isError}
        entityName="support"
        stats={stats}
        configs={SUPPORT_STAT_CONFIGS}
      />
    </Box>
  );
}
