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

import { Box, Typography, Grid, Chip, Skeleton } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import {
  getProjectTypeColor,
  getSupportTierColor,
  getSLAStatusColor,
} from "@utils/projectStats";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

interface ProjectMetadataProps {
  createdDate: string;
  projectType: string;
  supportTier: string;
  slaStatus: string;
  isLoading?: boolean;
  isError?: boolean;
}

const ProjectMetadata = ({
  createdDate,
  projectType,
  supportTier,
  slaStatus,
  isLoading,
  isError,
}: ProjectMetadataProps): JSX.Element => {
  return (
    <Box sx={{ pt: 3, borderTop: 1, borderColor: "divider" }}>
      <Grid container spacing={2}>
        {/* Created Date */}
        <Grid size={{ xs: 12, md: 3 }}>
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
            <Typography variant="caption">{createdDate}</Typography>
          )}
        </Grid>

        {/* Project Type */}
        <Grid size={{ xs: 12, md: 3 }}>
          <Typography
            variant="body2"
            fontWeight="medium"
            sx={{ display: "block", mb: 0.5 }}
          >
            Project Type
          </Typography>
          {isLoading ? (
            <Skeleton variant="rounded" width={80} height={24} />
          ) : (
            <Chip
              label={projectType}
              size="small"
              variant="outlined"
              color={getProjectTypeColor(projectType)}
              sx={{ font: "caption" }}
            />
          )}
        </Grid>

        {/* Support Tier */}
        <Grid size={{ xs: 12, md: 3 }}>
          <Typography
            variant="body2"
            fontWeight="medium"
            sx={{ display: "block", mb: 0.5 }}
          >
            Support Tier
          </Typography>
          {isLoading ? (
            <Skeleton variant="rounded" width={80} height={24} />
          ) : (
            <Chip
              label={supportTier}
              size="small"
              color={getSupportTierColor(supportTier)}
              variant="outlined"
              sx={{ font: "caption" }}
            />
          )}
        </Grid>

        {/* SLA Status */}
        <Grid size={{ xs: 12, md: 3 }}>
          <Typography
            variant="body2"
            fontWeight="medium"
            sx={{ display: "block", mb: 0.5 }}
          >
            SLA Status
          </Typography>
          {isLoading ? (
            <Skeleton variant="rounded" width={60} height={24} />
          ) : (
            <Chip
              label={slaStatus}
              size="small"
              color={getSLAStatusColor(slaStatus)}
              variant="outlined"
              sx={{ font: "caption" }}
            />
          )}
        </Grid>
      </Grid>
    </Box>
  );
};

export default ProjectMetadata;
