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
import { type JSX } from "react";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import type { StatCardColor } from "@/features/dashboard/constants/dashboard";
import type { StatCardProps } from "@features/updates/types/updates";
import { UPDATES_NULL_PLACEHOLDER } from "@features/updates/constants/updatesConstants";

/**
 * Component to display a single statistic card for updates.
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
  isLoading,
  isError,
  extraContent,
}: StatCardProps): JSX.Element => {
  const theme = useTheme();

  return (
    <Card
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 3,
        p: 2.5,
        height: "100%",
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          mb: 1,
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

        {/* Value */}
        <Box sx={{ display: "flex", alignItems: "center" }}>
          <Typography
            variant="h4"
            sx={{
              fontWeight: 600,
              color: theme.palette[iconColor as StatCardColor].light,
            }}
          >
            {isLoading ? (
              <Skeleton variant="text" width={40} height={32} />
            ) : isError ? (
              <ErrorIndicator entityName={label.toLowerCase()} />
            ) : (
              (value ?? UPDATES_NULL_PLACEHOLDER)
            )}
          </Typography>
        </Box>
      </Box>

      {/* Label and Extra Content */}
      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ fontWeight: 500 }}
          >
            {label}
          </Typography>

          <Tooltip title={tooltipText} arrow placement="bottom">
            <span>
              <Info size={14} style={{ color: theme.palette.text.secondary }} />
            </span>
          </Tooltip>
        </Box>

        {extraContent && <Box sx={{ mt: 0.5 }}>{extraContent}</Box>}
      </Box>
    </Card>
  );
};
