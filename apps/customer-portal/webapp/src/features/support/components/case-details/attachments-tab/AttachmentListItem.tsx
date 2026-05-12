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

import type { AttachmentListItemProps } from "@features/support/types/supportComponents";
import {
  Box,
  CircularProgress,
  IconButton,
  Paper,
  Skeleton,
  Stack,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ChevronDown,
  ChevronUp,
  Download,
  File,
  FileArchive,
  FileText,
  Image,
  PencilLine,
  Trash2,
} from "@wso2/oxygen-ui-icons-react";
import { useEffect, useRef, useState, type JSX } from "react";
import type { CaseAttachment } from "@features/support/types/cases";
import {
  formatDateTime,
  formatFileSize,
  getAttachmentFileCategory,
} from "@features/support/utils/support";
import ImageFullscreenModal from "@case-details-activity/ImageFullscreenModal";
import { useAttachmentPreview } from "@api/useAttachmentPreview";

const PREVIEW_SKELETON_MIN_DISPLAY_MS = 4000;

// TODO: Use attachment category enum when introduced (see support.ts AttachmentFileCategory).
function getAttachmentIcon(att: CaseAttachment): JSX.Element {
  const category = getAttachmentFileCategory(att.name ?? "", att.type ?? "");
  switch (category) {
    case "image":
      return <Image size={20} aria-hidden />;
    case "pdf":
      return <FileText size={20} aria-hidden />;
    case "text":
      return <FileText size={20} aria-hidden />;
    case "archive":
      return <FileArchive size={20} aria-hidden />;
    default:
      return <File size={20} aria-hidden />;
  }
}

/**
 * Single attachment row: header (icon, name, actions), optional image preview (`previewUrl` only),
 * then meta line. Clicking the preview opens the same fullscreen dialog as the activity tab.
 *
 * @param {AttachmentListItemProps} props - Attachment, handlers, delete disabled/tooltip, download loading.
 * @returns {JSX.Element} The attachment list item.
 */
