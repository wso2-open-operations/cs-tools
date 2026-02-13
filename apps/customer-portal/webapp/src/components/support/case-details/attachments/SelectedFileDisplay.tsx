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

import { Box, Typography } from "@wso2/oxygen-ui";
import { File, FileArchive } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { getAttachmentFileCategory } from "@utils/support";

export interface SelectedFileDisplayProps {
  fileName: string;
  fileType: string;
}

/**
 * Displays selected file name with icon (FileArchive for zip/archive, File for others).
 *
 * @param {SelectedFileDisplayProps} props - File name and type.
 * @returns {JSX.Element} The file display row.
 */
export default function SelectedFileDisplay({
  fileName,
  fileType,
}: SelectedFileDisplayProps): JSX.Element {
  const category = getAttachmentFileCategory(fileName, fileType);
  const icon = category === "archive" ? (
    <FileArchive size={24} aria-hidden />
  ) : (
    <File size={24} aria-hidden />
  );

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 2,
        p: 2,
        mb: 2,
        bgcolor: "action.hover",
        border: 1,
        borderColor: "divider",
      }}
    >
      <Box
        sx={{
          width: 40,
          height: 40,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          color: "text.secondary",
          flexShrink: 0,
        }}
        aria-hidden
      >
        {icon}
      </Box>
      <Typography variant="body2" color="text.primary" noWrap sx={{ flex: 1 }}>
        {fileName}
      </Typography>
    </Box>
  );
}
