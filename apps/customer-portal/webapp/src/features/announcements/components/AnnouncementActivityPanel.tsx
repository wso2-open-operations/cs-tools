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
  Avatar,
  Box,
  Button,
  Divider,
  Paper,
  Skeleton,
  Stack,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { useMemo, type JSX } from "react";
import useGetCaseCommentsInfinite from "@features/support/api/useGetCaseCommentsInfinite";
import type { CaseComment } from "@features/support/types/cases";
import {
  compareByCreatedOnThenId,
  collapseCommentSourceWhitespace,
  convertCodeTagsToHtml,
  formatCommentDate,
  getInitials,
  SYSTEM_AUTHOR_LABEL,
  hasDisplayableContent,
  hasSingleCodeWrapper,
  parseApiLocalDateTimeMs,
  plainTextCommentToHtml,
  stripAllCodeBlocks,
  stripCodeWrapper,
  stripCustomerCommentAddedLabel,
  trimLeadingBr,
} from "@features/support/utils/support";
import ApiErrorState from "@components/error/ApiErrorState";
import { stripLightModeInlineStyles } from "@/utils/common";
import { useDarkMode } from "@utils/useDarkMode";
import {
  ANNOUNCEMENT_COMMENTS_ERROR_MESSAGE,
  ANNOUNCEMENT_COMMENTS_LOADING_LABEL,
  ANNOUNCEMENT_COMMENTS_LOAD_MORE_LABEL,
} from "@features/announcements/constants/announcementsConstants";
import {
  isDescriptionEchoedInComment,
  sanitizeAnnouncementHtml,
} from "@features/announcements/utils/announcements";

type AnnouncementActivityPanelProps = {
  projectId: string;
  caseId: string;
  /**
   * The announcement's description HTML. When the oldest comment is just an
   * echo of it (ServiceNow writes one when the announcement case is created),
   * that comment is left out here because the Description card already
   * shows the same body.
   */
  description?: string | null;
};

function commentAuthorDisplayName(comment: CaseComment): string {
  if (comment.createdByFullName?.trim()) {
    return comment.createdByFullName.trim();
  }
  return comment.createdBy?.trim() || SYSTEM_AUTHOR_LABEL;
}

function sanitizeCommentContent(content: string, isDarkMode: boolean): string {
  const rawContent = plainTextCommentToHtml(
    collapseCommentSourceWhitespace(content ?? ""),
  );
  const isFullCodeWrap = hasSingleCodeWrapper(rawContent);
  const codeBlockCount = (rawContent.match(/\[code\]/gi) ?? []).length;
  const afterCode = isFullCodeWrap
    ? stripCodeWrapper(rawContent)
    : codeBlockCount > 1
      ? stripAllCodeBlocks(rawContent)
      : convertCodeTagsToHtml(rawContent);
  const trimmedBr = trimLeadingBr(afterCode);
  const withoutLabel = stripCustomerCommentAddedLabel(trimmedBr);
  const darkModeHtml = isDarkMode
    ? stripLightModeInlineStyles(withoutLabel)
    : withoutLabel;
  return sanitizeAnnouncementHtml(darkModeHtml);
}

/**
 * AnnouncementActivityPanel shows the latest comments for an announcement (case),
 * newest first, with each comment rendered as its own card matching the description card style.
 *
 * @param props - projectId and caseId of the announcement to fetch comments for, plus its description (used to drop the origin comment that merely repeats it).
 * @returns {JSX.Element | null} The rendered comments section, or null when there is nothing to show.
 */
