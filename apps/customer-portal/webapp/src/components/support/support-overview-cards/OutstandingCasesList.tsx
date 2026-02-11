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
  Tooltip,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { Clock } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { CaseListItem } from "@models/responses";
import OutstandingCasesSkeleton from "./OutstandingCasesSkeleton";
import { getPriorityColor, getStatusColor } from "@utils/casesTable";
import { formatRelativeTime, resolveColorFromTheme } from "@utils/support";

export interface OutstandingCasesListProps {
  cases: CaseListItem[];
  isLoading?: boolean;
}

/**
 * Renders a list of outstanding case rows for the support overview card.
 *
 * @param {OutstandingCasesListProps} props - Cases array and loading state.
 * @returns {JSX.Element} The list of case rows.
 */
export default function OutstandingCasesList({
  cases,
  isLoading,
}: OutstandingCasesListProps): JSX.Element {
  const theme = useTheme();

  if (isLoading) {
    return <OutstandingCasesSkeleton />;
  }

  if (cases.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No outstanding cases.
      </Typography>
    );
  }

  return (
    <Box
      sx={{ display: "flex", flexDirection: "column", gap: 1.5, width: "100%" }}
    >
      {cases.map((c) => {
        const colorPath = getStatusColor(c.status?.label);
        const resolvedColor = resolveColorFromTheme(colorPath, theme);

        return (
          <Form.CardButton
            key={c.id}
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
                <Stack direction="row" spacing={1} alignItems="center">
                  <Typography
                    variant="body2"
                    fontWeight={500}
                    color="text.primary"
                  >
                    {c.number}
                  </Typography>
                  <Box
                    sx={{
                      width: 8,
                      height: 8,
                      borderRadius: "50%",
                      flexShrink: 0,
                      bgcolor: `${getPriorityColor(c.severity?.label)}.main`,
                    }}
                  />
                  <Typography variant="caption" color="text.secondary">
                    {c.severity?.label ?? "—"}
                  </Typography>
                </Stack>
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
                  {c.title}
                </Typography>
              </Box>
            </Form.CardContent>

            <Form.CardActions sx={{ p: 0, justifyContent: "space-between" }}>
              <Chip
                size="small"
                variant="outlined"
                label={c.status?.label ?? "—"}
                icon={<Clock size={12} />}
                sx={{
                  bgcolor: alpha(resolvedColor, 0.1),
                  color: resolvedColor,
                  px: 0,
                  height: 20,
                  fontSize: "0.75rem",
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
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                {(() => {
                  const raw = c.assignedEngineer;
                  const label =
                    raw == null
                      ? ""
                      : typeof raw === "object" && "label" in raw
                        ? raw.label
                        : String(raw);
                  return label ? (
                    <Tooltip title={`Assigned to ${label}`}>
                      <Typography variant="caption" color="text.secondary">
                        Assigned to {label}
                      </Typography>
                    </Tooltip>
                  ) : null;
                })()}
                <Typography variant="caption" color="text.secondary">
                  {formatRelativeTime(c.createdOn)}
                </Typography>
              </Box>
            </Form.CardActions>
          </Form.CardButton>
        );
      })}
    </Box>
  );
}
