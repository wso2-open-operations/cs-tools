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

import { Box, Chip, Form, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { Calendar, FileText } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { CaseListItem } from "@models/responses";
import {
  formatUtcToLocalNoTimezone,
  stripHtml,
  getStatusColor,
  getStatusIconElement,
  resolveColorFromTheme,
} from "@utils/support";
import AllCasesListSkeleton from "@components/support/all-cases/AllCasesListSkeleton";

export interface AnnouncementListProps {
  cases: CaseListItem[];
  isLoading: boolean;
  onCaseClick?: (caseItem: CaseListItem) => void;
}

/**
 * Component to display announcements using the same card layout as case list (Form.CardButton).
 * No severity icon/chip, no View Details / Mark Read / Archive; whole card navigates to detail.
 *
 * @param {AnnouncementListProps} props - Announcements array, loading state, and handlers.
 * @returns {JSX.Element} The rendered announcement cards list.
 */
export default function AnnouncementList({
  cases,
  isLoading,
  onCaseClick,
}: AnnouncementListProps): JSX.Element {
  const theme = useTheme();

  if (isLoading) {
    return <AllCasesListSkeleton compact />;
  }

  if (cases.length === 0) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <Typography variant="body1" color="text.secondary">
          No announcements found.
        </Typography>
      </Box>
    );
  }

  const cardSx = {
    p: 3,
    display: "flex" as const,
    flexDirection: "column" as const,
    alignItems: "stretch" as const,
    gap: 1,
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {cases.map((caseItem) => {
        const statusLabel = caseItem.status?.label;
        const statusColor = getStatusColor(statusLabel);
        const resolvedStatusColor = resolveColorFromTheme(statusColor, theme);
        const statusChipIcon = getStatusIconElement(statusLabel, 12);

        const cardContent = (
          <>
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
                  <Typography
                    variant="body2"
                    fontWeight={500}
                    color="text.primary"
                  >
                    {caseItem.number || "--"}
                  </Typography>
                  {statusLabel && statusChipIcon && (
                    <Chip
                      size="small"
                      variant="outlined"
                      label={statusLabel}
                      icon={statusChipIcon as React.ReactElement}
                      sx={{
                        bgcolor: alpha(resolvedStatusColor, 0.1),
                        color: resolvedStatusColor,
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
                  )}
                </Box>
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
                    Created {formatUtcToLocalNoTimezone(caseItem.createdOn)}
                  </Typography>
                </Box>
                {caseItem.issueType?.label && (
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
                      {caseItem.issueType.label}
                    </Typography>
                  </Box>
                )}
              </Box>
            </Form.CardActions>
          </>
        );
        return onCaseClick ? (
          <Form.CardButton
            key={caseItem.id}
            onClick={() => onCaseClick(caseItem)}
            sx={cardSx}
          >
            {cardContent}
          </Form.CardButton>
        ) : (
          <Box key={caseItem.id} sx={cardSx}>
            {cardContent}
          </Box>
        );
      })}
    </Box>
  );
}
