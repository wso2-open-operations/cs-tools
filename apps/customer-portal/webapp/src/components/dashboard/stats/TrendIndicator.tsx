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

import { Box, Typography, Skeleton } from "@wso2/oxygen-ui";
import { TrendingUp, TrendingDown } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import ErrorIndicator from "@/components/common/errorIndicator/ErrorIndicator";
import { type TrendData } from "@/models/responses";

interface TrendIndicatorProps {
  trend?: TrendData;
  isLoading?: boolean;
  isError?: boolean;
}

/**
 * Component to display a trend indicator with an icon and value.
 *
 * @param {TrendIndicatorProps} props - Component props.
 * @returns {JSX.Element | null} The rendered component.
 */
export const TrendIndicator = ({
  trend,
  isLoading,
  isError,
}: TrendIndicatorProps): JSX.Element | null => {
  if (!isLoading && !trend && !isError) {
    return null;
  }

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "flex-end",
        gap: 0.5,
      }}
    >
      {/* Loading state */}
      {isLoading ? (
        <>
          <Skeleton variant="rounded" width={45} height={16} />
          <Typography variant="caption" color="text.secondary">
            vs last month
          </Typography>
        </>
      ) : isError ? (
        <>
          <ErrorIndicator entityName="trend" />
          <Typography variant="caption" color="text.secondary">
            vs last month
          </Typography>
        </>
      ) : (
        trend && (
          <>
            {/* Trend value and icon */}
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 0.5,
                color:
                  trend.direction === "down"
                    ? "error.main"
                    : trend.color === "success"
                      ? "success.main"
                      : "text.secondary",
              }}
            >
              {/* Trend icon */}
              {trend.direction === "up" ? (
                <TrendingUp size={16} />
              ) : (
                <TrendingDown size={16} />
              )}
              {/* Trend value */}
              <Typography
                variant="body2"
                component="span"
                fontWeight="bold"
                color="inherit"
              >
                {trend.value}
              </Typography>
            </Box>

            {/* Trend period */}
            <Typography variant="caption" color="text.secondary">
              vs last month
            </Typography>
          </>
        )
      )}
    </Box>
  );
};
