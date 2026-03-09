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
  Chip,
  LinearProgress,
  Skeleton,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import {
  getSubscriptionStatus,
  getSubscriptionColor,
  calculateProgress,
  getRemainingDays,
} from "@utils/projectDetails";
import { SUBSCRIPTION_STATUS } from "@constants/projectDetailsConstants";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

interface SubscriptionDetailsProps {
  startDate?: string | null;
  endDate?: string | null;
  isLoading?: boolean;
  isError?: boolean;
}

const SubscriptionDetails = ({
  startDate,
  endDate,
  isLoading,
  isError,
}: SubscriptionDetailsProps): JSX.Element => {
  const isDateMissing =
    !startDate || startDate === "--" || !endDate || endDate === "--";
  const isDateInvalid = isError || isDateMissing;

  const subscriptionStatus = getSubscriptionStatus(
    endDate || "",
    startDate || undefined,
  );
  const subscriptionColor = getSubscriptionColor(subscriptionStatus);
  const remainingDays = getRemainingDays(endDate || "");

  const progress = isDateInvalid
    ? 0
    : subscriptionStatus === SUBSCRIPTION_STATUS.EXPIRED
      ? 100
      : calculateProgress(startDate, endDate);

  return (
    <Box sx={{ pt: 3, borderTop: 1, borderColor: "divider" }}>
      {/* Header */}
      <Box sx={{ display: "flex", justifyContent: "space-between", mb: 2 }}>
        <Typography
          variant="body2"
          fontWeight="medium"
          sx={{ display: "block", mb: 0.5 }}
        >
          Subscription Period
        </Typography>

        {isLoading ? (
          <Skeleton variant="rounded" width={70} height={24} />
        ) : isError ? (
          <ErrorIndicator entityName="subscription status" />
        ) : isDateMissing ? (
          <Typography variant="caption" color="text.secondary">
            Not available
          </Typography>
        ) : subscriptionStatus !== SUBSCRIPTION_STATUS.ACTIVE ? (
          <Chip
            label={subscriptionStatus}
            size="small"
            color={subscriptionColor}
            variant="outlined"
          />
        ) : null}
      </Box>

      {/* Progress bar */}
      <Box sx={{ mb: 2 }}>
        {isLoading ? (
          <Skeleton variant="rounded" width="100%" height={10} />
        ) : (
          <LinearProgress
            variant="determinate"
            value={progress}
            color={
              isDateInvalid
                ? "error"
                : subscriptionColor === "default"
                  ? "primary"
                  : subscriptionColor
            }
          />
        )}
      </Box>

      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-end",
        }}
      >
        {/* Start Date*/}
        <Box>
          <Typography variant="body2" sx={{ display: "block" }}>
            Start
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width="60%" />
          ) : isError ? (
            <ErrorIndicator entityName="subscription details" />
          ) : isDateMissing ? (
            <Typography variant="body2" color="text.secondary">
              Not available
            </Typography>
          ) : (
            <Typography variant="body2">{startDate}</Typography>
          )}
        </Box>

        {/* Remaining (hidden when expired) */}
        <Box sx={{ textAlign: "center" }}>
          <Typography variant="body2" sx={{ display: "block" }}>
            Remaining
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width={100} />
          ) : isError ? (
            <ErrorIndicator entityName="subscription status" />
          ) : isDateMissing ? (
            <Typography variant="body2" color="text.secondary">
              Not available
            </Typography>
          ) : subscriptionStatus === SUBSCRIPTION_STATUS.EXPIRED ? (
            <Typography variant="body2">
              Expired on {endDate}
            </Typography>
          ) : (
            <Typography variant="body2">
              {remainingDays} {remainingDays === 1 ? "day" : "days"}
            </Typography>
          )}
        </Box>

        {/* End Date */}
        <Box sx={{ textAlign: "right" }}>
          <Typography variant="body2" sx={{ display: "block" }}>
            End
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width={80} />
          ) : isError ? (
            <ErrorIndicator entityName="subscription details" />
          ) : isDateMissing ? (
            <Typography variant="body2" color="text.secondary">
              Not available
            </Typography>
          ) : (
            <Typography variant="body2">{endDate}</Typography>
          )}
        </Box>
      </Box>
    </Box>
  );
};

export default SubscriptionDetails;
