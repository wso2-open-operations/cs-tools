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


import { Box, CircularProgress, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * The spinner shown on a download button while a file downloads: a ring that
 * fills with the percentage once the file's size is known, and the number
 * beside it. Before that it is an ordinary spinner.
 */
export default function DownloadProgressIndicator({
  progress,
}: {
  /** Percentage (0-100), or null while it is not known. */
  progress: number | null;
}): JSX.Element {
  const known = progress !== null;
  return (
    <Box
      sx={{ display: "inline-flex", alignItems: "center", gap: 0.5 }}
      role="progressbar"
      aria-label="Downloading"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={known ? progress : undefined}
    >
      <Box sx={{ position: "relative", display: "inline-flex", width: 16, height: 16 }}>
        {known && (
          <CircularProgress
            variant="determinate"
            value={100}
            size={16}
            aria-hidden
            sx={{ position: "absolute", inset: 0, color: "action.disabledBackground" }}
          />
        )}
        <CircularProgress
          color="warning"
          size={16}
          variant={known ? "determinate" : "indeterminate"}
          value={known ? progress : undefined}
          aria-hidden
        />
      </Box>
      {known && (
        <Typography variant="caption" sx={{ minWidth: "3ch", lineHeight: 1, color: "warning.main" }}>
          {progress}%
        </Typography>
      )}
    </Box>
  );
}
