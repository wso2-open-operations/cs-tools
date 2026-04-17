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
import {
  getProjectTypeColor,
  getSupportTierColor,
} from "@features/project-details/utils/projectDetails";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { PROJECT_METADATA_CHIP_SX } from "@features/project-details/constants/projectInformationConstants";
import type { ProjectMetadataPrimaryRowProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * First metadata row: created date, project type, support tier.
 *
 * @param props - Field values and loading state.
 * @returns {JSX.Element} Grid row.
 */
export default function ProjectMetadataPrimaryRow({
  createdDate,
  type,
  supportTier,
  isLoading,
  isError,
}: ProjectMetadataPrimaryRowProps): JSX.Element {
  return (
    <Grid
      container
      spacing={2}
      sx={{
        alignItems: "center",
        justifyContent: "space-between",
        mb: 2,
      }}
    >
      <Grid size={{ xs: 12, md: 4 }}>
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
            Created Date
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width="60%" />
          ) : isError ? (
            <ErrorIndicator entityName="project metadata" />
          ) : (
            <Typography variant="body2">{createdDate}</Typography>
          )}
        </Box>
      </Grid>
      <Grid size={{ xs: 12, md: 4 }}>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
          }}
        >
          <Typography
            variant="body2"
            fontWeight="medium"
            sx={{ display: "block", mb: 0.5 }}
          >
            Project Type
          </Typography>
          {isLoading || isError ? (
            <Skeleton variant="rounded" width={80} height={24} />
          ) : type?.label ? (
            <Tooltip title={type.label} arrow>
              <Chip
                label={type.label}
                size="small"
                variant="outlined"
                color={getProjectTypeColor(type.label)}
                sx={PROJECT_METADATA_CHIP_SX}
              />
            </Tooltip>
          ) : (
            <Chip
              label=""
              size="small"
              variant="outlined"
              color={getProjectTypeColor("")}
              sx={PROJECT_METADATA_CHIP_SX}
            />
          )}
        </Box>
      </Grid>
      <Grid size={{ xs: 12, md: 4 }}>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: { xs: "center", md: "flex-end" },
          }}
        >
          <Typography
            variant="body2"
            fontWeight="medium"
            sx={{ display: "block", mb: 0.5 }}
          >
            Support Tier
          </Typography>
          {isLoading || isError ? (
            <Skeleton variant="rounded" width={80} height={24} />
          ) : supportTier ? (
            <Tooltip title={supportTier} arrow>
              <Chip
                label={supportTier}
                size="small"
                color={getSupportTierColor(supportTier)}
                variant="outlined"
                sx={PROJECT_METADATA_CHIP_SX}
              />
            </Tooltip>
          ) : (
            <Chip
              label={supportTier}
              size="small"
              color={getSupportTierColor(supportTier)}
              variant="outlined"
              sx={PROJECT_METADATA_CHIP_SX}
            />
          )}
        </Box>
      </Grid>
    </Grid>
  );
}
