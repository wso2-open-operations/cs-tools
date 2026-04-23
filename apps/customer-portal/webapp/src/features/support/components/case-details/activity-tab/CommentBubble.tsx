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

import type { CommentBubbleProps } from "@features/support/types/supportComponents";
import { Avatar, Box, Skeleton, Stack, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { Bot } from "@wso2/oxygen-ui-icons-react";
import { useMemo } from "react";
import type { CaseComment } from "@features/support/types/cases";
import {
  getInitials,
  hasSingleCodeWrapper,
  stripCodeWrapper,
  stripAllCodeBlocks,
  convertCodeTagsToHtml,
  trimLeadingBr,
  stripCustomerCommentAddedLabel,
  replaceInlineImageSources,
  formatCommentDate,
  INLINE_COMMENT_HTML_PURIFY,
  isNoveraOrBotSender,
} from "@features/support/utils/support";
import DOMPurify from "dompurify";
import ChatMessageCard from "@case-details-activity/ChatMessageCard";
import { useResolvedInlineImageHtml } from "@features/support/hooks/useResolvedInlineImageHtml";

function commentAuthorDisplayName(comment: CaseComment): string {
  const fromNames = [comment.createdByFirstName, comment.createdByLastName]
    .filter((p) => p != null && String(p).trim() !== "")
    .join(" ")
    .trim();
  if (fromNames.length > 0) {
    return fromNames;
  }
  return comment.createdBy?.trim() || "Unknown";
}

/**
 * Single comment bubble: avatar, display name, date, and ChatMessageCard.
 *
 * @param {CommentBubbleProps} props - Comment data and styling.
 * @returns {JSX.Element} The comment bubble.
 */
export default function CommentBubble({
  comment,
  isCurrentUser,
  primaryBg,
  onImageClick,
  userDetails,
}: CommentBubbleProps): import("react").JSX.Element {
  const theme = useTheme();
  const rawContent = comment.content ?? "";
  const isFullCodeWrap = hasSingleCodeWrapper(rawContent);
  const codeBlockCount = rawContent.match(/\[code\]/gi)?.length ?? 0;
  const afterCode = isFullCodeWrap
    ? stripCodeWrapper(rawContent)
    : codeBlockCount > 1
      ? stripAllCodeBlocks(rawContent)
      : convertCodeTagsToHtml(rawContent);
  const trimmedBr = trimLeadingBr(afterCode);
  const withoutLabel = stripCustomerCommentAddedLabel(trimmedBr);
  const withImages = replaceInlineImageSources(
    withoutLabel,
    comment.inlineAttachments,
  );
  const renderAsMarkdown = isNoveraOrBotSender(comment.createdBy, comment.type);
  const sanitizedHtml = DOMPurify.sanitize(withImages, INLINE_COMMENT_HTML_PURIFY);
  const { resolvedHtml: htmlContent, isLoading: isImagesLoading } =
    useResolvedInlineImageHtml(sanitizedHtml, comment.inlineAttachments);
  const displayName = useMemo(() => {
    if (isCurrentUser && userDetails) {
      const { firstName, lastName, email } = userDetails;
      const fromName =
        firstName != null || lastName != null
          ? [firstName, lastName].filter(Boolean).join(" ").trim()
          : "";
      if (fromName) return fromName;
      if (email) return email;
    }
    if (!isCurrentUser) {
      return commentAuthorDisplayName(comment);
    }
    return null;
  }, [isCurrentUser, comment, userDetails]);

  const initials = useMemo(() => {
    if (isCurrentUser && userDetails) {
      const { firstName, lastName, email } = userDetails;
      const fromName =
        firstName != null || lastName != null
          ? getInitials([firstName, lastName].filter(Boolean).join(" "))
          : "";
      if (fromName !== "--") return fromName;
      if (email) return getInitials(email);
    }
    if (!isCurrentUser) {
      return getInitials(commentAuthorDisplayName(comment));
    }
    return "?";
  }, [isCurrentUser, comment, userDetails]);

  const isRight = isCurrentUser;
  const isNovera = isNoveraOrBotSender(comment.createdBy, comment.type);

  return (
    <Stack
      direction="row"
      alignItems="flex-start"
      sx={{
        flexDirection: isRight ? "row-reverse" : "row",
        gap: 2,
      }}
    >
      {isNovera ? (
        <Box
          sx={{
            width: 32,
            height: 32,
            borderRadius: "50%",
            background: "linear-gradient(135deg, #EA580C 0%, #F97316 100%)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
        >
          <Bot size={16} color="white" />
        </Box>
      ) : (
        <Avatar
          sx={{
            width: 32,
            height: 32,
            fontSize: "0.75rem",
            flexShrink: 0,
            bgcolor: isCurrentUser
              ? primaryBg
              : alpha(theme.palette.info?.light ?? "#0288d1", 0.2),
            color: isCurrentUser
              ? theme.palette.primary.main
              : (theme.palette.info?.main ?? "#0288d1"),
          }}
        >
          {initials}
        </Avatar>
      )}
      <Stack
        spacing={0.75}
        sx={{
          width: "100%",
          maxWidth: 1000,
          minWidth: 0,
          alignItems: isRight ? "flex-end" : "flex-start",
        }}
      >
        <Stack
          direction="row"
          alignItems="center"
          flexWrap="wrap"
          sx={{
            flexDirection: isRight ? "row-reverse" : "row",
            gap: 1,
            minHeight: 32,
          }}
        >
          {displayName && (
            <Typography variant="body2" color="text.primary" fontWeight={500}>
              {displayName}
            </Typography>
          )}
          <Typography variant="caption" color="text.secondary">
            {formatCommentDate(comment.createdOn)}
          </Typography>
        </Stack>
        {isImagesLoading ? (
          <Box sx={{ width: "100%", display: "flex", flexDirection: "column", gap: 0.75 }}>
            <Skeleton variant="rectangular" width="100%" height={160} sx={{ borderRadius: 1 }} />
          </Box>
        ) : (
          <ChatMessageCard
            htmlContent={htmlContent}
            markdownContent={withoutLabel}
            renderAsMarkdown={renderAsMarkdown}
            isCurrentUser={isCurrentUser}
            primaryBg={primaryBg}
            onImageClick={onImageClick}
          />
        )}
      </Stack>
    </Stack>
  );
}
