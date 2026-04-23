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

import type { CaseDetailsHeaderProps } from "@features/support/types/supportComponents";
import { Box, Chip, Stack, Typography, alpha } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { getSeverityLegendColor } from "@features/dashboard/utils/dashboard";
import {
  formatValue,
  hasSeverityLabelForChip,
  mapSeverityToDisplay,
} from "@features/support/utils/support";
import { CaseDetailsHeaderSkeleton } from "@case-details/CaseDetailsSkeleton";

/**
 * Case details header: case number, optional severity, optional status chip, and title.
 *
 * @param {CaseDetailsHeaderProps} props - Header data and styling.
 * @returns {JSX.Element} The header block.
 */
export default function CaseDetailsHeader({
  caseNumber,
  title,
  severityLabel,
  statusLabel,
  statusChipSx,
  isLoading = false,
  showSeverityChip = true,
  showStatusChip = true,
  variant = "default",
}: CaseDetailsHeaderProps): JSX.Element {
  if (isLoading) {
    return <CaseDetailsHeaderSkeleton variant={variant} />;
  }

  const displaySeverity =
    showSeverityChip && hasSeverityLabelForChip(severityLabel);
  const statusColor =
    typeof statusChipSx === "object" &&
    statusChipSx !== null &&
    "color" in statusChipSx
      ? (statusChipSx.color as string)
      : undefined;

  return (
    <Box>
      <Stack
        direction="row"
        spacing={1.5}
        alignItems="center"
        sx={{ mb: 0.5, flexWrap: "wrap" }}
      >
        <Typography variant="body2" fontWeight={500} color="text.primary">
          {formatValue(caseNumber)}
        </Typography>
        {displaySeverity && (
          <Chip
            label={mapSeverityToDisplay(severityLabel ?? undefined)}
            size="small"
            variant="outlined"
            sx={{
              bgcolor: alpha(
                getSeverityLegendColor(severityLabel ?? undefined),
                0.1,
              ),
              color: getSeverityLegendColor(severityLabel ?? undefined),
              borderColor: alpha(
                getSeverityLegendColor(severityLabel ?? undefined),
                0.3,
              ),
              fontWeight: 500,
              px: 0,
              height: 20,
              fontSize: "0.75rem",
              "& .MuiChip-label": {
                pl: "6px",
                pr: "6px",
              },
            }}
          />
        )}
        {showStatusChip && (
          <Stack direction="row" spacing={0.75} alignItems="center">
            <Box
              sx={{
                width: 8,
                height: 8,
                borderRadius: "50%",
                bgcolor: statusColor ?? "text.secondary",
                flexShrink: 0,
              }}
            />
            <Typography variant="caption" color="text.secondary">
              {formatValue(statusLabel)}
            </Typography>
          </Stack>
        )}
      </Stack>
      <Typography variant="h6" color="text.primary" sx={{ fontWeight: 500 }}>
        {formatValue(title)}
      </Typography>
    </Box>
  );
}
