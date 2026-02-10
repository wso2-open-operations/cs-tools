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
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import { type SupportStatConfig } from "@constants/supportConstants";

export interface SupportStatGridProps {
  isLoading: boolean;
  configs: SupportStatConfig<any>[];
  stats: any;
  isError?: boolean;
  entityName?: string;
}

/**
 * SupportStatGrid component to display a grid of support statistics.
 *
 * @param {SupportStatGridProps} props - The props for the component.
 * @returns {JSX.Element} The rendered SupportStatGrid component.
 */
export default function SupportStatGrid({
  isLoading,
  configs,
  stats,
  isError,
  entityName = "statistics",
}: SupportStatGridProps): JSX.Element {
  return (
    <Grid container spacing={2}>
      {configs.map((stat) => {
        const SecondaryIcon = stat.secondaryIcon;
        const Icon = stat.icon;

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
                isLoading ? (
                  ((
                    <Skeleton
                      data-testid="Skeleton"
                      variant="rounded"
                      width={60}
                      height={24}
                    />
                  ) as any)
                ) : isError ? (
                  <ErrorIndicator entityName={entityName} />
                ) : (
                  (stats?.[stat.key] ?? 0)
                )
              }
              icon={<Icon />}
              iconColor={stat.iconColor}
            />
          </Grid>
        );
      })}
    </Grid>
  );
}
