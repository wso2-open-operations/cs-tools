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
import CaseCardDescriptionClamp from "@components/list-view/CaseCardDescriptionClamp";
import {
  getStatusColor,
  getStatusIconElement,
  resolveColorFromTheme,
} from "@features/support/utils/support";
import { formatAnnouncementDateDisplay } from "@features/announcements/utils/announcements";

import ListSkeleton from "@components/list-view/ListSkeleton";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import SearchNoResultsIcon from "@components/empty-state/SearchNoResultsIcon";
import type { AnnouncementListProps } from "@features/announcements/types/announcements";
import {
  ANNOUNCEMENTS_EMPTY_MESSAGE_DEFAULT,
  ANNOUNCEMENTS_EMPTY_MESSAGE_REFINED,
  ANNOUNCEMENTS_EMPTY_STATE_CONTAINER_PY,
  ANNOUNCEMENTS_EMPTY_STATE_ICON_MARGIN_BOTTOM_PX,
  ANNOUNCEMENTS_EMPTY_STATE_ICON_MAX_WIDTH_PX,
} from "@features/announcements/constants/announcementsConstants";

/**
 * Component to display announcements using the same card layout as case list (Form.CardButton).
 * No severity icon/chip, no View Details / Mark Read / Archive; whole card navigates to detail.
 *
 * @param props - Announcements array, loading state, and handlers.
 * @returns {JSX.Element} The rendered announcement cards list.
 */
export default function AnnouncementList({
  cases,
  isLoading,
  hasListRefinement = false,
  onCaseClick,
}: AnnouncementListProps): JSX.Element {
  const theme = useTheme();

  if (isLoading) {
    return <ListSkeleton compact />;
  }

  if (cases.length === 0) {
    const containerSx = {
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      py: ANNOUNCEMENTS_EMPTY_STATE_CONTAINER_PY,
    } as const;
    const iconStyle = {
      width: ANNOUNCEMENTS_EMPTY_STATE_ICON_MAX_WIDTH_PX,
      maxWidth: "100%",
      height: "auto",
      marginBottom: ANNOUNCEMENTS_EMPTY_STATE_ICON_MARGIN_BOTTOM_PX,
    };
    const EmptyStateIcon = hasListRefinement ? SearchNoResultsIcon : EmptyIcon;
    const emptyMessage = hasListRefinement
      ? ANNOUNCEMENTS_EMPTY_MESSAGE_REFINED
      : ANNOUNCEMENTS_EMPTY_MESSAGE_DEFAULT;
    return (
      <Box sx={containerSx}>
        <EmptyStateIcon style={iconStyle} />
        <Typography variant="body1" color="text.secondary">
          {emptyMessage}
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
        const createdOnLabel = formatAnnouncementDateDisplay(caseItem.createdOn);

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

              <CaseCardDescriptionClamp description={caseItem.description} />
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
                    Created {createdOnLabel}
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
