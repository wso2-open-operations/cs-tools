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
} from "@/utils/projectStats";

interface SubscriptionDetailsProps {
  startDate: string;
  endDate: string;
  isLoading?: boolean;
}

const SubscriptionDetails = ({
  startDate,
  endDate,
  isLoading,
}: SubscriptionDetailsProps): JSX.Element => {
  const subscriptionStatus = getSubscriptionStatus(endDate);
  const subscriptionColor = getSubscriptionColor(subscriptionStatus);
  const progress =
    subscriptionStatus === "Expired"
      ? 100
      : calculateProgress(startDate, endDate);

  return (
    <Box sx={{ pt: 3, borderTop: 1, borderColor: "divider" }}>
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
        ) : (
          <Chip
            label={subscriptionStatus}
            size="small"
            color={subscriptionColor}
            variant="outlined"
          />
        )}
      </Box>

      <Box sx={{ mb: 2 }}>
        {isLoading ? (
          <Skeleton variant="rounded" width="100%" height={10} />
        ) : (
          <LinearProgress
            variant="determinate"
            value={progress}
            color={
              subscriptionColor === "default" ? "primary" : subscriptionColor
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
        <Box>
          <Typography variant="body2" sx={{ display: "block" }}>
            Start
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width={80} />
          ) : (
            <Typography variant="caption">{startDate}</Typography>
          )}
        </Box>
        <Box sx={{ textAlign: "center" }}>
          <Typography variant="body2" sx={{ display: "block" }}>
            Status
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width={100} />
          ) : (
            <Typography variant="caption">
              {subscriptionStatus === "Expired" ? "Expired on" : "Expires on"}{" "}
              {endDate}
            </Typography>
          )}
        </Box>
        <Box sx={{ textAlign: "right" }}>
          <Typography variant="body2" sx={{ display: "block" }}>
            End
          </Typography>
          {isLoading ? (
            <Skeleton variant="text" width={80} />
          ) : (
            <Typography variant="caption">{endDate}</Typography>
          )}
        </Box>
      </Box>
    </Box>
  );
};

export default SubscriptionDetails;
