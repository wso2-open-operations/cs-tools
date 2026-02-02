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

import { Box, Grid, StatCard, Skeleton } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { SUPPORT_STAT_CONFIGS } from "@/constants/supportConstants";
import type { ProjectSupportStats } from "@/models/responses";

export interface CasesOverviewStatCardProps {
  isLoading: boolean;
  stats: ProjectSupportStats | undefined;
}

/**
 * CasesOverviewStatCard component to display a grid of support statistics.
 *
 * @param {CasesOverviewStatCardProps} props - The props for the component.
 * @returns {JSX.Element} The rendered CasesOverviewStatCard component.
 */
export default function CasesOverviewStatCard({
  isLoading,
  stats,
}: CasesOverviewStatCardProps): JSX.Element {
  return (
    <Box>
      <Grid
        container
        spacing={2}
        sx={{
          mb: 3,
        }}
      >
        {SUPPORT_STAT_CONFIGS.map((stat) => {
          const SecondaryIcon = stat.secondaryIcon;

          return (
            <Grid
              key={stat.key}
              size={{
                xs: 12,
                sm: 6,
                md: 3,
              }}
              sx={{
                position: "relative",
              }}
            >
              {SecondaryIcon && (
                <Box
                  sx={{
                    opacity: 0.4,
                    position: "absolute",
                    right: 24,
                    top: 20,
                    zIndex: 1,
                  }}
                >
                  <SecondaryIcon />
                </Box>
              )}
              <StatCard
                label={stat.label}
                value={
                  isLoading
                    ? ((
                        <Skeleton variant="text" width="40%" height={24} />
                      ) as unknown as number)
                    : (stats?.[stat.key] ?? 0)
                }
                icon={<stat.icon />}
                iconColor={stat.iconColor}
              />
            </Grid>
          );
        })}
      </Grid>
    </Box>
  );
}
