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

import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { Box, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { ChartLegendProps } from "@/features/dashboard/types/charts";
import { useDarkMode } from "@utils/useDarkMode";
import { DASHBOARD_CHART_DARK_MODE_OPACITY } from "@/features/dashboard/constants/charts";

/**
 * ChartLegend component renders a legend for a chart.
 *
 * @param props - Component props
 * @param props.data - Rows with label, value, and color for each legend row.
 * @returns {JSX.Element} Chart legend.
 */
export const ChartLegend = ({
  data,
  isError,
  showValues = false,
  onItemClick,
}: ChartLegendProps): JSX.Element => {
  const isDarkMode = useDarkMode();
  return (
  <Box
    sx={{
      mt: 2,
      display: "flex",
      flexDirection: "column",
      gap: 1,
    }}
  >
    {data.map((entry) => {
      const isClickable = !!onItemClick && !!entry.id && !isError;
      return (
      <Box
        key={entry.name}
        onClick={isClickable ? () => onItemClick!(entry.id!) : undefined}
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          cursor: isClickable ? "pointer" : "default",
          borderRadius: 0.5,
          px: 0.5,
          mx: -0.5,
          transition: "background-color 0.15s ease",
          ...(isClickable && {
            "&:hover": { bgcolor: "action.hover" },
          }),
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
          {/* Legend icon/box */}
          {isError ? (
            <ErrorIndicator entityName="chart's stats" />
          ) : (
            <Box
              sx={{
                width: 12,
                height: 12,
                borderRadius: "50%",
                bgcolor: entry.color,
                opacity: isDarkMode ? DASHBOARD_CHART_DARK_MODE_OPACITY : 1,
              }}
            />
          )}
          {/* Legend text */}
          <Typography variant="caption" color="text.secondary">
            {entry.name}
          </Typography>
        </Box>
        {showValues && !isError && (
          <Typography variant="caption" fontWeight={600}>
            {entry.value}
          </Typography>
        )}
      </Box>
      );
    })}
  </Box>
  );
};
