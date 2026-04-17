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
import type { JSX } from "react";
import type { UsageChartSurfaceProps } from "@features/usage-metrics/types/usageMetrics";

/**
 * Full-width chart area with focus rings suppressed (OutstandingIncidents-style shell).
 *
 * @param children - LineChart or other chart output.
 * @param minHeight - Optional fixed height for the chart area.
 * @returns {JSX.Element} Wrapper for usage metric charts.
 */
export function UsageChartSurface({
  children,
  minHeight,
}: UsageChartSurfaceProps): JSX.Element {
  return (
    <Box
      sx={{
        width: "100%",
        position: "relative",
        ...(minHeight != null ? { minHeight, height: minHeight } : {}),
        "& *:focus": { outline: "none" },
        "& .recharts-responsive-container": {
          width: "100% !important",
          maxWidth: "100%",
        },
        "& .recharts-surface": { outline: "none" },
      }}
    >
      {children}
    </Box>
  );
}
