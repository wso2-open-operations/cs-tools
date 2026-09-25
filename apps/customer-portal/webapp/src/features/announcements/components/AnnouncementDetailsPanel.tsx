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
  Button,
  Chip,
  Paper,
  Skeleton,
  Stack,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";
import { ArrowLeft, Calendar, FileText } from "@wso2/oxygen-ui-icons-react";
import { INLINE_COMMENT_HTML_PURIFY } from "@features/support/utils/support";
import type { JSX } from "react";
import CaseDetailsActionRow from "@features/support/components/case-details/header/CaseDetailsActionRow";
import {
  getStatusColor,
  resolveColorFromTheme,
} from "@features/support/utils/support";
import ApiErrorState from "@components/error/ApiErrorState";
import AnnouncementActivityPanel from "@features/announcements/components/AnnouncementActivityPanel";
import type { AnnouncementDetailsPanelProps } from "@features/announcements/types/announcements";
import {
  ANNOUNCEMENT_DETAILS_BODY_EMPTY,
  ANNOUNCEMENT_DETAILS_DESCRIPTION_HEADING,
  ANNOUNCEMENT_DETAILS_ERROR_MESSAGE,
  ANNOUNCEMENTS_BACK_LABEL,
} from "@features/announcements/constants/announcementsConstants";
import {
  formatAnnouncementDateDisplay,
  isAnnouncementDescriptionEffectivelyEmpty,
  normalizeAnnouncementDescriptionHtml,
} from "@features/announcements/utils/announcements";
import { useDarkMode } from "@utils/useDarkMode";

/**
 * AnnouncementDetailsPanel displays announcement details: Back, title, date, issue type, state management buttons, and description.
 *
 * @param props - Data, loading/error state, onBack, projectId, caseId.
 * @returns {JSX.Element} The rendered announcement details panel.
 */
