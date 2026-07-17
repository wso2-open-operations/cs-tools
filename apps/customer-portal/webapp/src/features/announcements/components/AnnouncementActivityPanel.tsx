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
import DOMPurify from "dompurify";
import { useMemo, type JSX } from "react";
import useGetCaseCommentsInfinite from "@features/support/api/useGetCaseCommentsInfinite";
import type { CaseComment } from "@features/support/types/cases";
import {
  compareByCreatedOnThenId,
  convertCodeTagsToHtml,
  formatCommentDate,
  getInitials,
  hasDisplayableContent,
  hasSingleCodeWrapper,
  INLINE_COMMENT_HTML_PURIFY,
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

type AnnouncementActivityPanelProps = {
  projectId: string;
  caseId: string;
};

function commentAuthorDisplayName(comment: CaseComment): string {
  if (comment.createdByFullName?.trim()) {
    return comment.createdByFullName.trim();
  }
  return comment.createdBy?.trim() || "Unknown";
}

function sanitizeCommentContent(content: string, isDarkMode: boolean): string {
  const rawContent = content ?? "";
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
  return DOMPurify.sanitize(darkModeHtml, INLINE_COMMENT_HTML_PURIFY);
}

/**
 * AnnouncementActivityPanel shows the latest comments for an announcement (case),
 * newest first, with each comment rendered as its own card matching the description card style.
 *
 * @param props - projectId and caseId of the announcement to fetch comments for.
 * @returns {JSX.Element | null} The rendered comments section, or null when there is nothing to show.
 */
export default function AnnouncementActivityPanel({
  projectId,
  caseId,
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
    return comments
      .filter((comment) => comment.type?.toLowerCase() !== "attachment")
      .filter(hasDisplayableContent)
      .sort((a, b) => -compareByCreatedOnThenId(a, b));
  }, [commentsData?.pages]);

  if (!isLoading && !isError && commentsToShow.length === 0) {
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
                    "& img": {
                      maxWidth: "100%",
                      height: "auto",
                      borderRadius: 1,
                      display: "block",
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
