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
  Stack,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Download,
  File,
  FileArchive,
  FileText,
  Image,
  PencilLine,
  Trash2,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import type { CaseAttachment } from "@features/support/types/cases";
import {
  formatFileSize,
  getAttachmentFileCategory,
} from "@features/support/utils/support";
import ImageFullscreenModal from "@case-details-activity/ImageFullscreenModal";

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
  const [fullscreenSrc, setFullscreenSrc] = useState<string | null>(null);
  const attachmentCategory = getAttachmentFileCategory(
    attachment.name ?? "",
    attachment.type ?? "",
  );
  const previewUrl = attachment.previewUrl?.trim() ?? "";
  const hasPreviewImage =
    attachmentCategory === "image" && previewUrl.length > 0;

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
            alignItems: "flex-start",
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
          <Stack direction="row" spacing={0.25} sx={{ flexShrink: 0 }}>
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

        {hasPreviewImage && (
          <Box
            component="button"
            type="button"
            onClick={() => setFullscreenSrc(previewUrl)}
            sx={{
              m: 0,
              p: 0,
              border: "none",
              background: "none",
              cursor: "pointer",
              alignSelf: "flex-start",
              maxWidth: "100%",
              maxHeight: 200,
              overflow: "hidden",
              display: "block",
            }}
          >
            <Box
              component="img"
              src={previewUrl}
              alt={attachment.name}
              loading="lazy"
              sx={{
                maxWidth: "100%",
                maxHeight: 200,
                width: "auto",
                height: "auto",
                objectFit: "contain",
                display: "block",
              }}
            />
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
            {attachment.createdOn}
          </Typography>
        </Stack>
      </Paper>

      <ImageFullscreenModal
        open={!!fullscreenSrc}
        imageSrc={fullscreenSrc}
        onClose={() => setFullscreenSrc(null)}
      />
    </>
  );
}
