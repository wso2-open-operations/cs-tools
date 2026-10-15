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

import { Box, Button, CircularProgress, IconButton, Typography } from "@wso2/oxygen-ui";
import { Download, Paperclip, Upload, X } from "@wso2/oxygen-ui-icons-react";
import { useRef, type ChangeEvent, type JSX, type ReactNode } from "react";
import { formatBytes } from "@utils/formatBytes";
import type { AnnouncementAttachmentItem } from "@features/csm-announcements/api/useAnnouncementAttachments";
import {
  ANNOUNCEMENT_ATTACHMENT_ACCEPT,
  ANNOUNCEMENT_ATTACHMENT_TYPES_LABEL,
  MAX_ANNOUNCEMENT_ATTACHMENTS,
  MAX_ANNOUNCEMENT_ATTACHMENT_BYTES,
} from "@features/csm-announcements/utils/announcementAttachments";

interface AnnouncementAttachmentsFieldProps {
  items: AnnouncementAttachmentItem[];
  /** Not needed by a read-only list. */
  onAddFiles?: (files: File[]) => void;
  /** Not needed by a read-only list. */
  onRemove?: (item: AnnouncementAttachmentItem) => void;
  /** Offered for saved files when given (a read of the bytes needs a permission the caller may lack: the backend refuses and the caller reports it). */
  onDownload?: (item: AnnouncementAttachmentItem) => void;
  /** Lists the files without offering to add or remove any (a request whose files cannot change now). */
  readOnly?: boolean;
  /** Disables adding and removing while the parent form is busy. */
  disabled?: boolean;
  /** The saved list is loading: shown instead of an empty list so a request's files are never reported as none. */
  isLoading?: boolean;
  /** The saved list could not be loaded. */
  loadFailed?: boolean;
  /** Extra line under the hint, e.g. what changing the files does to a request that is waiting for approval. */
  note?: ReactNode;
}

const STATUS_LABEL: Record<AnnouncementAttachmentItem["status"], string | null> = {
  saved: null,
  staged: "Saved with the draft",
  uploading: "Uploading…",
};

/**
 * The files sent with an announcement (a PDF above all). Every file is attached to
 * each announcement case, so customers see it among that case's attachments, as on
 * any other case. Purely presentational: `useAnnouncementAttachments` decides what
 * the rows are and what the actions do, so the create forms and the request dialog
 * share one behaviour.
 */
export default function AnnouncementAttachmentsField({
  items,
  onAddFiles,
  onRemove,
  onDownload,
  readOnly = false,
  disabled = false,
  isLoading = false,
  loadFailed = false,
  note,
}: AnnouncementAttachmentsFieldProps): JSX.Element {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const atLimit = items.length >= MAX_ANNOUNCEMENT_ATTACHMENTS;

  const handlePick = (e: ChangeEvent<HTMLInputElement>): void => {
    // Snapshot the live FileList before clearing the input, which empties it.
    const picked = Array.from(e.target.files ?? []);
    if (fileInputRef.current) fileInputRef.current.value = "";
    if (picked.length > 0) onAddFiles?.(picked);
  };

  return (
    <Box role="group" aria-labelledby="announcement-attachments-label" sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Typography id="announcement-attachments-label" variant="caption" color="text.secondary">
        Attachments
      </Typography>

      {!readOnly && (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Button
            component="label"
            variant="outlined"
            size="small"
            startIcon={<Upload size={16} />}
            disabled={disabled || atLimit}
          >
            Add files
            <input
              ref={fileInputRef}
              type="file"
              multiple
              hidden
              accept={ANNOUNCEMENT_ATTACHMENT_ACCEPT}
              disabled={disabled || atLimit}
              onChange={handlePick}
            />
          </Button>
          <Typography variant="caption" color="text.secondary">
            Optional · {ANNOUNCEMENT_ATTACHMENT_TYPES_LABEL} · up to {MAX_ANNOUNCEMENT_ATTACHMENTS} files,{" "}
            {formatBytes(MAX_ANNOUNCEMENT_ATTACHMENT_BYTES)} each · customers see them on every announcement case
          </Typography>
        </Box>
      )}

      {note && (
        <Typography variant="caption" color="text.secondary">
          {note}
        </Typography>
      )}

      {isLoading && (
        <Box role="status" aria-live="polite" sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <CircularProgress size={14} />
          <Typography variant="caption" color="text.secondary">
            Loading attachments…
          </Typography>
        </Box>
      )}
      {loadFailed && (
        <Typography variant="caption" color="error">
          Couldn&apos;t load this request&apos;s attachments.
        </Typography>
      )}
      {readOnly && !isLoading && !loadFailed && items.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          No attachments.
        </Typography>
      )}

      {items.map((item) => (
        <Box
          key={item.key}
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
            px: 1.5,
            py: 0.75,
          }}
        >
          <Paperclip size={16} aria-hidden style={{ opacity: 0.7 }} />
          <Typography variant="body2" sx={{ flexGrow: 1, minWidth: 0 }} noWrap title={item.name}>
            {item.name}
          </Typography>
          {STATUS_LABEL[item.status] && (
            <Typography variant="caption" color="text.secondary">
              {STATUS_LABEL[item.status]}
            </Typography>
          )}
          <Typography variant="caption" color="text.secondary">
            {formatBytes(item.sizeBytes)}
          </Typography>
          {item.status === "saved" && onDownload && (
            <IconButton size="small" aria-label={`Download ${item.name}`} onClick={() => onDownload(item)}>
              <Download size={16} />
            </IconButton>
          )}
          {!readOnly && (
            <IconButton
              size="small"
              aria-label={`Remove ${item.name}`}
              onClick={() => onRemove?.(item)}
              disabled={disabled || item.status === "uploading" || item.removing}
            >
              <X size={16} />
            </IconButton>
          )}
        </Box>
      ))}
    </Box>
  );
}
