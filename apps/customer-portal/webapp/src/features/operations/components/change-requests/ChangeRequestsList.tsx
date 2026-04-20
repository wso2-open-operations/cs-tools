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
import ChangeRequestsListSkeleton from "@features/operations/components/change-requests/ChangeRequestsListSkeleton";
import Error500Page from "@components/error/Error500Page";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";
import { formatDateTime } from "@features/support/utils/support";
import { formatDuration } from "@features/operations/utils/changeRequests";
import {
  getChangeRequestImpactColor,
  getChangeRequestStateColor,
  formatImpactLabel,
} from "@features/operations/utils/changeRequestUi";
import type { ChangeRequestsListProps } from "@features/operations/types/changeRequests";
import {
  CHANGE_REQUESTS_LIST_CREATED_PREFIX,
  CHANGE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE,
  CHANGE_REQUESTS_LIST_EMPTY_REFINED_MESSAGE,
  CHANGE_REQUESTS_LIST_ERROR_MESSAGE,
  CHANGE_REQUESTS_LIST_PLACEHOLDER,
  CHANGE_REQUESTS_LIST_SERVICE_OUTAGE_LABEL,
  CHANGE_REQUESTS_LIST_SR_PREFIX,
  OPERATIONS_LIST_EMPTY_CONTAINER_PY,
  OPERATIONS_LIST_EMPTY_ICON_MARGIN_BOTTOM_PX,
  OPERATIONS_LIST_EMPTY_ILLUSTRATION_WIDTH_PX,
} from "@features/operations/constants/operationsConstants";

/**
 * Component to display change requests as cards.
 *
 * @param props - Change requests array and loading state.
 * @returns {JSX.Element} The rendered change request cards list.
 */
export default function ChangeRequestsList({
  changeRequests,
  isLoading,
  isError = false,
  hasListRefinement = false,
  onChangeRequestClick,
}: ChangeRequestsListProps): JSX.Element {
  if (isLoading) {
    return <ChangeRequestsListSkeleton />;
  }

  if (isError) {
    return (
      <Box sx={{ textAlign: "center", py: OPERATIONS_LIST_EMPTY_CONTAINER_PY }}>
        <Error500Page
          style={{
            width: OPERATIONS_LIST_EMPTY_ILLUSTRATION_WIDTH_PX,
            height: "auto",
          }}
        />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 3 }}>
          {CHANGE_REQUESTS_LIST_ERROR_MESSAGE}
        </Typography>
      </Box>
    );
  }

  if (changeRequests.length === 0) {
    const emptyIconStyle = {
      width: OPERATIONS_LIST_EMPTY_ILLUSTRATION_WIDTH_PX,
      maxWidth: "100%",
      height: "auto",
      marginBottom: OPERATIONS_LIST_EMPTY_ICON_MARGIN_BOTTOM_PX,
    };
    if (hasListRefinement) {
      return (
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            py: OPERATIONS_LIST_EMPTY_CONTAINER_PY,
          }}
        >
          <SearchNoResultsIcon style={emptyIconStyle} />
          <Typography variant="body1" color="text.secondary">
            {CHANGE_REQUESTS_LIST_EMPTY_REFINED_MESSAGE}
          </Typography>
        </Box>
      );
    }
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          py: OPERATIONS_LIST_EMPTY_CONTAINER_PY,
        }}
      >
        <EmptyIcon style={emptyIconStyle} />
        <Typography variant="body1" color="text.secondary">
          {CHANGE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE}
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {changeRequests.map((item) => {
        const statusColor = getChangeRequestStateColor(item.state);
        const impactColor = getChangeRequestImpactColor(item.impact?.label);

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
            </Box>

            <Box sx={{ flex: 1, minWidth: 0, order: 1 }}>
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
                  {item.title || CHANGE_REQUESTS_LIST_PLACEHOLDER}
                </Typography>
                {item.hasServiceOutage && (
                  <Chip
                    icon={<TriangleAlert size={12} />}
                    label={CHANGE_REQUESTS_LIST_SERVICE_OUTAGE_LABEL}
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
                  {item.number || CHANGE_REQUESTS_LIST_PLACEHOLDER}
                </Typography>
                {item.state?.label && (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: "50%",
                        bgcolor: statusColor,
                        flexShrink: 0,
                      }}
                    />
                    <Typography variant="caption" color="text.secondary">
                      {item.state.label}
                    </Typography>
                  </Box>
                )}
                {item.case?.number && (
                  <>
                    <Typography variant="body2" color="text.disabled">
                      |
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {CHANGE_REQUESTS_LIST_SR_PREFIX} {item.case.number}
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
                      {item.deployedProduct?.label ||
                        CHANGE_REQUESTS_LIST_PLACEHOLDER}
                    </Typography>
                  </Box>
                </>
              </Box>

              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  flexWrap: "wrap",
                }}
              >
                {item.createdOn && (
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.5,
                    }}
                  >
                    <Typography variant="body2" color="text.secondary">
                      {CHANGE_REQUESTS_LIST_CREATED_PREFIX}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {formatDateTime(item.createdOn)}
                    </Typography>
                  </Box>
                )}
                {item.createdOn && (item.startDate || item.endDate) && (
                  <Typography variant="body2" color="text.disabled">
                    |
                  </Typography>
                )}
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
