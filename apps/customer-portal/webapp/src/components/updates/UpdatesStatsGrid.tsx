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

import { Box, Grid, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { StatCard } from "@components/dashboard/stats/StatCard";
import { UPDATES_STATS } from "@constants/updatesConstants";
import type { UpdatesStats } from "@models/responses";

const NULL_PLACEHOLDER = "--";

export interface UpdatesStatsGridProps {
  data: UpdatesStats | undefined;
  isLoading: boolean;
  isError: boolean;
}

/**
 * Grid of stat cards for Overall Update Status.
 *
 * @param {UpdatesStatsGridProps} props - Component props.
 * @returns {JSX.Element} The rendered component.
 */
export function UpdatesStatsGrid({
  data,
  isLoading,
  isError,
}: UpdatesStatsGridProps): JSX.Element {
  const getValue = (id: keyof UpdatesStats): string | number => {
    if (!data) return NULL_PLACEHOLDER;
    const val = data[id];
    if (val === null || val === undefined) return NULL_PLACEHOLDER;
    if (typeof val === "object") return NULL_PLACEHOLDER;
    return val as string | number;
  };

  const getTooltipText = (stat: (typeof UPDATES_STATS)[number]): string => {
    if (!data) return stat.tooltipText;
    if (
      stat.id === "totalUpdatesInstalled" &&
      data.totalUpdatesInstalledBreakdown
    ) {
      const { regular, security } = data.totalUpdatesInstalledBreakdown;
      return `${stat.tooltipText} (${regular} Regular • ${security} Security)`;
    }
    if (
      stat.id === "totalUpdatesPending" &&
      data.totalUpdatesPendingBreakdown
    ) {
      const { regular, security } = data.totalUpdatesPendingBreakdown;
      return `${stat.tooltipText} (${regular} Regular • ${security} Security)`;
    }
    return stat.tooltipText;
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom sx={{ mb: 2 }}>
        Overall Update Status
      </Typography>
      <Grid container spacing={2}>
        {UPDATES_STATS.map((stat) => {
          const Icon = stat.icon;
          const value = getValue(stat.id as keyof UpdatesStats);

          return (
            <Grid key={stat.id} size={{ xs: 12, sm: 6, md: 3 }}>
              <StatCard
                label={stat.label}
                value={value}
                icon={<Icon size={20} />}
                iconColor={stat.iconColor}
                tooltipText={getTooltipText(stat)}
                isLoading={isLoading}
                isError={isError}
              />
            </Grid>
          );
        })}
      </Grid>
    </Box>
  );
}
