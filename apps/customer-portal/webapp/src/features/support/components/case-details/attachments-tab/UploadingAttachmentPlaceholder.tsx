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

import { Box, CircularProgress, LinearProgress, Paper, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface UploadingAttachmentPlaceholderProps {
  name: string;
  /** Percentage sent (0-100), when known. */
  percent?: number;
}

/**
 * Placeholder row for an attachment that is still uploading in the background
 * (e.g. one created on the create-case page whose upload outlasted the wait window).
 *
 * @param {UploadingAttachmentPlaceholderProps} props - name of the attachment being uploaded and how far it has got.
 * @returns {JSX.Element} The placeholder row.
 */
export default function UploadingAttachmentPlaceholder({
  name,
  percent,
}: UploadingAttachmentPlaceholderProps): JSX.Element {
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 2,
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
        <CircularProgress color="inherit" size={20} />
      </Box>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography
          variant="body2"
          color="text.primary"
          sx={{
            fontWeight: 500,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {name}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {percent === undefined ? "Uploading…" : `Uploading… ${percent}%`}
        </Typography>
        {percent !== undefined && (
          <LinearProgress
            variant="determinate"
            value={percent}
            aria-label={`Uploading ${name}`}
            sx={{ mt: 0.5 }}
          />
        )}
      </Box>
    </Paper>
  );
}
