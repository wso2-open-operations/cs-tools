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

import { Box, Button, IconButton, Typography } from "@wso2/oxygen-ui";
import { Paperclip, Upload, X } from "@wso2/oxygen-ui-icons-react";
import { useRef, type ChangeEvent, type JSX } from "react";
import { formatBytes } from "@utils/formatBytes";
import {
  attachmentLimitMessage,
  entityAttachmentMaxBytes,
  formatSizeLimit,
} from "@config/attachmentLimitsConfig";
import type { PendingAttachment } from "@components/attachments/pendingAttachment";

interface AttachmentsFieldProps {
  attachments: PendingAttachment[];
  onChange: (next: PendingAttachment[]) => void;
  /** Surface size errors (e.g. via the error banner). */
  onError: (message: string, err?: unknown) => void;
  /** Adjusts the hint text only ("At least one required" vs "Optional"). */
  required?: boolean;
  /** Disables adding/removing files, e.g. while the parent form is submitting. */
  disabled?: boolean;
  /** The most files the field takes. Unlimited when omitted. */
  maxFiles?: number;
}

/**
 * A multi-file attachment picker for create forms. It only holds the chosen
 * files and enforces the per-file size cap; the parent uploads them once the
 * record exists.
 */
export default function AttachmentsField({
  attachments,
  onChange,
  onError,
  required = false,
  disabled = false,
  maxFiles,
}: AttachmentsFieldProps): JSX.Element {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const maxBytes = entityAttachmentMaxBytes();
  const full = maxFiles !== undefined && attachments.length >= maxFiles;

  const onAddFiles = (e: ChangeEvent<HTMLInputElement>): void => {
    // Snapshot the FileList into a stable array BEFORE clearing the input —
    // `e.target.files` is live, so resetting `value` first would empty it.
    const list = Array.from(e.target.files ?? []);
    // Reset so picking the same file again re-fires change. The File objects in
    // `list` remain readable after this (only the input's FileList is cleared).
    if (fileInputRef.current) fileInputRef.current.value = "";
    if (list.length === 0) return;

    // Take only as many as there is room for; say so when some are left out.
    let picked = list;
    if (maxFiles !== undefined) {
      const room = Math.max(0, maxFiles - attachments.length);
      if (list.length > room) {
        onError(attachmentLimitMessage(maxFiles));
        picked = list.slice(0, room);
      }
      if (picked.length === 0) return;
    }

    const tooBig = picked.find((f) => f.size > maxBytes);
    if (tooBig) {
      onError(
        `"${tooBig.name}" is too large. The maximum attachment size is ${formatSizeLimit(maxBytes)}.`,
      );
      return;
    }
    onChange([...attachments, ...picked.map((f) => ({ name: f.name, size: f.size, raw: f }))]);
  };

  const removeAt = (index: number): void => {
    onChange(attachments.filter((_, i) => i !== index));
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Button
          component="label"
          variant="outlined"
          size="small"
          startIcon={<Upload size={16} />}
          disabled={disabled || full}
        >
          {maxFiles === 1 ? "Add attachment" : "Add attachments"}
          <input
            ref={fileInputRef}
            type="file"
            multiple={maxFiles !== 1}
            hidden
            disabled={disabled || full}
            onChange={onAddFiles}
          />
        </Button>
        <Typography variant="caption" color="text.secondary">
          {required ? (maxFiles === 1 ? "Required" : "At least one required") : "Optional"} ·{" "}
          {maxFiles === 1
            ? `1 attachment, up to ${formatSizeLimit(maxBytes)}`
            : `up to ${formatSizeLimit(maxBytes)} each`}
        </Typography>
      </Box>

      {attachments.map((a, i) => (
        <Box
          key={`${a.name}-${i}`}
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
          <Typography variant="body2" sx={{ flexGrow: 1, minWidth: 0 }} noWrap>
            {a.name}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {formatBytes(a.size)}
          </Typography>
          <IconButton
            size="small"
            aria-label={`Remove ${a.name}`}
            onClick={() => removeAt(i)}
            disabled={disabled}
          >
            <X size={16} />
          </IconButton>
        </Box>
      ))}
    </Box>
  );
}
