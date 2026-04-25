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

import type { OutstandingCasesListProps } from "@features/support/types/supportComponents";
import {
  Box,
  Form,
  Stack,
  Tooltip,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { NULL_PLACEHOLDER } from "@constants/common";
import CaseCardDescriptionClamp from "@components/list-view/CaseCardDescriptionClamp";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import OutstandingCasesSkeleton from "./OutstandingCasesSkeleton";
import {
  formatRelativeTime,
  getAssignedEngineerLabel,
  getStatusColor,
  resolveColorFromTheme,
  stripHtml,
} from "@features/support/utils/support";
import { getChangeRequestStateColor } from "@features/operations/utils/changeRequestUi";

/**
 * Renders a list of outstanding case rows for the support overview card.
 *
 * @param {OutstandingCasesListProps} props - Cases array and loading state.
 * @returns {JSX.Element} The list of case rows.
 */
export default function OutstandingCasesList({
  cases,
  isLoading,
  isError,
  onCaseClick,
  useChangeRequestColors = false,
  showInternalId = false,
}: OutstandingCasesListProps): JSX.Element {
  const theme = useTheme();

  const listShellSx = {
    display: "flex",
    flexDirection: "column" as const,
    gap: 1.5,
    width: "100%",
    flex: 1,
    minHeight: 0,
  };

  if (isError) {
    return <ErrorIndicator entityName="outstanding cases" size="medium" />;
  }

  if (isLoading) {
    return (
      <Box sx={listShellSx}>
        <OutstandingCasesSkeleton />
      </Box>
    );
  }

  if (cases.length === 0) {
    return (
      <Box
        sx={{
          ...listShellSx,
          alignItems: "center",
          justifyContent: "center",
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
          No outstanding cases.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={listShellSx}>
      {cases.map((c) => {
        const resolvedColor = useChangeRequestColors
          ? getChangeRequestStateColor(c.status)
          : resolveColorFromTheme(getStatusColor(c.status?.label), theme);

        return (
          <Form.CardButton
            key={c.id}
            onClick={() => onCaseClick?.(c)}
            sx={{
              p: 2,
              width: "100%",
              minWidth: 0,
              overflow: "hidden",
              display: "flex",
              flexDirection: "column",
              alignItems: "stretch",
              gap: 1,
            }}
          >
            <Form.CardHeader
              sx={{ p: 0 }}
              title={
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  flexWrap="wrap"
                >
                  {showInternalId && c.internalId && (
                    <>
                      <Typography
                        variant="body2"
                        fontWeight={500}
                        color="text.secondary"
                      >
                        {c.internalId}
                      </Typography>
                      <Typography variant="body2" color="text.disabled">
                        |
                      </Typography>
                    </>
                  )}
                  <Typography
                    variant="body2"
                    fontWeight={500}
                    color="text.primary"
                  >
                    {c.number}
                  </Typography>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        bgcolor: resolvedColor,
                        borderRadius: "50%",
                        flexShrink: 0,
                      }}
                    />
                    <Typography variant="caption" sx={{ color: resolvedColor }}>
                      {c.status?.label ?? NULL_PLACEHOLDER}
                    </Typography>
                  </Box>
                </Stack>
              }
            />

            <Form.CardContent sx={{ p: 0 }}>
              <Box
                sx={{
                  minWidth: 0,
                  display: "flex",
                  flexDirection: "column",
                  gap: 0.5,
                }}
              >
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
                  {stripHtml(c.title)}
                </Typography>
                <CaseCardDescriptionClamp
                  description={c.description}
                  emptyLabel=""
                  sx={{ mb: 0, minHeight: "2.4em" }}
                />
              </Box>
            </Form.CardContent>

            <Form.CardActions sx={{ p: 0, justifyContent: "flex-end", minWidth: 0 }}>
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1.5,
                  minWidth: 0,
                  maxWidth: "100%",
                }}
              >
                {(() => {
                  const label = getAssignedEngineerLabel(c.assignedEngineer);
                  return label ? (
                    <Tooltip title={`Assigned to ${label}`}>
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{
                          minWidth: 0,
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                        }}
                      >
                        Assigned to {label}
                      </Typography>
                    </Tooltip>
                  ) : null;
                })()}
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{ flexShrink: 0 }}
                >
                  {formatRelativeTime(c.createdOn ?? undefined)}
                </Typography>
              </Box>
            </Form.CardActions>
          </Form.CardButton>
        );
      })}
    </Box>
  );
}
