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

import { Box, Button, CircularProgress, IconButton, LinearProgress, Tooltip, Typography } from "@wso2/oxygen-ui";
import { Download, X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { cancelAttachmentDownload, useAttachmentDownloadProgress } from "@features/csm-cases/api/attachmentDownloadProgress";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";

/** The Cancel button of a running download: it stops it, saves nothing and shows no error. */
export function AttachmentDownloadCancelButton({ id, name }: { id: string; name: string }): JSX.Element {
  return (
    <Tooltip title="Cancel download">
      <IconButton
        size="small"
        aria-label={`Cancel download of ${name}`}
        onClick={() => cancelAttachmentDownload(id)}
      >
        <X size={14} aria-hidden />
      </IconButton>
    </Tooltip>
  );
}

/**
 * An icon-only Download button (for rows with little room, like a deployment's
 * files): while the file downloads it shows the ring and percentage, with a
 * Cancel button beside it.
 */
export function AttachmentDownloadIconButton({
  id,
  name,
  onDownload,
}: {
  id: string;
  name: string;
  onDownload: () => void;
}): JSX.Element {
  const progress = useAttachmentDownloadProgress(id);
  const downloading = progress !== undefined;
  const known = progress !== undefined && progress >= 0;
  return (
    <>
      <Tooltip title={downloading ? "" : "Download"}>
        <span>
          <IconButton
            size="small"
            aria-label={`Download ${name}`}
            aria-busy={downloading || undefined}
            disabled={downloading}
            onClick={onDownload}
          >
            {downloading ? (
              <Box sx={{ display: "inline-flex", alignItems: "center", gap: 0.5 }}>
                <CircularProgress
                  size={14}
                  variant={known ? "determinate" : "indeterminate"}
                  value={known ? progress : undefined}
                  aria-hidden
                />
                {known && (
                  <Typography variant="caption" sx={{ lineHeight: 1 }}>
                    {progress}%
                  </Typography>
                )}
              </Box>
            ) : (
              <Download size={14} />
            )}
          </IconButton>
        </span>
      </Tooltip>
      {downloading && <AttachmentDownloadCancelButton id={id} name={name} />}
    </>
  );
}

/** The bar along the bottom of an attachment row while it downloads (the row is `position: relative`). */
export function AttachmentDownloadBar({ id }: { id: string }): JSX.Element | null {
  const progress = useAttachmentDownloadProgress(id);
  if (progress === undefined) return null;
  const known = progress >= 0;
  return (
    <LinearProgress
      aria-hidden
      variant={known ? "determinate" : "indeterminate"}
      value={known ? progress : undefined}
      sx={{ position: "absolute", left: 0, right: 0, bottom: 0, height: 4 }}
    />
  );
}

/** The Download button of an attachment row; while it downloads it shows the percentage. */
export function AttachmentDownloadButton({
  attachment,
  onDownload,
}: {
  attachment: CaseAttachment;
  onDownload?: (attachment: CaseAttachment) => void;
}): JSX.Element {
  const progress = useAttachmentDownloadProgress(attachment.id);
  const downloading = progress !== undefined;
  const known = progress !== undefined && progress >= 0;
  // Announced from beside the button: a disabled button is skipped by screen
  // readers, so a status inside it would never be read. Every 10 percent only.
  const announced = known ? Math.floor(progress / 10) * 10 : null;
  return (
    <>
      <Box
        component="span"
        role="status"
        aria-live="polite"
        aria-atomic="true"
        sx={{
          position: "absolute",
          width: 1,
          height: 1,
          overflow: "hidden",
          clip: "rect(0 0 0 0)",
          whiteSpace: "nowrap",
        }}
      >
        {downloading
          ? announced === null
            ? `Downloading ${attachment.filename}`
            : `Downloading ${attachment.filename}: ${announced}%`
          : ""}
      </Box>
      <Button
        size="small"
        variant="outlined"
        startIcon={
          downloading ? (
            <CircularProgress
              size={14}
              color="inherit"
              variant={known ? "determinate" : "indeterminate"}
              value={known ? progress : undefined}
              aria-hidden
            />
          ) : (
            <Download size={14} />
          )
        }
        onClick={() => onDownload?.(attachment)}
        disabled={!onDownload || downloading}
        aria-busy={downloading || undefined}
        aria-label={`Download ${attachment.filename}`}
      >
        {known ? `${progress}%` : downloading ? "Starting" : "Download"}
      </Button>
      {downloading && <AttachmentDownloadCancelButton id={attachment.id} name={attachment.filename} />}
    </>
  );
}
