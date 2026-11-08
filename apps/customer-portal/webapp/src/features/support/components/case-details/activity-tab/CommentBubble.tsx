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
  Button,
  IconButton,
  Paper,
  Skeleton,
  Stack,
  Tooltip,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { Bot } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type KeyboardEvent } from "react";
import {
  Download,
  File,
  FileArchive,
  FileText,
  Image,
} from "@wso2/oxygen-ui-icons-react";
import type { CaseComment } from "@features/support/types/cases";
import {
  downloadSizeOf,
  formatFileSize,
  getAttachmentFileCategory,
  getInitials,
  collapseCommentSourceWhitespace,
  plainTextCommentToHtml,
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
import { useStoredAttachmentDownload } from "@api/useStoredAttachments";
import DownloadProgressIndicator from "@components/download-progress/DownloadProgressIndicator";
import DownloadProgressStatus from "@components/download-progress/DownloadProgressStatus";
import { useAttachmentPreview } from "@api/useAttachmentPreview";
import { stripLightModeInlineStyles } from "@/utils/common";
import { stripThinkingBlocks } from "@features/support/utils/chat";
import { NOVERA_DISPLAY_NAME } from "@features/support/constants/chatConstants";

function commentAuthorDisplayName(comment: CaseComment): string {
  if (comment.createdByFullName?.trim()) {
    return comment.createdByFullName.trim();
  }
  if (comment.createdBy?.trim()) {
    return comment.createdBy.trim();
  }
  // An empty createdBy is how the API represents a Novera/bot message (see
  // isNoveraOrBotSender) -- never show that as "Unknown".
  return isNoveraOrBotSender(comment.createdBy, comment.type)
    ? NOVERA_DISPLAY_NAME
    : "Unknown";
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
  const { downloadStoredAttachment, isDownloadingId, downloadProgressOf } =
    useStoredAttachmentDownload();
  // The case Activity tab merges the linked Novera chat into the timeline, so a
  // Novera answer can arrive here still carrying its <thinking> reasoning.
  const isNoveraComment = isNoveraOrBotSender(comment.createdBy, comment.type);
  const rawContent = isNoveraComment
    ? stripThinkingBlocks(comment.content ?? "")
    : plainTextCommentToHtml(
        collapseCommentSourceWhitespace(comment.content ?? ""),
      );
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
  const renderAsMarkdown = isNoveraComment;
  const darkModeHtml = isDarkMode
    ? stripLightModeInlineStyles(withImages)
    : withImages;
  const sanitizedHtml = DOMPurify.sanitize(
    darkModeHtml,
    INLINE_COMMENT_HTML_PURIFY,
  );
  const { resolvedHtml: htmlContent, isLoading: isImagesLoading } =
    useResolvedInlineImageHtml(sanitizedHtml);
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

  const isNovera = isNoveraComment;
  const isAttachmentEntry = comment.type?.toLowerCase() === "attachment";
  const attachmentCategory = getAttachmentFileCategory(
    comment.fileName ?? "",
    comment.contentType ?? "",
  );
  const isPreviewableImage =
    attachmentCategory === "image" &&
    /\.(png|jpe?g|webp)$/i.test(comment.fileName ?? "");
  // An attached image is only fetched once the person asks: each preview
  // streams the whole file through the backend, and a busy feed would otherwise
  // fetch every image on every render.
  const [previewRequested, setPreviewRequested] = useState(false);
  const {
    data: imageDataUrl,
    isLoading: isImagePreviewLoading,
    isError: isImagePreviewError,
    refetch: refetchImagePreview,
  } = useAttachmentPreview(
    isAttachmentEntry && isPreviewableImage && previewRequested
      ? comment.id
      : null,
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
    await downloadStoredAttachment({
      type: "work_item",
      id: comment.id,
      name: comment.fileName ?? "attachment",
      size: downloadSizeOf(comment.sizeBytes),
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
                  <DownloadProgressStatus
                    name={comment.fileName ?? "attachment"}
                    downloading={isDownloadingId(comment.id)}
                    progress={downloadProgressOf(comment.id)}
                  />
                  <IconButton
                    size="small"
                    aria-label="Download attachment"
                    aria-busy={isDownloadingId(comment.id) || undefined}
                    onClick={() => {
                      void handleAttachmentDownload();
                    }}
                    disabled={isDownloadingId(comment.id)}
                  >
                    {isDownloadingId(comment.id) ? (
                      <DownloadProgressIndicator
                        progress={downloadProgressOf(comment.id)}
                      />
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
            {isPreviewableImage && !previewRequested && (
              <Box>
                <Button
                  size="small"
                  variant="outlined"
                  startIcon={<Image size={14} aria-hidden />}
                  onClick={() => setPreviewRequested(true)}
                >
                  Preview
                </Button>
              </Box>
            )}
            {isPreviewableImage &&
              previewRequested &&
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
                    // Same thumbnail size as inline images; a click opens it full size.
                    maxWidth: "min(100%, 360px)",
                    maxHeight: 240,
                    width: "auto",
                    height: "auto",
                    objectFit: "contain",
                    borderRadius: 1,
                    display: "block",
                    cursor: "zoom-in",
                  }}
                  role="button"
                  tabIndex={0}
                  aria-label={`Open ${comment.fileName ?? "image attachment"} full size`}
                  onClick={() => onImageClick?.(imageDataUrl)}
                  onKeyDown={(e: KeyboardEvent<HTMLElement>) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onImageClick?.(imageDataUrl);
                    }
                  }}
                />
              ) : isImagePreviewError ? (
                <Box>
                  <Typography variant="caption" color="text.secondary">
                    The preview could not be loaded.{" "}
                  </Typography>
                  <Button size="small" onClick={() => void refetchImagePreview()}>
                    Try again
                  </Button>
                </Box>
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
