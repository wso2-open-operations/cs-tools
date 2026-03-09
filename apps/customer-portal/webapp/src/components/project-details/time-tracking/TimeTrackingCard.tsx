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

import { Card, Box, Typography, Chip, useTheme } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import type { TimeCard } from "@models/responses";
import { getTimeCardStateColorPath } from "@utils/projectDetails";
import { getSupportOverviewChipSx, getPlainChipSx } from "@utils/support";

interface TimeTrackingCardProps {
  card: TimeCard;
}

/**
 * TimeTrackingCard displays a single time card with case label, state, billable, case number, total time, and approver.
 *
 * @param {TimeTrackingCardProps} props - Time card data.
 * @returns {JSX.Element} The rendered time card.
 */
export default function TimeTrackingCard({
  card,
}: TimeTrackingCardProps): JSX.Element {
  const theme = useTheme();
  const { case: caseData, state, hasBillable, totalTime, approvedBy } = card;

  const label = caseData?.label?.trim() || "--";
  const caseNumber = caseData?.number?.trim() || "--";
  const approvedByName = approvedBy?.label?.trim() || "--";

  // Convert totalTime from minutes to hours
  const totalTimeInHours =
    totalTime !== undefined && totalTime !== null
      ? Math.round((totalTime / 60) * 100) / 100
      : null;

  const stateColorPath = getTimeCardStateColorPath(state);

  return (
    <Card
      sx={{
        p: "20px",
        display: "flex",
        flexDirection: "column",
        gap: "24px",
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          mb: "12px",
        }}
      >
        <Box sx={{ flex: 1 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: "8px",
              mb: "8px",
              flexWrap: "wrap",
            }}
          >
            <Chip
              label={caseNumber}
              size="small"
              variant="outlined"
              sx={getPlainChipSx()}
            />
            <Chip
              label={state?.label || "--"}
              size="small"
              variant="outlined"
              sx={getSupportOverviewChipSx(stateColorPath, theme)}
            />
            {hasBillable && (
              <Chip
                label="Billable"
                size="small"
                variant="outlined"
                sx={getSupportOverviewChipSx("success.main", theme)}
              />
            )}
          </Box>
          <Typography
            variant="body2"
            sx={{
              mb: "8px",
              color: "text.primary",
              fontSize: "0.875rem",
            }}
          >
            {label}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            Approved by: {approvedByName}
          </Typography>
        </Box>
        <Box sx={{ textAlign: "right" }}>
          <Typography
            variant="h5"
            sx={{
              fontWeight: 400,
              fontSize: "1.5rem",
              color: "text.primary",
            }}
          >
            {totalTimeInHours !== null ? `${totalTimeInHours} hrs` : "--"}
          </Typography>
        </Box>
      </Box>
    </Card>
  );
}
