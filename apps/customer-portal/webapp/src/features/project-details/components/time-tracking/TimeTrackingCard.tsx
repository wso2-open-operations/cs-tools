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

import { Card, Box, Typography, Chip } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { formatMinutesAsHrMin } from "@features/project-details/utils/projectDetails";
import { getPlainChipSx } from "@features/support/utils/support";
import type { TimeTrackingCardProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * Displays a single case time card with case number, name, billable/non-billable breakdown and total time.
 *
 * @param {TimeTrackingCardProps} props - Case time card data.
 * @returns {JSX.Element} The rendered time card.
 */
export default function TimeTrackingCard({
  card,
}: TimeTrackingCardProps): JSX.Element {
  const { case: caseData, totalTime } = card;

  const caseNumber = caseData?.number?.trim() || "--";
  const caseName = caseData?.name?.trim() || "--";

  const totalTimeDisplay = formatMinutesAsHrMin(totalTime);

  return (
    <Card sx={{ p: "20px", display: "flex", flexDirection: "column", gap: 2 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          gap: 2,
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              mb: 1,
              flexWrap: "wrap",
            }}
          >
            <Chip
              label={caseNumber}
              size="small"
              variant="outlined"
              sx={getPlainChipSx()}
            />
          </Box>
          <Typography variant="body2" color="text.primary" sx={{ mb: 0.5 }}>
            {caseName}
          </Typography>
        </Box>
        <Box sx={{ textAlign: "right", flexShrink: 0 }}>
          <Typography
            variant="h5"
            sx={{ fontWeight: 400, fontSize: "1.5rem", color: "text.primary" }}
          >
            {totalTimeDisplay === "Not Available" ? "--" : totalTimeDisplay}
          </Typography>
        </Box>
      </Box>
    </Card>
  );
}
