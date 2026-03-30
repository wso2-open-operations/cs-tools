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
  Grid,
  Chip,
  Skeleton,
  Tooltip,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import {
  getProjectTypeColor,
  getSupportTierColor,
  getSLAStatusColor,
} from "@utils/projectDetails";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

interface ProjectMetadataProps {
  createdDate: string;
  type: {
    id: string;
    label: string;
  };
  supportTier: string;
  slaStatus: string;
  goLivePlanDate: string;
  onboardingStatus: string;
  isLoading?: boolean;
  isError?: boolean;
}

const ProjectMetadata = ({
  createdDate,
  type,
  supportTier,
  slaStatus,
  goLivePlanDate,
  onboardingStatus,
  isLoading,
  isError,
}: ProjectMetadataProps): JSX.Element => {
  const chipStyle = {
    typography: "caption",
    maxWidth: "140px",
    "& .MuiChip-label": {
      display: "block",
      overflow: "hidden",
      textOverflow: "ellipsis",
      whiteSpace: "nowrap",
    },
  };

  return (
    <Box sx={{ pt: 3, borderTop: 1, borderColor: "divider" }}>
      {/* Row 1: Created Date, Project Type, Support Tier */}
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
                  sx={chipStyle}
                />
              </Tooltip>
            ) : (
              <Chip
                label=""
                size="small"
                variant="outlined"
                color={getProjectTypeColor("")}
                sx={chipStyle}
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
                  sx={chipStyle}
                />
              </Tooltip>
            ) : (
              <Chip
                label={supportTier}
                size="small"
                color={getSupportTierColor(supportTier)}
                variant="outlined"
                sx={chipStyle}
              />
            )}
          </Box>
        </Grid>
      </Grid>

      {/* Row 2: Overall Status, Go Live Date, Onboarding Status */}
      <Grid
        container
        spacing={2}
        sx={{
          alignItems: "center",
          justifyContent: "space-between",
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
              Overall Status
            </Typography>
            {isLoading || isError ? (
              <Skeleton variant="rounded" width={60} height={24} />
            ) : slaStatus ? (
              <Tooltip title={slaStatus} arrow>
                <Chip
                  label={slaStatus}
                  size="small"
                  color={getSLAStatusColor(slaStatus)}
                  variant="outlined"
                  sx={chipStyle}
                />
              </Tooltip>
            ) : (
              <Chip
                label={slaStatus}
                size="small"
                color={getSLAStatusColor(slaStatus)}
                variant="outlined"
                sx={chipStyle}
              />
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
              Onboarding Status
            </Typography>
            {isLoading || isError ? (
              <Skeleton variant="rounded" width={80} height={24} />
            ) : onboardingStatus ? (
              <Tooltip title={onboardingStatus} arrow>
                <Chip
                  label={onboardingStatus}
                  size="small"
                  color="info"
                  variant="outlined"
                  sx={chipStyle}
                />
              </Tooltip>
            ) : (
              <Chip
                label={onboardingStatus}
                size="small"
                color="info"
                variant="outlined"
                sx={chipStyle}
              />
            )}
          </Box>
        </Grid>
      </Grid>
    </Box>
  );
};

export default ProjectMetadata;
