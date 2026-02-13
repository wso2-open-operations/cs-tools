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

import { Box, Button, Typography } from "@wso2/oxygen-ui";
import { Upload } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

export interface UploadAttachmentDropZoneProps {
  dragOver: boolean;
  onDrop: (e: React.DragEvent) => void;
  onDragOver: (e: React.DragEvent) => void;
  onDragLeave: () => void;
  onChooseFile: () => void;
}

/**
 * Drag-and-drop zone with "Choose file" button for attachment upload.
 *
 * @param {UploadAttachmentDropZoneProps} props - Handlers and drag state.
 * @returns {JSX.Element} The drop zone.
 */
export default function UploadAttachmentDropZone({
  dragOver,
  onDrop,
  onDragOver,
  onDragLeave,
  onChooseFile,
}: UploadAttachmentDropZoneProps): JSX.Element {
  return (
    <Box
      onDrop={onDrop}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      sx={{
        border: 2,
        borderColor: dragOver ? "primary.main" : "divider",
        borderStyle: "dashed",
        minHeight: 180,
        width: "100%",
        p: 3,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        textAlign: "center",
        bgcolor: dragOver ? "action.hover" : "background.default",
        mb: 2,
      }}
    >
      <Upload
        size={48}
        aria-hidden
        style={{ color: "var(--oxygen-palette-text-secondary)" }}
      />
      <Typography variant="body2" color="text.secondary" sx={{ mt: 1.5 }}>
        Drag and drop a file here, or
      </Typography>
      <Button
        variant="outlined"
        size="small"
        onClick={onChooseFile}
        sx={{ mt: 1.5 }}
      >
        Choose file
      </Button>
      <Typography variant="caption" color="text.secondary" sx={{ mt: 2 }}>
        Maximum file size: 15 MB.
      </Typography>
    </Box>
  );
}
