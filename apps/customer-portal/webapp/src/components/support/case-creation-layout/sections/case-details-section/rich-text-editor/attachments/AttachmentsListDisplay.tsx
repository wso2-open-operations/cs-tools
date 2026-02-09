// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, IconButton, Paper, Typography } from "@wso2/oxygen-ui";
import { FileText, Image as ImageIcon, X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

export interface AttachmentItem {
  id: string;
  name: string;
  type: "image" | "file";
  dataUrl?: string;
}

export interface AttachmentsListProps {
  attachments: AttachmentItem[];
  onRemove: (id: string) => void;
}

/**
 * Displays a list of attachments.
 *
 * @param {AttachmentsListProps} props - Component props.
 * @returns {JSX.Element} The rendered attachments list.
 */
export function AttachmentsListDisplay({
  attachments,
  onRemove,
}: AttachmentsListProps): JSX.Element {
  if (attachments.length === 0) return <></>;

  return (
    <Box
      sx={{
        display: "flex",
        flexWrap: "wrap",
        gap: 1,
        p: 1,
        borderTop: "1px solid",
        borderColor: "divider",
        bgcolor: "background.default",
      }}
    >
      {attachments.map((att) => (
        <Paper
          key={att.id}
          variant="outlined"
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            pl: 1,
            pr: 0.5,
            py: 0.5,
            bgcolor: "action.hover",
          }}
        >
          {att.type === "image" && att.dataUrl ? (
            <Box
              component="img"
              src={att.dataUrl}
              alt={att.name}
              sx={{
                width: 28,
                height: 28,
                objectFit: "cover",
              }}
            />
          ) : att.type === "image" ? (
            <ImageIcon size={18} />
          ) : (
            <FileText size={18} />
          )}

          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Typography
              variant="caption"
              sx={{
                display: "block",
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
                maxWidth: 140,
              }}
            >
              {att.name}
            </Typography>
          </Box>
          <IconButton
            className="remove-btn"
            size="small"
            onClick={() => onRemove(att.id)}
            aria-label={`Remove ${att.name}`}
            color="warning"
          >
            <X size={14} />
          </IconButton>
        </Paper>
      ))}
    </Box>
  );
}
