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

import { Box, Button, Paper, Stack, Typography } from "@wso2/oxygen-ui";
import {
  Download,
  File,
  FileArchive,
  FileText,
  Image,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { CaseAttachment } from "@models/responses";
import { formatFileSize, getAttachmentFileCategory } from "@utils/support";

export interface AttachmentListItemProps {
  attachment: CaseAttachment;
  onDownload: (attachment: CaseAttachment) => void;
}

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
 * @param {AttachmentListItemProps} props - Attachment and download handler.
 * @returns {JSX.Element} The attachment list item.
 */
export default function AttachmentListItem({
  attachment,
  onDownload,
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
      <Button
        variant="outlined"
        size="small"
        startIcon={<Download size={16} aria-hidden />}
        onClick={() => onDownload(attachment)}
      >
        Download
      </Button>
    </Paper>
  );
}