export default function AttachmentListItem({
  attachment,
  onDownload,
  onDelete,
  onEdit,
  deleteDisabled = false,
  deleteTooltip,
  hideDescription = false,
  isDownloadLoading = false,
}: AttachmentListItemProps): JSX.Element {
  const [fullscreenOpen, setFullscreenOpen] = useState(false);
  const [imageExpanded, setImageExpanded] = useState(false);
  const [previewMinTimeElapsed, setPreviewMinTimeElapsed] = useState(false);
  const previewTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (imageExpanded) {
      setPreviewMinTimeElapsed(false);
      previewTimerRef.current = setTimeout(
        () => setPreviewMinTimeElapsed(true),
        PREVIEW_SKELETON_MIN_DISPLAY_MS,
      );
    } else {
      if (previewTimerRef.current) {
        clearTimeout(previewTimerRef.current);
        previewTimerRef.current = null;
      }
      setPreviewMinTimeElapsed(false);
    }
    return () => {
      if (previewTimerRef.current) clearTimeout(previewTimerRef.current);
    };
  }, [imageExpanded]);
  const attachmentCategory = getAttachmentFileCategory(
    attachment.name ?? "",
    attachment.type ?? "",
  );
  const isImageAttachment = attachmentCategory === "image";
  const hasPreviewImage = isImageAttachment;

  // Fetch authenticated image data URL only when the preview is expanded.
  const { data: imageDataUrl, isLoading: isImageLoading } = useAttachmentPreview(
    imageExpanded && isImageAttachment ? attachment.id : null,
  );

  const deleteIconButton = onDelete ? (
    <IconButton
      size="small"
      aria-label={`Delete ${attachment.name}`}
      sx={{ color: "text.secondary" }}
      disabled={deleteDisabled}
      onClick={() => onDelete(attachment)}
    >
      <Trash2 size={16} aria-hidden />
    </IconButton>
  ) : null;

  const deleteButton =
    deleteIconButton && deleteDisabled && deleteTooltip ? (
      <Tooltip title={deleteTooltip} arrow>
        <span>{deleteIconButton}</span>
      </Tooltip>
    ) : (
      deleteIconButton
    );

  return (
    <>
      <Paper
        variant="outlined"
        sx={{
          p: 2,
          display: "flex",
          flexDirection: "column",
          alignItems: "stretch",
          gap: 1.5,
          "&:hover": {
            bgcolor: "action.hover",
          },
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 2,
            minWidth: 0,
          }}
        >
          <Box
            sx={{
              width: 40,
              height: 40,
              bgcolor: "action.hover",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "text.secondary",
              flexShrink: 0,
            }}
            aria-hidden
          >
            {getAttachmentIcon(attachment)}
          </Box>
          <Typography
            variant="body2"
            color="text.primary"
            sx={{
              flex: 1,
              minWidth: 0,
              fontWeight: 500,
              overflow: "hidden",
              textOverflow: "ellipsis",
            }}
          >
            {attachment.name}
          </Typography>
          <Stack direction="row" spacing={0.25} alignItems="center" sx={{ flexShrink: 0 }}>
            {hasPreviewImage && (
              <IconButton
                size="small"
                aria-label={imageExpanded ? "Collapse image" : "Expand image"}
                sx={{ color: "text.secondary" }}
                onClick={() => setImageExpanded((prev) => !prev)}
              >
                {imageExpanded ? (
                  <ChevronUp size={16} aria-hidden />
                ) : (
                  <ChevronDown size={16} aria-hidden />
                )}
              </IconButton>
            )}
            {onEdit && (
              <IconButton
                size="small"
                aria-label={`Edit ${attachment.name}`}
                sx={{ color: "text.secondary" }}
                onClick={() => onEdit(attachment)}
              >
                <PencilLine size={16} aria-hidden />
              </IconButton>
            )}
            {deleteButton}
            <IconButton
              size="small"
              aria-label={`Download ${attachment.name}`}
              aria-busy={isDownloadLoading || undefined}
              sx={{ color: "text.secondary" }}
              disabled={isDownloadLoading}
              onClick={() => onDownload(attachment)}
            >
              {isDownloadLoading ? (
                <CircularProgress color="inherit" size={16} aria-hidden />
              ) : (
                <Download size={16} aria-hidden />
              )}
            </IconButton>
          </Stack>
        </Box>

        {hasPreviewImage && imageExpanded && (
          <Box sx={{ position: "relative", width: "100%" }}>
            {!previewMinTimeElapsed || isImageLoading ? (
              <Skeleton
                variant="rectangular"
                width="100%"
                height={160}
                sx={{ borderRadius: 1 }}
              />
            ) : imageDataUrl ? (
              <Box
                component="button"
                type="button"
                onClick={() => setFullscreenOpen(true)}
                aria-label={`View ${attachment.name} full size`}
                sx={{
                  m: 0,
                  p: 0,
                  border: "none",
                  background: "none",
                  cursor: "pointer",
                  width: "100%",
                  display: "block",
                  textAlign: "left",
                }}
              >
                <Box
                  component="img"
                  src={imageDataUrl}
                  alt={attachment.name}
                  sx={{
                    display: "block",
                    maxHeight: 400,
                    maxWidth: "100%",
                    width: "100%",
                    objectFit: "contain",
                  }}
                />
              </Box>
            ) : null}
          </Box>
        )}

        {!hideDescription && attachment.description && (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{
              display: "-webkit-box",
              WebkitLineClamp: 2,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
            }}
          >
            {attachment.description}
          </Typography>
        )}

        <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
          <Typography variant="caption" color="text.secondary" component="span">
            {formatFileSize(attachment.size ?? attachment.sizeBytes)}
          </Typography>
          <Typography variant="caption" color="text.secondary" component="span">
            •
          </Typography>
          <Typography variant="caption" color="text.secondary" component="span">
            Uploaded by {attachment.createdBy}
          </Typography>
          <Typography variant="caption" color="text.secondary" component="span">
            •
          </Typography>
          <Typography variant="caption" color="text.secondary" component="span">
            {formatDateTime(attachment.createdOn)}
          </Typography>
        </Stack>
      </Paper>

      <ImageFullscreenModal
        open={fullscreenOpen}
        imageSrc={imageDataUrl ?? null}
        onClose={() => setFullscreenOpen(false)}
      />
    </>
  );
}
