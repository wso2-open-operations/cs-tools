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

import { useRef, useState } from "react";
import { Button, IconButton, Stack, Typography } from "@wso2/oxygen-ui";
import { FileText, Paperclip, Trash2 } from "@wso2/oxygen-ui-icons-react";
import {
  formatBytes,
  MAX_ATTACHMENT_SIZE_BYTES,
  toPendingAttachment,
  type PendingAttachment,
} from "@utils/attachments";

interface AttachmentsFieldProps {
  attachments: PendingAttachment[];
  onChange: (attachments: PendingAttachment[]) => void;
  disabled?: boolean;
}

// Mirrors the webapp's shared AttachmentsField
// (apps/csm-portal/webapp/src/components/attachments/AttachmentsField.tsx): a button that opens
// the native file picker, a list of added files with size + remove, oversized files rejected
// client-side with an inline error instead of hitting the backend's 413.
export function AttachmentsField({ attachments, onChange, disabled }: AttachmentsFieldProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | null>(null);

  const handleFiles = async (fileList: FileList | null) => {
    if (!fileList || fileList.length === 0) return;
    setError(null);

    const files = Array.from(fileList);
    const oversized = files.find((f) => f.size > MAX_ATTACHMENT_SIZE_BYTES);
    if (oversized) {
      setError(`"${oversized.name}" is larger than ${formatBytes(MAX_ATTACHMENT_SIZE_BYTES)}.`);
      return;
    }

    const encoded = await Promise.all(files.map(toPendingAttachment));
    onChange([...attachments, ...encoded]);
    if (inputRef.current) inputRef.current.value = "";
  };

  return (
    <Stack gap={1}>
      <input ref={inputRef} type="file" multiple hidden onChange={(event) => void handleFiles(event.target.files)} />

      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="body2" fontWeight={500}>
          Attachments (optional)
        </Typography>
        <Button
          size="small"
          variant="outlined"
          startIcon={<Paperclip size={14} />}
          disabled={disabled}
          onClick={() => inputRef.current?.click()}
        >
          Add files
        </Button>
      </Stack>

      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}

      {attachments.map((attachment) => (
        <Stack
          key={attachment.id}
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          gap={1}
          sx={{ p: 1, border: "1px solid", borderColor: "divider", borderRadius: 1 }}
        >
          <Stack direction="row" alignItems="center" gap={1} sx={{ minWidth: 0 }}>
            <FileText size={16} />
            <Typography variant="body2" noWrap>
              {attachment.name}
            </Typography>
            <Typography variant="caption" color="text.secondary" noWrap>
              {formatBytes(attachment.size)}
            </Typography>
          </Stack>
          <IconButton
            size="small"
            aria-label={`Remove ${attachment.name}`}
            disabled={disabled}
            onClick={() => onChange(attachments.filter((a) => a.id !== attachment.id))}
          >
            <Trash2 size={14} />
          </IconButton>
        </Stack>
      ))}
    </Stack>
  );
}
