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
import { MAX_ATTACHMENT_SIZE_BYTES } from "@features/csm-cases/api/useCsmCaseAttachments";
import {
  readFileAsBase64,
  totalEncodedBytes,
  type EncodedAttachment,
} from "@components/attachments/encodeAttachment";

interface AttachmentsFieldProps {
  attachments: EncodedAttachment[];
  onChange: (next: EncodedAttachment[]) => void;
  /** Surface read/size errors (e.g. via the error banner). */
  onError: (message: string, err?: unknown) => void;
  /** Combined-encoded-size ceiling; an over-limit list shows an inline error. */
  maxEncodedBytes: number;
  /** Adjusts the hint text only ("At least one required" vs "Optional"). */
  required?: boolean;
}

/**
 * A multi-file attachment picker that encodes each file to raw base64 for the
 * case-create payload. Enforces a per-file size cap and surfaces a combined-size
 * error; the parent gates submission on `totalEncodedBytes` vs `maxEncodedBytes`.
 */
export default function AttachmentsField({
  attachments,
  onChange,
  onError,
  maxEncodedBytes,
  required = false,
}: AttachmentsFieldProps): JSX.Element {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const encodedBytes = totalEncodedBytes(attachments);
  const overLimit = encodedBytes > maxEncodedBytes;

  const onAddFiles = async (e: ChangeEvent<HTMLInputElement>): Promise<void> => {
    // Snapshot the FileList into a stable array BEFORE clearing the input —
    // `e.target.files` is live, so resetting `value` first would empty it.
    const list = Array.from(e.target.files ?? []);
    // Reset so picking the same file again re-fires change. The File objects in
    // `list` remain readable after this (only the input's FileList is cleared).
    if (fileInputRef.current) fileInputRef.current.value = "";
    if (list.length === 0) return;

    const tooBig = list.find((f) => f.size > MAX_ATTACHMENT_SIZE_BYTES);
    if (tooBig) {
      onError(
        `"${tooBig.name}" is too large. The maximum attachment size is ${formatBytes(
          MAX_ATTACHMENT_SIZE_BYTES,
        )}.`,
      );
      return;
    }
    try {
      const encoded = await Promise.all(
        list.map(async (f) => ({
          name: f.name,
          file: await readFileAsBase64(f),
          size: f.size,
          raw: f,
        })),
      );
      onChange([...attachments, ...encoded]);
    } catch (err) {
      onError("Could not read one of the selected files. Please try again.", err);
    }
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
        >
          Add attachments
          <input
            ref={fileInputRef}
            type="file"
            multiple
            hidden
            onChange={(e) => void onAddFiles(e)}
          />
        </Button>
        <Typography variant="caption" color="text.secondary">
          {required ? "At least one required" : "Optional"} · up to{" "}
          {formatBytes(MAX_ATTACHMENT_SIZE_BYTES)} each
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
          <IconButton size="small" aria-label={`Remove ${a.name}`} onClick={() => removeAt(i)}>
            <X size={16} />
          </IconButton>
        </Box>
      ))}

      {overLimit && (
        <Typography variant="caption" color="error">
          The attachments are too large together ({formatBytes(encodedBytes)} encoded).
          Maximum is {formatBytes(maxEncodedBytes)} — remove a file or attach smaller ones.
        </Typography>
      )}
    </Box>
  );
}
