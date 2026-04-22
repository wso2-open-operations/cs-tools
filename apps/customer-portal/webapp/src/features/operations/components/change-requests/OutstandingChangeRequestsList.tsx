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

import { Box, Form, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { OutstandingChangeRequestsListProps } from "@features/operations/types/changeRequests";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { formatRelativeTime } from "@features/support/utils/support";
import OutstandingChangeRequestsSkeleton from "./OutstandingChangeRequestsSkeleton";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import {
  getChangeRequestStateColor,
} from "@features/operations/utils/changeRequestUi";
import CaseCardDescriptionClamp from "@components/list-view/CaseCardDescriptionClamp";

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
  const getDescription = (
    item: OutstandingChangeRequestsListProps["changeRequests"][number],
  ): string | null | undefined => {
    return item.description;
  };

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
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  flexWrap: "wrap",
                }}
              >
                <Typography variant="body2" fontWeight={500} color="text.primary">
                  {cr.number}
                </Typography>
                {cr.state?.label ? (
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        bgcolor: getChangeRequestStateColor(cr.state),
                        borderRadius: "50%",
                        flexShrink: 0,
                      }}
                    />
                    <Typography
                      variant="caption"
                      sx={{ color: getChangeRequestStateColor(cr.state) }}
                    >
                      {cr.state.label}
                    </Typography>
                  </Box>
                ) : null}
              </Box>
            }
          />
          <Form.CardContent sx={{ p: 0, flex: 1 }}>
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
              <CaseCardDescriptionClamp
                description={getDescription(cr)}
                hideWhenEmpty
              />
            </Box>
          </Form.CardContent>
          <Form.CardActions
            sx={{
              p: 0,
              justifyContent: "flex-end",
              alignItems: "center",
              flexWrap: "wrap",
              gap: 1,
            }}
          >
            <Typography variant="caption" color="text.secondary">
              {formatRelativeTime(cr.createdOn ?? undefined)}
            </Typography>
          </Form.CardActions>
        </Form.CardButton>
      ))}
    </Box>
  );
}
