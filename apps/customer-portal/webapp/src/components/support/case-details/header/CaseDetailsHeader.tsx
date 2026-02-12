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

import { Box, Chip, Stack, Typography } from "@wso2/oxygen-ui";
import { type ReactElement, type ReactNode, type JSX } from "react";
import { getSeverityColor } from "@utils/casesTable";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import { formatValue } from "@utils/support";
import { CaseDetailsHeaderSkeleton } from "@case-details/CaseDetailsSkeleton";

export interface CaseDetailsHeaderProps {
  caseNumber: string | null | undefined;
  title: string | null | undefined;
  severityLabel: string | null | undefined;
  statusLabel: string | null | undefined;
  statusChipIcon: ReactNode;
  statusChipSx: Record<string, unknown>;
  isError: boolean;
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
  isError,
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
        {isError ? (
          <ErrorIndicator entityName="case details" size="small" />
        ) : (
          <Typography variant="body2" fontWeight={500} color="text.primary">
            {formatValue(caseNumber)}
          </Typography>
        )}
        {isError ? (
          <ErrorIndicator entityName="case details" size="small" />
        ) : (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 0.5,
            }}
          >
            <Box
              sx={{
                width: 8,
                height: 8,
                borderRadius: "50%",
                bgcolor: getSeverityColor(severityLabel ?? undefined),
              }}
            />
            <Typography variant="caption" color="text.secondary">
              {formatValue(severityLabel)}
            </Typography>
          </Box>
        )}
        {isError ? (
          <ErrorIndicator entityName="case details" size="small" />
        ) : (
          <Chip
            size="small"
            variant="outlined"
            label={formatValue(statusLabel)}
            icon={statusChipIcon as ReactElement}
            sx={statusChipSx}
          />
        )}
      </Stack>
      {isError ? (
        <ErrorIndicator entityName="case details" size="small" />
      ) : (
        <Typography variant="h6" color="text.primary" sx={{ fontWeight: 500 }}>
          {formatValue(title)}
        </Typography>
      )}
    </Box>
  );
}
