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

import { Box, Chip, Grid, Skeleton, Tooltip, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { PROJECT_METADATA_CHIP_SX } from "@features/project-details/constants/projectInformationConstants";
import type { ProjectMetadataSecondaryRowProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * Secondary metadata cells: go-live date and onboarding status.
 *
 * @param props - Field values and loading state.
 * @returns {JSX.Element} Grid row.
 */
export default function ProjectMetadataSecondaryRow({
  goLivePlanDate,
  onboardingStatus,
  hideOnboardingStatus = false,
  isLoading,
  isError,
}: ProjectMetadataSecondaryRowProps): JSX.Element {
  return (
    <>
      <Grid size={{ xs: 12, sm: 6, md: 3 }}>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: { xs: "center", md: "flex-start" },
          }}
        >
          <Typography
            variant="body2"
            fontWeight="medium"
            sx={{ display: "block", mb: 0.5 }}
          >
            Go Live Date
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width="60%" />
          ) : isError ? (
            <ErrorIndicator entityName="go live date" />
          ) : (
            <Typography variant="body2">{goLivePlanDate}</Typography>
          )}
        </Box>
      </Grid>
      {hideOnboardingStatus && (
        <Grid
          size={{ xs: 12, sm: 6, md: 3 }}
          sx={{ display: { xs: "none", md: "block" } }}
        />
      )}
      {!hideOnboardingStatus && (
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              alignItems: { xs: "center", md: "flex-start" },
            }}
          >
            <Typography
              variant="body2"
              fontWeight="medium"
              sx={{ display: "block", mb: 0.5 }}
            >
              Onboarding Status
            </Typography>
            {isLoading ? (
              <Skeleton variant="rounded" width={80} height={24} />
            ) : isError ? (
              <ErrorIndicator entityName="onboarding status" />
            ) : onboardingStatus ? (
              <Tooltip title={onboardingStatus} arrow>
                <Chip
                  label={onboardingStatus}
                  size="small"
                  color="info"
                  variant="outlined"
                  sx={PROJECT_METADATA_CHIP_SX}
                />
              </Tooltip>
            ) : (
              <Chip
                label={onboardingStatus}
                size="small"
                color="info"
                variant="outlined"
                sx={PROJECT_METADATA_CHIP_SX}
              />
            )}
          </Box>
        </Grid>
      )}
    </>
  );
}
