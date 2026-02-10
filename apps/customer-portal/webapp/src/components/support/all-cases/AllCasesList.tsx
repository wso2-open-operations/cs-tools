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
import type { JSX } from "react";
import type { CaseListItem } from "@models/responses";
import { getPriorityColor, getStatusColor } from "@utils/casesTable";
import {
  formatRelativeTime,
  resolveColorFromTheme,
  getStatusIcon,
} from "@utils/support";
import AllCasesListSkeleton from "@components/support/all-cases/AllCasesListSkeleton";

export interface AllCasesListProps {
  cases: CaseListItem[];
  isLoading: boolean;
}

/**
 * AllCasesList component to display cases as cards.
 *
 * @param {AllCasesListProps} props - Cases array and loading state.
 * @returns {JSX.Element} The rendered case cards list.
 */
export default function AllCasesList({
  cases,
  isLoading,
}: AllCasesListProps): JSX.Element {
  const theme = useTheme();

  if (isLoading) {
    return <AllCasesListSkeleton />;
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
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 0.5,
                    }}
                  >
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: "50%",
                        bgcolor: `${getPriorityColor(caseItem.severity?.label)}.main`,
                      }}
                    />
                    <Typography variant="caption" color="text.secondary">
                      {caseItem.severity?.label || "—"}
                    </Typography>
                  </Box>
                  <Chip
                    size="small"
                    variant="outlined"
                    label={caseItem.status?.label || "—"}
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
                  {caseItem.type?.label && (
                    <Chip
                      size="small"
                      label={caseItem.type.label || "--"}
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
                {caseItem.description || "--"}
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
                    Created {formatRelativeTime(caseItem.createdOn) || "--"}
                  </Typography>
                </Box>
                {caseItem.assignedEngineer && (
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
                      Assigned to {caseItem.assignedEngineer}
                    </Typography>
                  </Box>
                )}
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
