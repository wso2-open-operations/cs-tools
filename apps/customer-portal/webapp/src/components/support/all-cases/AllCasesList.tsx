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
  Chip,
  Form,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { Calendar, FileText, User } from "@wso2/oxygen-ui-icons-react";
import type { JSX, KeyboardEvent } from "react";
import type { CaseListItem } from "@models/responses";
import { getSeverityLegendColor } from "@constants/dashboardConstants";
import {
  formatDateTime,
  getAssignedEngineerLabel,
  getStatusColor,
  getStatusIcon,
  mapSeverityToDisplay,
  resolveColorFromTheme,
  stripHtml,
} from "@utils/support";
import AllCasesListSkeleton from "@components/support/all-cases/AllCasesListSkeleton";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

export interface AllCasesListProps {
  cases: CaseListItem[];
  isLoading: boolean;
  isError?: boolean;
  onCaseClick?: (caseItem: CaseListItem) => void;
}

/**
 * Component to display cases as cards.
 *
 * @param {AllCasesListProps} props - Cases array and loading state.
 * @returns {JSX.Element} The rendered case cards list.
 */
export default function AllCasesList({
  cases,
  isLoading,
  isError = false,
  onCaseClick,
}: AllCasesListProps): JSX.Element {
  const theme = useTheme();

  if (isLoading) {
    return <AllCasesListSkeleton />;
  }

  if (isError) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <ErrorIndicator entityName="cases" size="medium" />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          Failed to load cases. Please try again.
        </Typography>
      </Box>
    );
  }

  if (cases.length === 0) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <Typography variant="body1" color="text.secondary">
          No cases found.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {cases.map((caseItem) => {
        const StatusIcon = getStatusIcon(caseItem.status?.label);
        const colorPath = getStatusColor(caseItem.status?.label);
        const resolvedColor = resolveColorFromTheme(colorPath, theme);

        return (
          <Form.CardButton
            key={caseItem.id}
            onClick={() => onCaseClick?.(caseItem)}
            tabIndex={onCaseClick ? 0 : undefined}
            role={onCaseClick ? "button" : undefined}
            onKeyDown={(event: KeyboardEvent<HTMLElement>) => {
              if (!onCaseClick) return;
              if (event.key === "Enter" || event.key === " " || event.key === "Spacebar") {
                event.preventDefault();
                onCaseClick(caseItem);
              }
            }}
            sx={{
              p: 3,
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
                  spacing={1.5}
                  alignItems="center"
                  sx={{ mb: 1, flexWrap: "wrap" }}
                >
                  <Typography
                    variant="body2"
                    fontWeight={500}
                    color="text.primary"
                  >
                    {caseItem.number || "--"}
                  </Typography>
                  <Chip
                    label={mapSeverityToDisplay(caseItem.severity?.label)}
                    size="small"
                    variant="outlined"
                    sx={{
                      bgcolor: alpha(
                        getSeverityLegendColor(caseItem.severity?.label),
                        0.1,
                      ),
                      color: getSeverityLegendColor(caseItem.severity?.label),
                      borderColor: alpha(
                        getSeverityLegendColor(caseItem.severity?.label),
                        0.3,
                      ),
                      fontWeight: 500,
                      px: 0,
                      height: 20,
                      fontSize: "0.75rem",
                      "& .MuiChip-label": {
                        pl: "6px",
                        pr: "6px",
                      },
                    }}
                  />
                  <Chip
                    size="small"
                    variant="outlined"
                    label={caseItem.status?.label || "--"}
                    icon={<StatusIcon size={12} />}
                    sx={{
                      bgcolor: alpha(resolvedColor, 0.1),
                      color: resolvedColor,
                      height: 20,
                      fontSize: "0.75rem",
                      px: 0,
                      "& .MuiChip-icon": {
                        color: "inherit",
                        ml: "6px",
                        mr: "6px",
                      },
                      "& .MuiChip-label": {
                        pl: 0,
                        pr: "6px",
                      },
                    }}
                  />
                  {caseItem.issueType?.label && (
                    <Chip
                      size="small"
                      label={caseItem.issueType.label || "--"}
                      variant="outlined"
                      sx={{
                        height: 20,
                        fontSize: "0.75rem",
                      }}
                    />
                  )}
                </Stack>
              }
            />

            <Form.CardContent sx={{ p: 0 }}>
              <Typography
                variant="h6"
                color="text.primary"
                sx={{ mb: 1, fontWeight: 500 }}
              >
                {caseItem.title || "--"}
              </Typography>

              <Typography
                variant="body2"
                color="text.secondary"
                sx={{
                  mb: 2,
                  display: "-webkit-box",
                  WebkitLineClamp: 2,
                  WebkitBoxOrient: "vertical",
                  overflow: "hidden",
                }}
              >
                {stripHtml(caseItem.description) || "--"}
              </Typography>
            </Form.CardContent>

            <Form.CardActions
              sx={{
                p: 0,
                justifyContent: "flex-start",
                flexWrap: "wrap",
                gap: 2,
              }}
            >
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 2,
                  flexWrap: "wrap",
                }}
              >
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 0.5,
                    flexShrink: 0,
                  }}
                >
                  <Calendar size={14} />
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ lineHeight: 1 }}
                  >
                    Created {formatDateTime(caseItem.createdOn) || "--"}
                  </Typography>
                </Box>
                {caseItem.createdBy && (
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.5,
                      flexShrink: 0,
                    }}
                  >
                    <User size={14} />
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ lineHeight: 1 }}
                    >
                      Created by {caseItem.createdBy}
                    </Typography>
                  </Box>
                )}
                {(() => {
                  const assignedLabel = getAssignedEngineerLabel(
                    caseItem.assignedEngineer,
                  );
                  return assignedLabel ? (
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 0.5,
                        flexShrink: 0,
                      }}
                    >
                      <User size={14} />
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ lineHeight: 1 }}
                      >
                        Assigned to {assignedLabel}
                      </Typography>
                    </Box>
                  ) : null;
                })()}
                {caseItem.deployment?.label && (
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.5,
                      flexShrink: 0,
                    }}
                  >
                    <FileText size={14} />
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ lineHeight: 1 }}
                    >
                      {caseItem.deployment.label}
                    </Typography>
                  </Box>
                )}
              </Box>
            </Form.CardActions>
          </Form.CardButton>
        );
      })}
    </Box>
  );
}