export default function AnnouncementDetailsPanel({
  data,
  isLoading,
  isError,
  error,
  onBack,
  projectId = "",
  caseId = "",
}: AnnouncementDetailsPanelProps): JSX.Element {
  const theme = useTheme();
  const isDarkMode = useDarkMode();

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={onBack}
          sx={{ mb: 0, alignSelf: "flex-start" }}
          variant="text"
        >
          {ANNOUNCEMENTS_BACK_LABEL}
        </Button>
        <Paper variant="outlined" sx={{ p: 4 }}>
          <Skeleton width="60%" height={28} sx={{ mb: 2 }} />
          <Stack direction="row" spacing={2} sx={{ mb: 2 }}>
            <Skeleton width={120} height={20} />
            <Skeleton width={80} height={20} />
          </Stack>
        </Paper>
        <Paper variant="outlined" sx={{ p: 4 }}>
          <Skeleton width="25%" height={24} sx={{ mb: 2 }} />
          <Skeleton width="100%" height={80} variant="rounded" />
        </Paper>
      </Box>
    );
  }

  if (isError || !data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={onBack}
          sx={{ mb: 0, alignSelf: "flex-start" }}
          variant="text"
        >
          {ANNOUNCEMENTS_BACK_LABEL}
        </Button>
        <ApiErrorState
          error={error}
          fallbackMessage={ANNOUNCEMENT_DETAILS_ERROR_MESSAGE}
        />
      </Box>
    );
  }

  const statusLabel = data.status?.label;
  const statusColorPath = getStatusColor(statusLabel ?? undefined);
  const resolvedStatusColor = resolveColorFromTheme(statusColorPath, theme);
  const updatedOnLabel = formatAnnouncementDateDisplay(data.updatedOn);
  // announcementType is the source of truth going forward, but a case
  // created before that column existed always reads back "GENERAL" (its
  // default -- see migration 000084_announcement_add_type, which added no
  // backfill for pre-existing rows) even though it still carries the
  // mandatory "Security Announcement" tag. Checking both keeps the chip
  // showing for that historical data without giving the tag priority over
  // the real column for anything created after this switch.
  const isSecurityAnnouncement =
    data.announcementType === "SECURITY" ||
    (data.tags ?? []).some(
      (t) => t.label.toLowerCase() === "security announcement",
    );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Button
        startIcon={<ArrowLeft size={16} />}
        onClick={onBack}
        sx={{ mb: 0, alignSelf: "flex-start" }}
        variant="text"
      >
        {ANNOUNCEMENTS_BACK_LABEL}
      </Button>

      <Paper
        variant="outlined"
        elevation={0}
        sx={{
          p: 4,
          display: "flex",
          flexDirection: "column",
          gap: 1,
        }}
      >
        {(data.number || data.internalId) && (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 1,
              mb: 0.5,
              flexWrap: "wrap",
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
              {data.internalId && (
                <>
                  <Typography variant="caption" color="text.secondary" fontWeight={500}>
                    {data.internalId}
                  </Typography>
                  <Typography variant="caption" color="text.disabled">|</Typography>
                </>
              )}
              <Typography variant="caption" color="text.secondary">
                {data.number}
              </Typography>
              {statusLabel && (
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 0.75,
                  }}
                >
                  <Box
                    sx={{
                      width: 8,
                      height: 8,
                      borderRadius: "50%",
                      bgcolor: resolvedStatusColor,
                      flexShrink: 0,
                    }}
                  />
                  <Typography variant="caption" color="text.secondary">
                    {statusLabel}
                  </Typography>
                </Box>
              )}
            </Box>
            {data.status?.label?.toLowerCase() !== "closed" && (
              <CaseDetailsActionRow
                projectId={projectId}
                caseId={caseId}
                statusLabel={data.status?.label}
                assignedEngineer={data.assignedEngineer}
                engineerInitials={
                  typeof data.assignedEngineer === "object" &&
                  data.assignedEngineer?.name
                    ? (data.assignedEngineer.name.split(" ")[0]?.[0] ?? "")
                    : ""
                }
                closedOn={data.closedOn}
                restrictToCloseOnly={true}
              />
            )}
          </Box>
        )}

        <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1 }}>
          <Typography variant="h6" color="text.primary" sx={{ fontWeight: 500 }}>
            {data.title || "--"}
          </Typography>
          {isSecurityAnnouncement && (
            <Chip size="small" color="warning" label="Security" sx={{ flexShrink: 0 }} />
          )}
        </Box>

        <Stack
          direction="row"
          spacing={2}
          alignItems="center"
          sx={{ flexWrap: "wrap", gap: 1 }}
        >
          <Stack direction="row" spacing={1} alignItems="center">
            <Calendar
              size={16}
              color={theme.palette.text.secondary}
              aria-hidden
            />
            <Typography variant="body2" color="text.secondary">
              {updatedOnLabel || "--"}
            </Typography>
          </Stack>
          {data.issueType?.label && (
            <Stack direction="row" spacing={1} alignItems="center">
              <FileText
                size={16}
                color={theme.palette.text.secondary}
                aria-hidden
              />
              <Typography variant="body2" color="text.secondary">
                {data.issueType.label}
              </Typography>
            </Stack>
          )}
        </Stack>
      </Paper>

      {projectId && caseId && (
        <AnnouncementActivityPanel projectId={projectId} caseId={caseId} />
      )}

      <Paper
        variant="outlined"
        elevation={0}
        sx={{
          p: 4,
          display: "flex",
          flexDirection: "column",
          gap: 2,
        }}
      >
        <Typography variant="h6" color="text.primary">
          {ANNOUNCEMENT_DETAILS_DESCRIPTION_HEADING}
        </Typography>
        {data.description ? (
          (() => {
            const normalizedHtml = normalizeAnnouncementDescriptionHtml(
              data.description,
              isDarkMode,
            );
            const isEffectivelyEmpty =
              isAnnouncementDescriptionEffectivelyEmpty(data.description);

            if (isEffectivelyEmpty) {
              return (
                <Typography variant="body2" color="text.primary">
                  {ANNOUNCEMENT_DETAILS_BODY_EMPTY}
                </Typography>
              );
            }

            return (
              <Box
                component="div"
                sx={{
                  typography: "body2",
                  color: "text.primary",
                  "& p": { mb: 0.5 },
                  "& p:last-child": { mb: 0 },
                  "& code": {
                    display: "block",
                    p: 1,
                    bgcolor: "action.hover",
                    fontSize: "0.875rem",
                    whiteSpace: "pre-wrap",
                    overflowWrap: "break-word",
                  },
                }}
                // biome-ignore lint/security/noDangerouslySetInnerHtml: sanitized with DOMPurify
                dangerouslySetInnerHTML={{
                  // DESCRIPTION_PURIFY_CONFIG (case/change-request descriptions)
                  // forbids tables and code blocks -- wrong policy for an
                  // announcement, whose description legitimately carries
                  // things like an EOL product-version table. Use the same
                  // permissive (default-policy) sanitizer this feature's own
                  // AnnouncementActivityPanel already uses for comments.
                  __html: DOMPurify.sanitize(normalizedHtml, INLINE_COMMENT_HTML_PURIFY),
                }}
              />
            );
          })()
        ) : (
          <Typography variant="body2" color="text.primary">
            {ANNOUNCEMENT_DETAILS_BODY_EMPTY}
          </Typography>
        )}
      </Paper>
    </Box>
  );
}
