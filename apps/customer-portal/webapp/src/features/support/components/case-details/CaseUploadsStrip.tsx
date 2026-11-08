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


import { Box, LinearProgress, Paper, Stack, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useCaseUploads } from "@api/caseUploadProgress";

/**
 * A strip on the case page for each file still uploading to the case, with its
 * percentage: files that outlasted the create-case page's wait, or any upload
 * the person started. It shows nothing when no upload runs.
 */
export default function CaseUploadsStrip({ caseId }: { caseId: string }): JSX.Element | null {
  const uploads = useCaseUploads(caseId);
  if (uploads.length === 0) return null;
  return (
    <Stack spacing={1} sx={{ px: 2, pt: 2 }} aria-live="polite">
      {uploads.map((upload) => (
        <Paper key={upload.key} variant="outlined" sx={{ p: 1.5, minWidth: 0 }}>
          <Box sx={{ display: "flex", justifyContent: "space-between", gap: 2, mb: 0.75 }}>
            <Typography
              variant="body2"
              sx={{ fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
            >
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
        </Paper>
      ))}
    </Stack>
  );
}
