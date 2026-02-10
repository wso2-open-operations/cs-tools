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
import { type JSX } from "react";
import SupportStatGrid from "@components/common/stat-grid/SupportStatGrid";
import { TIME_TRACKING_STAT_CONFIGS } from "@constants/supportConstants";
import type { ProjectTimeTrackingStats } from "@models/responses";

export interface TimeTrackingStatCardsProps {
  isLoading: boolean;
  isError?: boolean;
  stats: ProjectTimeTrackingStats | undefined;
}

/**
 * TimeTrackingStatCards displays a grid of time tracking stats (Total Hours, Billable Hours, Non-Billable) using SupportStatGrid.
 *
 * @param {TimeTrackingStatCardsProps} props - Loading state, error state, and stats data.
 * @returns {JSX.Element} The rendered stat cards grid.
 */
export default function TimeTrackingStatCards({
  isLoading,
  isError,
  stats,
}: TimeTrackingStatCardsProps): JSX.Element {
  return (
    <Box sx={{ mb: 3 }}>
      <SupportStatGrid
        isLoading={isLoading}
        isError={isError}
        entityName="time tracking"
        configs={TIME_TRACKING_STAT_CONFIGS}
        stats={stats ?? undefined}
        spacing={3}
        itemSize={{ xs: 12, md: 4 }}
      />
    </Box>
  );
}
