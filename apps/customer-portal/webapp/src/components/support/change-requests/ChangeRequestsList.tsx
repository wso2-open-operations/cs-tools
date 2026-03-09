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

import { Box, Chip, Form, Typography, alpha, colors } from "@wso2/oxygen-ui";
import { Calendar, Server, TriangleAlert } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { ChangeRequestItem } from "@models/responses";
import ChangeRequestsListSkeleton from "@components/support/change-requests/ChangeRequestsListSkeleton";
import ErrorStateIcon from "@components/common/error-state/ErrorStateIcon";
import { formatDateTime, formatDuration } from "@utils/support";
import {
  getChangeRequestStateColor,
  getChangeRequestStateIcon,
  getChangeRequestImpactColor,
  formatImpactLabel,
} from "@constants/supportConstants";

export interface ChangeRequestsListProps {
  changeRequests: ChangeRequestItem[];
  isLoading: boolean;
  isError?: boolean;
  onChangeRequestClick?: (item: ChangeRequestItem) => void;
}

/**
 * Component to display change requests as cards.
 *
 * @param {ChangeRequestsListProps} props - Change requests array and loading state.
 * @returns {JSX.Element} The rendered change request cards list.
 */
export default function ChangeRequestsList({
  changeRequests,
  isLoading,
  isError = false,
  onChangeRequestClick,
}: ChangeRequestsListProps): JSX.Element {
  if (isLoading) {
    return <ChangeRequestsListSkeleton />;
  }

  if (isError) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <ErrorStateIcon style={{ width: 200, height: "auto" }} />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 3 }}>
          Failed to load change requests. Please try again.
        </Typography>
      </Box>
    );
  }

  if (changeRequests.length === 0) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <Typography variant="body1" color="text.secondary">
          No change requests found.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {changeRequests.map((item) => {
        const statusColor = getChangeRequestStateColor(item.state?.label);
        const StatusIcon = getChangeRequestStateIcon(item.state?.label);
        const impactColor = getChangeRequestImpactColor(item.impact?.label);

        // Format dates
        const startTime = formatDateTime(item.startDate);
        const endTime = formatDateTime(item.endDate);
        const durationText = formatDuration(item.duration);

        return (
          <Form.CardButton
            key={item.id}
            onClick={() => onChangeRequestClick?.(item)}
            sx={{
              p: 3,
              display: "flex",
              flexDirection: "row",
              alignItems: "flex-start",
              justifyContent: "space-between",
              gap: 3,
            }}
          >
            {/* Right Section - Status badges */}
            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                gap: 1,
                flexShrink: 0,
                order: 2,
              }}
            >
              {item.impact?.label && (
                <Chip
                  label={formatImpactLabel(item.impact.label)}
                  size="small"
                  sx={{
                    bgcolor: alpha(impactColor, 0.1),
                    color: impactColor,
                    borderColor: alpha(impactColor, 0.2),
                    border: "1px solid",
                    height: 22,
                    fontSize: "0.75rem",
                    fontWeight: 500,
                  }}
                />
              )}
              {item.state?.label && (
                <Chip
                  icon={<StatusIcon size={12} />}
                  label={item.state.label}
                  size="small"
                  sx={{
                    bgcolor: alpha(statusColor, 0.1),
                    color: statusColor,
                    borderColor: alpha(statusColor, 0.2),
                    border: "1px solid",
                    height: 22,
                    fontSize: "0.75rem",
                    fontWeight: 500,
                    "& .MuiChip-icon": {
                      color: statusColor,
                      ml: "6px",
                      mr: "-2px",
                    },
                  }}
                />
              )}
            </Box>

            {/* Left Section */}
            <Box sx={{ flex: 1, minWidth: 0, order: 1 }}>
              {/* Title with badges */}
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  mb: 1.5,
                  flexWrap: "wrap",
                }}
              >
                <Typography
                  variant="h6"
                  color="text.primary"
                  sx={{ fontWeight: 500, fontSize: "1rem" }}
                >
                  {item.title || "Not Available"}
                </Typography>
                {item.hasServiceOutage && (
                  <Chip
                    icon={<TriangleAlert size={12} />}
                    label="Service Outage"
                    size="small"
                    sx={{
                      bgcolor: alpha(colors.red[500], 0.1),
                      color: colors.red[800],
                      borderColor: alpha(colors.red[500], 0.2),
                      border: "1px solid",
                      height: 22,
                      fontSize: "0.75rem",
                      "& .MuiChip-icon": {
                        color: colors.red[800],
                        ml: "6px",
                        mr: "-2px",
                      },
                    }}
                  />
                )}
              </Box>

              {/* Details row */}
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  mb: 1.5,
                  flexWrap: "wrap",
                }}
              >
                <Typography
                  variant="body2"
                  fontWeight={500}
                  color="text.primary"
                >
                  {item.number || "Not Available"}
                </Typography>
                {item.case?.number && (
                  <>
                    <Typography variant="body2" color="text.disabled">
                      |
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      SR: {item.case.number}
                    </Typography>
                  </>
                )}
                <>
                  <Typography variant="body2" color="text.disabled">
                    |
                  </Typography>
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.5,
                    }}
                  >
                    <Server size={16} />
                    <Typography variant="body2" color="text.secondary">
                      {item.deployedProduct?.label || "Not Available"}
                    </Typography>
                  </Box>
                </>
              </Box>

              {/* Date/Time row */}
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  flexWrap: "wrap",
                }}
              >
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 0.5,
                  }}
                >
                  <Calendar size={16} />
                  <Typography variant="body2" color="text.secondary">
                    {startTime}
                  </Typography>
                </Box>
                {item.startDate && item.endDate && (
                  <Typography variant="body2" color="text.disabled">
                    →
                  </Typography>
                )}
                {item.startDate && item.endDate && (
                  <Typography variant="body2" color="text.secondary">
                    {endTime}
                  </Typography>
                )}
                {item.duration != null && (
                  <Chip
                    label={durationText}
                    size="small"
                    variant="outlined"
                    sx={{
                      height: 20,
                      fontSize: "0.75rem",
                    }}
                  />
                )}
              </Box>
            </Box>
          </Form.CardButton>
        );
      })}
    </Box>
  );
}
