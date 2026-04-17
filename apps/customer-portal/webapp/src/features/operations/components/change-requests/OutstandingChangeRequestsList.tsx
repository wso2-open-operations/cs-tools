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

import { Box, Chip, Form, Typography, alpha } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { OutstandingChangeRequestsListProps } from "@features/operations/types/changeRequests";
import { NULL_PLACEHOLDER } from "@constants/common";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { formatRelativeTime } from "@features/support/utils/support";
import OutstandingChangeRequestsSkeleton from "./OutstandingChangeRequestsSkeleton";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import {
  getChangeRequestStateColor,
  getChangeRequestStateIcon,
} from "@features/operations/utils/changeRequestUi";

/**
 * Renders a list of change request rows for the operations overview card.
 *
 * @param {OutstandingChangeRequestsListProps} props - Change requests array and loading state.
 * @returns {JSX.Element} The list of change request rows.
 */
export default function OutstandingChangeRequestsList({
  changeRequests,
  isLoading,
  isError,
  onItemClick,
}: OutstandingChangeRequestsListProps): JSX.Element {
  if (isError) {
    return <ErrorIndicator entityName="change requests" size="medium" />;
  }

  if (isLoading) {
    return <OutstandingChangeRequestsSkeleton />;
  }

  if (changeRequests.length === 0) {
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          py: 2,
        }}
      >
        <EmptyIcon
          style={{
            width: 120,
            maxWidth: "100%",
            height: "auto",
            marginBottom: 12,
          }}
        />
        <Typography variant="body2" color="text.secondary">
          No change requests.
        </Typography>
      </Box>
    );
  }

  return (
    <Box
      sx={{ display: "flex", flexDirection: "column", gap: 1.5, width: "100%" }}
    >
      {changeRequests.map((cr) => (
        <Form.CardButton
          key={cr.id}
          onClick={() => onItemClick?.(cr)}
          sx={{
            p: 2,
            display: "flex",
            flexDirection: "column",
            alignItems: "stretch",
            gap: 1,
          }}
        >
          <Form.CardHeader
            sx={{ p: 0 }}
            title={
              <Typography variant="body2" fontWeight={500} color="text.primary">
                {cr.number}
              </Typography>
            }
          />
          <Form.CardContent sx={{ p: 0 }}>
            <Box sx={{ minWidth: 0 }}>
              <Typography
                variant="body2"
                color="text.primary"
                sx={{
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  display: "-webkit-box",
                  WebkitLineClamp: 1,
                  WebkitBoxOrient: "vertical",
                }}
              >
                {cr.title}
              </Typography>
            </Box>
          </Form.CardContent>
          <Form.CardActions
            sx={{
              p: 0,
              justifyContent: "space-between",
              alignItems: "center",
              flexWrap: "wrap",
              gap: 1,
            }}
          >
            {cr.state?.label ? (
              (() => {
                const statusColor = getChangeRequestStateColor(cr.state);
                const StatusIcon = getChangeRequestStateIcon(cr.state);
                return (
                  <Chip
                    icon={<StatusIcon size={12} />}
                    label={cr.state.label}
                    size="small"
                    variant="outlined"
                    sx={{
                      bgcolor: alpha(statusColor, 0.1),
                      color: statusColor,
                      px: 0,
                      height: 20,
                      fontSize: "0.75rem",
                      fontWeight: 500,
                      "& .MuiChip-icon": {
                        color: statusColor,
                        ml: "6px",
                        mr: "6px",
                      },
                      "& .MuiChip-label": {
                        pl: 0,
                        pr: "6px",
                      },
                    }}
                  />
                );
              })()
            ) : (
              <Typography variant="caption" color="text.secondary">
                {NULL_PLACEHOLDER}
              </Typography>
            )}
            <Typography variant="caption" color="text.secondary">
              {formatRelativeTime(cr.createdOn ?? undefined)}
            </Typography>
          </Form.CardActions>
        </Form.CardButton>
      ))}
    </Box>
  );
}
