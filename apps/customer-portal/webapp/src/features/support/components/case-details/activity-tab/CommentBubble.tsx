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
import {
  Avatar,
  Box,
  IconButton,
  Paper,
  CircularProgress,
  Skeleton,
  Stack,
  Tooltip,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { Bot } from "@wso2/oxygen-ui-icons-react";
import { useMemo } from "react";
import {
  Download,
  File,
  FileArchive,
  FileText,
  Image,
} from "@wso2/oxygen-ui-icons-react";
import type { CaseComment } from "@features/support/types/cases";
import {
  formatFileSize,
  getAttachmentFileCategory,
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
import { useDarkMode } from "@utils/useDarkMode";
import ChatMessageCard from "@case-details-activity/ChatMessageCard";
import { useResolvedInlineImageHtml } from "@features/support/hooks/useResolvedInlineImageHtml";
import { useGetAttachment } from "@api/useGetAttachment";
import { useAttachmentPreview } from "@api/useAttachmentPreview";
import { stripLightModeInlineStyles } from "@/utils/common";

function commentAuthorDisplayName(comment: CaseComment): string {
  if (comment.createdByFullName?.trim()) {
    return comment.createdByFullName.trim();
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
  hideAvatar = false,
  userDetails,
}: CommentBubbleProps): import("react").JSX.Element {
  const theme = useTheme();
  const isDarkMode = useDarkMode();
  const { downloadAttachment, isDownloading, downloadingId } =
    useGetAttachment();
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
  const darkModeHtml = isDarkMode
    ? stripLightModeInlineStyles(withImages)
    : withImages;
  const sanitizedHtml = DOMPurify.sanitize(
    darkModeHtml,
    INLINE_COMMENT_HTML_PURIFY,
  );
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

  const isNovera = isNoveraOrBotSender(comment.createdBy, comment.type);
  const isAttachmentEntry = comment.type?.toLowerCase() === "attachment";
  const attachmentCategory = getAttachmentFileCategory(
    comment.fileName ?? "",
    comment.contentType ?? "",
  );
  const { data: imageDataUrl, isLoading: isImagePreviewLoading } =
    useAttachmentPreview(
      isAttachmentEntry && attachmentCategory === "image" ? comment.id : null,
    );

  const renderAttachmentIcon = () => {
    switch (attachmentCategory) {
      case "image":
        return <Image size={18} aria-hidden />;
      case "pdf":
      case "text":
        return <FileText size={18} aria-hidden />;
      case "archive":
        return <FileArchive size={18} aria-hidden />;
      default:
        return <File size={18} aria-hidden />;
    }
  };

  const handleAttachmentDownload = async () => {
    await downloadAttachment({
      id: comment.id,
      name: comment.fileName ?? "attachment",
      type: comment.contentType,
      downloadUrl: comment.downloadUrl,
    });
  };

  return (
    <Stack
      direction="row"
      alignItems="flex-start"
      sx={{
        gap: 2,
        minWidth: 0,
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
      ) : !hideAvatar ? (
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
      ) : (
        <Box sx={{ width: 32, flexShrink: 0 }} />
      )}
      <Stack
        spacing={0.75}
        sx={{
          width: "100%",
          minWidth: 0,
          alignItems: "flex-start",
        }}
      >
        <Stack
          direction="row"
          alignItems="center"
          flexWrap="wrap"
          sx={{
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
        {isAttachmentEntry ? (
          <Paper
            variant="outlined"
            sx={{
              width: "100%",
              p: 1.5,
              display: "flex",
              flexDirection: "column",
              gap: 1,
              bgcolor: "background.paper",
            }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                minWidth: 0,
              }}
            >
              <Box
                sx={{
                  width: 36,
                  height: 36,
                  bgcolor: "action.hover",
                  display: "inline-flex",
                  alignItems: "center",
                  justifyContent: "center",
                  color: "text.secondary",
                  flexShrink: 0,
                }}
              >
                {renderAttachmentIcon()}
              </Box>
              <Typography
                variant="body2"
                color="text.primary"
                sx={{ flex: 1, minWidth: 0 }}
                noWrap
              >
                {comment.fileName ?? "Attachment"}
              </Typography>
              <Tooltip title="Download attachment">
                <span>
                  <IconButton
                    size="small"
                    aria-label="Download attachment"
                    onClick={() => {
                      void handleAttachmentDownload();
                    }}
                    disabled={isDownloading && downloadingId === comment.id}
                  >
                    {isDownloading && downloadingId === comment.id ? (
                      <CircularProgress size={16} />
                    ) : (
                      <Download size={16} />
                    )}
                  </IconButton>
                </span>
              </Tooltip>
            </Box>
            <Stack
              direction="row"
              spacing={1}
              alignItems="center"
              flexWrap="wrap"
            >
              <Typography
                variant="caption"
                color="text.secondary"
                component="span"
              >
                {formatFileSize(comment.sizeBytes)}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                component="span"
              >
                •
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                component="span"
              >
                Uploaded by {comment.createdBy}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                component="span"
              >
                •
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                component="span"
              >
                {formatCommentDate(comment.createdOn)}
              </Typography>
            </Stack>
            {attachmentCategory === "image" &&
              (isImagePreviewLoading ? (
                <Skeleton
                  variant="rectangular"
                  width="100%"
                  height={160}
                  sx={{ borderRadius: 1 }}
                />
              ) : imageDataUrl ? (
                <Box
                  component="img"
                  src={imageDataUrl}
                  alt={comment.fileName ?? "Image attachment"}
                  sx={{
                    maxWidth: "100%",
                    borderRadius: 1,
                    display: "block",
                    cursor: "pointer",
                  }}
                  onClick={() => onImageClick?.(imageDataUrl)}
                />
              ) : null)}
          </Paper>
        ) : isImagesLoading ? (
          <Box
            sx={{
              width: "100%",
              display: "flex",
              flexDirection: "column",
              gap: 0.75,
            }}
          >
            <Skeleton
              variant="rectangular"
              width="100%"
              height={160}
              sx={{ borderRadius: 1 }}
            />
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
