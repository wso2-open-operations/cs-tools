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

import { Box, LinearProgress, Paper, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import type { CaseDetails } from "@models/responses";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

export interface SecurityReportAnalysisHeaderProps {
  data: CaseDetails | undefined;
}

/**
 * Header section for Security Report Analysis cases.
 * Displays report information grid and findings resolution progress.
 *
 * @param {SecurityReportAnalysisHeaderProps} props - Case data and loading state.
 * @returns {JSX.Element} The security report analysis header.
 */
export default function SecurityReportAnalysisHeader({
  data,
}: SecurityReportAnalysisHeaderProps): JSX.Element {
  const findingsResolved = data?.findingsResolved ?? null;
  const findingsTotal = data?.findingsTotal ?? null;

  const hasProgressData =
    typeof findingsTotal === "number" && findingsResolved !== null;

  // Calculate progress percentage
  const progressPercentage = hasProgressData
    ? Math.max(
        0,
        Math.min(
          100,
          Math.round((findingsResolved / (findingsTotal || 1)) * 100),
        ),
      )
    : 0;

  return (
    <Paper
      variant="outlined"
      sx={{
        mt: 2,
        mb: 1,
        py: 1.5,
        px: 2,
        bgcolor: "background.default",
      }}
    >
      {/* Findings Resolution Progress */}
      <Box>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            mb: 1,
          }}
        >
          <Typography
            sx={{
              fontSize: "0.875rem",
              color: "text.secondary",
            }}
          >
            Findings Resolution Progress
          </Typography>
          {hasProgressData ? (
            <Typography
              sx={{
                fontSize: "0.875rem",
                color: "text.secondary",
              }}
            >
              {findingsResolved} / {findingsTotal} ({progressPercentage}%)
            </Typography>
          ) : (
            <ErrorIndicator entityName="progress data" />
          )}
        </Box>
        <Box
          sx={{
            width: "100%",
            bgcolor: "action.hover",
            borderRadius: 1,
            height: 8,
            overflow: "hidden",
          }}
        >
          {hasProgressData ? (
            <LinearProgress
              variant="determinate"
              value={progressPercentage}
              color="success"
              sx={{
                height: "100%",
                bgcolor: "transparent",
                "& .MuiLinearProgress-bar": {
                  borderRadius: 1,
                  transition: "transform 0.4s ease",
                },
              }}
            />
          ) : (
            <Box
              sx={{
                width: "100%",
                height: "100%",
                bgcolor: "action.hover",
              }}
            />
          )}
        </Box>
      </Box>
    </Paper>
  );
}
