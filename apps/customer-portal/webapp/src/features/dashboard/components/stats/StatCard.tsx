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
  Typography,
  Card,
  Tooltip,
  Skeleton,
  useTheme,
  alpha,
} from "@wso2/oxygen-ui";
import { Info } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type KeyboardEvent } from "react";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { TrendIndicator } from "@features/dashboard/components/stats/TrendIndicator";
import { type StatCardColor } from "@features/dashboard/types/dashboard";
import type { StatCardProps } from "@features/dashboard/types/stats";

/**
 * Component to display a single statistic card.
 *
 * @param {StatCardProps} props - Component props.
 * @returns {JSX.Element} The rendered component.
 */
export const StatCard = ({
  label,
  value,
  icon,
  iconColor,
  tooltipText,
  trend,
  showTrend = true,
  isLoading,
  isError,
  isTrendError,
  onClick,
}: StatCardProps): JSX.Element => {
  const theme = useTheme();

  const handleKeyDown = onClick
    ? (e: KeyboardEvent<HTMLDivElement>) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick();
        }
      }
    : undefined;

  return (
    <Card
      onClick={onClick}
      onKeyDown={handleKeyDown}
      {...(onClick && {
        role: "button",
        tabIndex: 0,
        "aria-label": label,
      })}
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 3,
        p: 2.5,
        height: "100%",
        ...(onClick && {
          cursor: "pointer",
          transition: "box-shadow 0.2s ease, transform 0.15s ease",
          "&:hover": {
            boxShadow: `0 0 0 1px ${theme.palette.primary.main}, 0 4px 16px rgba(0,0,0,0.12)`,
            transform: "translateY(-2px)",
          },
          "&:focus-visible": {
            outline: `2px solid ${theme.palette.primary.main}`,
            outlineOffset: 2,
          },
        }),
      }}
    >
      {/* Icon and trend indicator */}
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
        }}
      >
        {/* Icon */}
        <Box
          sx={{
            p: 1,
            borderRadius: "50%",
            bgcolor: alpha(
              theme.palette[iconColor as StatCardColor].light,
              0.1,
            ),
            color: theme.palette[iconColor as StatCardColor].light,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          {icon}
        </Box>

        {/* Trend indicator - hidden when showTrend is false */}
        {showTrend && (
          <TrendIndicator
            trend={trend}
            isLoading={isLoading}
            isError={isTrendError}
          />
        )}
      </Box>

      {/* Value and label */}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
        {/* Value */}
        <Typography variant="h4">
          {isLoading ? (
            <Skeleton variant="text" width="40%" height={28} />
          ) : isError ? (
            <ErrorIndicator entityName={label.toLowerCase()} />
          ) : (
            (value ?? "N/A")
          )}
        </Typography>

        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
          {/* Label */}
          <Typography variant="body2">{label}</Typography>

          {/* Tooltip */}
          {tooltipText && (
            <Tooltip title={tooltipText} arrow placement="bottom">
              <Info size={14} />
            </Tooltip>
          )}
        </Box>
      </Box>
    </Card>
  );
};
