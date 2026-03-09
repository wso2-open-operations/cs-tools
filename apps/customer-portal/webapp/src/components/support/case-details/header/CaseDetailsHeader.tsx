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

import { Box, Chip, Stack, Typography, alpha } from "@wso2/oxygen-ui";
import { type ReactElement, type ReactNode, type JSX } from "react";
import { getSeverityLegendColor } from "@constants/dashboardConstants";
import { formatValue, mapSeverityToDisplay } from "@utils/support";
import { CaseDetailsHeaderSkeleton } from "@case-details/CaseDetailsSkeleton";

export interface CaseDetailsHeaderProps {
  caseNumber: string | null | undefined;
  title: string | null | undefined;
  severityLabel: string | null | undefined;
  statusLabel: string | null | undefined;
  statusChipIcon: ReactNode;
  statusChipSx: Record<string, unknown>;
  isLoading?: boolean;
}

/**
 * Case details header: case number, severity, status chip, and title.
 *
 * @param {CaseDetailsHeaderProps} props - Header data and styling.
 * @returns {JSX.Element} The header block.
 */
export default function CaseDetailsHeader({
  caseNumber,
  title,
  severityLabel,
  statusLabel,
  statusChipIcon,
  statusChipSx,
  isLoading = false,
}: CaseDetailsHeaderProps): JSX.Element {
  if (isLoading) {
    return <CaseDetailsHeaderSkeleton />;
  }

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
        <Chip
          size="small"
          variant="outlined"
          label={formatValue(statusLabel)}
          icon={statusChipIcon as ReactElement}
          sx={statusChipSx}
        />
      </Stack>
      <Typography variant="h6" color="text.primary" sx={{ fontWeight: 500 }}>
        {formatValue(title)}
      </Typography>
    </Box>
  );
}
