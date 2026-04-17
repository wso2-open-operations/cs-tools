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

import { Box, Card, CardContent, Typography, Skeleton } from "@wso2/oxygen-ui";
import { Clock } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { PROJECT_DETAILS_SERVICE_HOURS_NOT_AVAILABLE } from "@features/project-details/constants/projectDetailsConstants";
import type { ServiceHoursAllocationsCardProps } from "@features/project-details/types/projectDetailsComponents";
import {
  formatProjectDate,
  formatServiceHoursDecimalAsHrMin,
} from "@features/project-details/utils/projectDetails";
import { formatServiceHoursAllocationDisplay } from "@features/project-details/utils/serviceHoursFormat";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";

function formatRemaining(value: number | undefined): string {
  return formatServiceHoursDecimalAsHrMin(value);
}

/**
 * ServiceHoursAllocationsCard displays Query Hours and Onboarding Hours allocations from project details.
 *
 * @param {ServiceHoursAllocationsCardProps} props - Project data, loading and error state.
 * @returns {JSX.Element} The rendered ServiceHoursAllocationsCard component.
 */
export default function ServiceHoursAllocationsCard({
  project,
  isLoading,
  isError,
}: ServiceHoursAllocationsCardProps): JSX.Element {
  const queryDisplay = formatServiceHoursAllocationDisplay(
    project?.consumedQueryHours,
    project?.totalQueryHours,
  );
  const queryRemaining = formatRemaining(project?.remainingQueryHours);

  const onboardingDisplay = formatServiceHoursAllocationDisplay(
    project?.consumedOnboardingHours,
    project?.totalOnboardingHours,
  );
  const onboardingRemaining = formatRemaining(
    project?.remainingOnboardingHours,
  );
  const onboardingExpiry =
    project?.onboardingExpiryDate?.trim() && project.onboardingExpiryDate
      ? formatProjectDate(project.onboardingExpiryDate)
      : PROJECT_DETAILS_SERVICE_HOURS_NOT_AVAILABLE;

  return (
    <Card sx={{ height: "100%" }}>
      <CardContent
        sx={{
          p: 3,
          height: "100%",
          display: "flex",
          flexDirection: "column",
          gap: 3,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Clock size={20} />
          <Typography variant="h6">Service Hours Allocations</Typography>
        </Box>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          {/* Query Hours */}
          <Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                mb: 1,
              }}
            >
              <Typography variant="body2" color="text.secondary">
                Query Hours
              </Typography>
              {isLoading ? (
                <Skeleton variant="text" width={80} height={24} />
              ) : isError ? (
                <ErrorIndicator entityName="query hours" />
              ) : (
                <Typography variant="body2">{queryDisplay}</Typography>
              )}
            </Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              <Typography variant="caption" color="text.secondary">
                Remaining
              </Typography>
              {isLoading ? (
                <Skeleton variant="text" width={40} height={20} />
              ) : isError ? (
                <span />
              ) : (
                <Typography variant="caption">{queryRemaining}</Typography>
              )}
            </Box>
          </Box>

          {/* Onboarding Hours */}
          <Box sx={{ borderTop: 1, borderColor: "divider", pt: 2 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                mb: 1,
              }}
            >
              <Typography variant="body2" color="text.secondary">
                Onboarding Hours
              </Typography>
              {isLoading ? (
                <Skeleton variant="text" width={80} height={24} />
              ) : isError ? (
                <ErrorIndicator entityName="onboarding hours" />
              ) : (
                <Typography variant="body2">{onboardingDisplay}</Typography>
              )}
            </Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                mb: 1,
              }}
            >
              <Typography variant="caption" color="text.secondary">
                Remaining
              </Typography>
              {isLoading ? (
                <Skeleton variant="text" width={40} height={20} />
              ) : isError ? (
                <span />
              ) : (
                <Typography variant="caption">{onboardingRemaining}</Typography>
              )}
            </Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              <Typography variant="caption" color="text.secondary">
                Onboarding Expiry Date
              </Typography>
              {isLoading ? (
                <Skeleton variant="text" width={80} height={20} />
              ) : isError ? (
                <span />
              ) : (
                <Typography variant="caption">{onboardingExpiry}</Typography>
              )}
            </Box>
          </Box>
        </Box>
      </CardContent>
    </Card>
  );
}
