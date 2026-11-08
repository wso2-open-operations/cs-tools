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

import { Box, Divider, LinearProgress, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/** A file still uploading and how far it has got (0-100). */
export interface EditorUpload {
  name: string;
  percent: number;
}

/**
 * The files a comment sent from the editor is still uploading, shown inside the
 * editor under the text so the progress stays next to the box it came from.
 * Shows nothing when no upload runs.
 */
export default function EditorUploadProgress({
  uploads,
  waiting = 0,
}: {
  uploads: readonly EditorUpload[];
  /** Files queued behind the running ones. */
  waiting?: number;
}): JSX.Element | null {
  if (uploads.length === 0) return null;
  return (
    <>
      <Divider sx={{ my: 1.5 }} />
      <Box aria-live="polite" sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
        {uploads.map((upload, index) => (
          <Box key={`${upload.name}-${index}`}>
            <Box sx={{ display: "flex", justifyContent: "space-between", gap: 2, mb: 0.5 }}>
              <Typography variant="caption" noWrap sx={{ fontWeight: 500, minWidth: 0 }}>
                Uploading {upload.name}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {upload.percent}%
              </Typography>
            </Box>
            <LinearProgress
              variant="determinate"
              value={upload.percent}
              aria-label={`Uploading ${upload.name}`}
            />
          </Box>
        ))}
        {waiting > 0 && (
          <Typography variant="caption" color="text.secondary">
            {waiting} more {waiting === 1 ? "file" : "files"} waiting to upload
          </Typography>
        )}
      </Box>
    </>
  );
}