export default function AnnouncementActivityPanel({
  projectId,
  caseId,
  description,
}: AnnouncementActivityPanelProps): JSX.Element | null {
  const isDarkMode = useDarkMode();
  const theme = useTheme();

  const {
    data: commentsData,
    isLoading,
    isError,
    error,
    hasNextPage,
    isFetchingNextPage,
    fetchNextPage,
  } = useGetCaseCommentsInfinite(projectId, caseId);

  const commentsToShow = useMemo(() => {
    const comments = commentsData?.pages?.flatMap((page) => page.comments) ?? [];
    const newestFirst = comments
      .filter((comment) => comment.type?.toLowerCase() !== "attachment")
      .filter(hasDisplayableContent)
      .sort((a, b) => -compareByCreatedOnThenId(a, b));

    // Creating the announcement case makes ServiceNow write an "origin"
    // comment that mirrors the description, which the Description card below
    // already renders -- so the same body showed up twice. The origin comment
    // is the oldest one, i.e. the last of the newest-first list, and is only
    // known to be the oldest once every page has loaded. Comparing the
    // rendered text (what the reader would see) keeps this tied to an actual
    // visible duplicate; a comment that differs from the description is
    // always kept.
    if (hasNextPage || newestFirst.length === 0) {
      return newestFirst;
    }
    const oldest = newestFirst[newestFirst.length - 1];
    // Only when it is unmistakably the oldest: a lone comment, or one with a
    // valid timestamp strictly earlier than the next-oldest. On a tie the sort
    // falls back to the id, so the last item could be a later post that merely
    // repeats the description, and an unparsable timestamp says nothing about
    // order; in both cases the comment stays.
    const oldestTime = parseApiLocalDateTimeMs(oldest.createdOn);
    const nextOldestTime = parseApiLocalDateTimeMs(
      newestFirst[newestFirst.length - 2]?.createdOn,
    );
    const isClearlyOldest =
      newestFirst.length === 1 ||
      (Number.isFinite(oldestTime) &&
        Number.isFinite(nextOldestTime) &&
        oldestTime < nextOldestTime);
    if (
      isClearlyOldest &&
      isDescriptionEchoedInComment(
        description,
        sanitizeCommentContent(oldest.content, isDarkMode),
      )
    ) {
      return newestFirst.slice(0, -1);
    }
    return newestFirst;
  }, [commentsData?.pages, hasNextPage, description, isDarkMode]);

  if (
    !isLoading &&
    !isError &&
    commentsToShow.length === 0 &&
    !hasNextPage
  ) {
    return null;
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {isLoading ? (
        <Stack spacing={2}>
          {[1, 2].map((i) => (
            <Paper
              key={i}
              variant="outlined"
              elevation={0}
              sx={{ p: 4, display: "flex", flexDirection: "column", gap: 1 }}
            >
              <Skeleton width="30%" height={20} />
              <Skeleton width="100%" height={48} sx={{ mt: 1 }} variant="rounded" />
            </Paper>
          ))}
        </Stack>
      ) : isError ? (
        <ApiErrorState
          error={error}
          fallbackMessage={ANNOUNCEMENT_COMMENTS_ERROR_MESSAGE}
        />
      ) : (
        <Stack divider={<Divider />} spacing={2}>
          {commentsToShow.map((comment) => (
            <Stack
              key={comment.id}
              direction="row"
              alignItems="flex-start"
              spacing={1.5}
            >
              <Avatar
                sx={{
                  width: 28,
                  height: 28,
                  fontSize: "0.7rem",
                  flexShrink: 0,
                  bgcolor: alpha(theme.palette.info?.light ?? "#0288d1", 0.2),
                  color: theme.palette.info?.main ?? "#0288d1",
                }}
              >
                {getInitials(commentAuthorDisplayName(comment))}
              </Avatar>
              <Stack sx={{ minWidth: 0, flex: 1, gap: 1 }}>
                <Stack
                  direction="row"
                  alignItems="center"
                  flexWrap="wrap"
                  sx={{ gap: 1 }}
                >
                  <Typography variant="body2" color="text.primary" fontWeight={500}>
                    {commentAuthorDisplayName(comment)}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {formatCommentDate(comment.createdOn)}
                  </Typography>
                </Stack>
                <Paper
                  variant="outlined"
                  elevation={0}
                  component="div"
                  sx={{
                    typography: "body2",
                    color: "text.primary",
                    bgcolor: "background.paper",
                    borderRadius: 1,
                    p: 1.5,
                    maxWidth: "100%",
                    overflowX: "auto",
                    "& p": { mb: 0.5 },
                    "& p:last-child": { mb: 0 },
                    // Lexical exports a nested list as a wrapper <li class="editor-list-item-nested"> holding only the sub-list;
                    // its marker would be a stray empty bullet (CSM portal does the same).
                    "& li.editor-list-item-nested": { listStyleType: "none" },
                    "& img": {
                      maxWidth: "100%",
                      height: "auto",
                      borderRadius: 1,
                      display: "block",
                    },
                    // Mirrors ChatMessageCard's own table styling so a table
                    // in an announcement's origin comment renders the same
                    // way one already does in a support-case comment,
                    // instead of the unstyled, borderless default a bare
                    // `<table>` gets here.
                    "& table": {
                      width: "100%",
                      minWidth: 420,
                      borderCollapse: "collapse",
                      mb: 1,
                    },
                    "& tr": {
                      borderBottom: "1px solid",
                      borderColor: "divider",
                    },
                    "& th": {
                      textAlign: "left",
                      p: 1,
                      fontSize: "0.75rem",
                      fontWeight: 600,
                      color: "text.secondary",
                    },
                    "& td": {
                      p: 1,
                      fontSize: "0.8125rem",
                      verticalAlign: "top",
                    },
                  }}
                  // biome-ignore lint/security/noDangerouslySetInnerHtml: sanitized with DOMPurify
                  dangerouslySetInnerHTML={{
                    __html: sanitizeCommentContent(comment.content, isDarkMode),
                  }}
                />
              </Stack>
            </Stack>
          ))}
        </Stack>
      )}

      {hasNextPage && (
        <Button
          variant="text"
          size="small"
          onClick={() => void fetchNextPage()}
          disabled={isFetchingNextPage}
          sx={{ alignSelf: "flex-start" }}
        >
          {isFetchingNextPage
            ? ANNOUNCEMENT_COMMENTS_LOADING_LABEL
            : ANNOUNCEMENT_COMMENTS_LOAD_MORE_LABEL}
        </Button>
      )}
    </Box>
  );
}
