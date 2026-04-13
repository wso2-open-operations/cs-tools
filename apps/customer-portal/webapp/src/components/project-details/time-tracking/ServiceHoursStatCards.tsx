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

import type { ProjectDetails } from "@/types/projects";
import {
  formatProjectDate,
  formatServiceHoursDecimalAsHrMin,
} from "@utils/projectDetails";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

export interface ServiceHoursStatCardsProps {
  project?: ProjectDetails | null;
  isLoading?: boolean;
  isError?: boolean;
}

const NOT_AVAILABLE = "Not Available";

function formatHoursDisplay(
  consumed: number | undefined,
  total: number | undefined,
): string {
  if (consumed == null && total == null) return NOT_AVAILABLE;
  const c = Number(consumed ?? 0);
  const t = Number(total ?? 0);
  const pct = t === 0 ? 0 : Math.round((c / t) * 100);
  return `${formatServiceHoursDecimalAsHrMin(c)}/${formatServiceHoursDecimalAsHrMin(t)} (${pct}%)`;
}

function formatRemaining(value: number | undefined): string {
  return formatServiceHoursDecimalAsHrMin(value);
}

/**
 * ServiceHoursStatCards displays Query Hours and Onboarding Hours stat cards for the time tracking page.
 *
 * @param {ServiceHoursStatCardsProps} props - Project data, loading and error state.
 * @returns {JSX.Element} The rendered ServiceHoursStatCards component.
 */
export default function ServiceHoursStatCards({
  project,
  isLoading,
  isError,
}: ServiceHoursStatCardsProps): JSX.Element {
  const queryDisplay = formatHoursDisplay(
    project?.consumedQueryHours,
    project?.totalQueryHours,
  );
  const queryRemaining = formatRemaining(project?.remainingQueryHours);

  const onboardingDisplay = formatHoursDisplay(
    project?.consumedOnboardingHours,
    project?.totalOnboardingHours,
  );
  const onboardingRemaining = formatRemaining(
    project?.remainingOnboardingHours,
  );
  const onboardingExpiry =
    project?.onboardingExpiryDate?.trim() && project.onboardingExpiryDate
      ? formatProjectDate(project.onboardingExpiryDate)
      : NOT_AVAILABLE;

  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
        gap: 2,
        mb: 3,
      }}
    >
      {/* Query Hours Card */}
      <Card sx={{ p: 2.5 }}>
        <CardContent sx={{ p: 0, "&:last-child": { pb: 0 } }}>
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
            <Box component="span" sx={{ color: "info.main", display: "inline-flex" }}>
              <Clock size={16} />
            </Box>
          </Box>
          {isLoading ? (
            <Skeleton variant="text" width="80%" height={32} sx={{ mb: 1 }} />
          ) : isError ? (
            <ErrorIndicator entityName="query hours" />
          ) : (
            <>
              <Typography variant="h5" sx={{ mb: 1 }}>
                {queryDisplay}
              </Typography>
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
                <Typography variant="caption">{queryRemaining}</Typography>
              </Box>
            </>
          )}
        </CardContent>
      </Card>

      {/* Onboarding Hours Card */}
      <Card sx={{ p: 2.5 }}>
        <CardContent sx={{ p: 0, "&:last-child": { pb: 0 } }}>
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
            <Box component="span" sx={{ color: "success.main", display: "inline-flex" }}>
              <Clock size={16} />
            </Box>
          </Box>
          {isLoading ? (
            <Skeleton variant="text" width="80%" height={32} sx={{ mb: 1 }} />
          ) : isError ? (
            <ErrorIndicator entityName="onboarding hours" />
          ) : (
            <>
              <Typography variant="h5" sx={{ mb: 1 }}>
                {onboardingDisplay}
              </Typography>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  mb: 0.5,
                }}
              >
                <Typography variant="caption" color="text.secondary">
                  Remaining
                </Typography>
                <Typography variant="caption">{onboardingRemaining}</Typography>
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
                <Typography variant="caption">{onboardingExpiry}</Typography>
              </Box>
            </>
          )}
        </CardContent>
      </Card>
    </Box>
  );
}
