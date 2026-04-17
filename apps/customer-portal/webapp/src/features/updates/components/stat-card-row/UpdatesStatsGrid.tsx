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

import {
  Box,
  Chip,
  Grid,
  Skeleton,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { type JSX } from "react";
import {
  UPDATES_SECURITY_PENDING_ACTION_CHIP_LABEL,
  UPDATES_STATS,
  UPDATES_STATS_GRID_SECTION_TITLE,
} from "@features/updates/constants/updatesConstants";
import {
  UpdatesStatKey,
  type UpdatesStatConfigItem,
  type UpdatesStats,
  type UpdatesStatsGridProps,
} from "@features/updates/types/updates";
import {
  aggregateUpdateStats,
  getStatTooltipText,
  getStatValue,
} from "@features/updates/utils/updates";
import { StatCard } from "@features/updates/components/stat-card-row/StatCard";

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
  const theme = useTheme();
  const aggregatedData = aggregateUpdateStats(data);
  const isEffectiveLoading = isLoading || (!data && !isError);

  const renderCountWithSkeleton = (
    count: number | undefined,
    width: number = 24,
  ): JSX.Element | number => {
    if (isEffectiveLoading || count === undefined) {
      return (
        <Skeleton
          variant="text"
          width={width}
          height={20}
          sx={{ display: "inline-block", verticalAlign: "middle", mx: 0.5 }}
        />
      );
    }
    return count;
  };

  const renderExtraContent = (
    stat: UpdatesStatConfigItem,
  ): JSX.Element | undefined => {
    if (isError) {
      return undefined;
    }

    switch (stat.id) {
      case UpdatesStatKey.TotalUpdatesInstalled:
        if (
          isEffectiveLoading ||
          aggregatedData?.totalUpdatesInstalledBreakdown
        ) {
          const { regular, security } =
            aggregatedData?.totalUpdatesInstalledBreakdown || {};
          return (
            <Box sx={{ display: "flex", gap: 0.5, alignItems: "center" }}>
              <Typography variant="caption" color="text.secondary">
                {renderCountWithSkeleton(regular)} Regular •{" "}
                {renderCountWithSkeleton(security)} Security
              </Typography>
            </Box>
          );
        }
        return undefined;
      case UpdatesStatKey.TotalUpdatesPending:
        if (
          isEffectiveLoading ||
          aggregatedData?.totalUpdatesPendingBreakdown
        ) {
          const { regular, security } =
            aggregatedData?.totalUpdatesPendingBreakdown || {};
          return (
            <Box sx={{ display: "flex", gap: 0.5, alignItems: "center" }}>
              <Typography variant="caption" color="text.secondary">
                {renderCountWithSkeleton(regular)} Regular •{" "}
                {renderCountWithSkeleton(security)} Security
              </Typography>
            </Box>
          );
        }
        return undefined;
      case UpdatesStatKey.SecurityUpdatesPending: {
        const securityPending = aggregatedData?.securityUpdatesPending;

        if (isEffectiveLoading) {
          return (
            <Skeleton
              variant="rectangular"
              width={90}
              height={20}
              sx={{ borderRadius: 1, mt: 0.5 }}
            />
          );
        }

        if (securityPending && securityPending > 0) {
          const resolvedColor = theme.palette.error.main;
          return (
            <Chip
              size="small"
              variant="outlined"
              label={UPDATES_SECURITY_PENDING_ACTION_CHIP_LABEL}
              sx={{
                bgcolor: alpha(resolvedColor, 0.1),
                color: resolvedColor,
                borderColor: alpha(resolvedColor, 0.3),
                height: 20,
                fontSize: "0.75rem",
                fontWeight: 500,
              }}
            />
          );
        }
        return undefined;
      }
      default:
        return undefined;
    }
  };

  return (
    <Box>
      <Typography variant="h6" gutterBottom sx={{ mb: 2 }}>
        {UPDATES_STATS_GRID_SECTION_TITLE}
      </Typography>
      <Grid container spacing={2}>
        {UPDATES_STATS.map((stat) => {
          const Icon = stat.icon;
          const value = getStatValue(
            aggregatedData,
            stat.id as keyof UpdatesStats,
          );

          return (
            <Grid key={stat.id} size={{ xs: 12, sm: 6, md: 3 }}>
              <StatCard
                label={stat.label}
                value={value}
                icon={<Icon size={20} />}
                iconColor={stat.iconColor}
                tooltipText={getStatTooltipText(stat, aggregatedData)}
                isLoading={isEffectiveLoading}
                isError={isError}
                extraContent={renderExtraContent(stat)}
              />
            </Grid>
          );
        })}
      </Grid>
    </Box>
  );
}
