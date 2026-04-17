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
import type { JSX } from "react";
import type { CaseAttachment } from "@features/support/types/cases";
import { formatFileSize, getAttachmentFileCategory } from "@features/support/utils/support";

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
 * Single attachment row with icon, name, meta, and download button.
 *
 * @param {AttachmentListItemProps} props - Attachment, handlers, optional download loading.
 * @returns {JSX.Element} The attachment list item.
 */
export default function AttachmentListItem({
  attachment,
  onDownload,
  onDelete,
  onEdit,
  hideDescription = false,
  isDownloadLoading = false,
}: AttachmentListItemProps): JSX.Element {
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 2,
        display: "flex",
        alignItems: "center",
        gap: 2,
        "&:hover": {
          bgcolor: "action.hover",
        },
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
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="body2" color="text.primary" noWrap>
          {attachment.name}
        </Typography>
        {!hideDescription && attachment.description && (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{
              mt: 0.25,
              display: "-webkit-box",
              WebkitLineClamp: 2,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
            }}
          >
            {attachment.description}
          </Typography>
        )}
        <Stack
          direction="row"
          spacing={1}
          alignItems="center"
          flexWrap="wrap"
          sx={{ mt: 0.5 }}
        >
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
      </Box>
      <Stack direction="row" spacing={0.25}>
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
        {onDelete && (
          <IconButton
            size="small"
            aria-label={`Delete ${attachment.name}`}
            sx={{ color: "text.secondary" }}
            onClick={() => onDelete(attachment)}
          >
            <Trash2 size={16} aria-hidden />
          </IconButton>
        )}
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
    </Paper>
  );
}
