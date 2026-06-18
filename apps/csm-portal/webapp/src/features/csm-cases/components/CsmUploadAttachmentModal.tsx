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

import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Paperclip, Upload, X } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useRef,
  useState,
  type ChangeEvent,
  type DragEvent,
  type JSX,
} from "react";
import { formatBytes } from "@utils/formatBytes";
import { MAX_ATTACHMENT_SIZE_BYTES } from "@features/csm-cases/api/useCsmCaseAttachments";

interface CsmUploadAttachmentModalProps {
  open: boolean;
  onClose: () => void;
  /** Hand the chosen file + display name back to the composer. */
  onSelect: (file: File, name: string) => void;
}

function fileExtension(fileName: string): string {
  const dot = fileName.lastIndexOf(".");
  return dot < 0 || dot === fileName.length - 1 ? "" : fileName.slice(dot);
}

function baseFileName(fileName: string): string {
  const dot = fileName.lastIndexOf(".");
  return dot < 0 || dot === fileName.length - 1 ? fileName : fileName.slice(0, dot);
}

/** Keep the original extension on a renamed file so the type stays obvious. */
function withExtension(name: string, ext: string): string {
  if (!ext) return name;
  return name.toLowerCase().endsWith(ext.toLowerCase()) ? name : `${name}${ext}`;
}

/**
 * Pick (or drop) a single file and give it a display name before attaching it
 * to a comment. Mirrors the customer portal's UploadAttachmentModal `onSelect`
 * flow — the file isn't uploaded here; the composer uploads it on send.
 */
export default function CsmUploadAttachmentModal({
  open,
  onClose,
  onSelect,
}: CsmUploadAttachmentModalProps): JSX.Element {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState("");
  const [dragOver, setDragOver] = useState(false);

  const tooLarge = !!file && file.size > MAX_ATTACHMENT_SIZE_BYTES;
  const displayName = name.trim() || (file ? baseFileName(file.name) : "");
  const canAdd = !!file && !tooLarge && !!displayName;

  const reset = useCallback(() => {
    setFile(null);
    setName("");
    setDragOver(false);
  }, []);

  const handleClose = useCallback(() => {
    reset();
    onClose();
  }, [onClose, reset]);

  const pickFile = useCallback((selected: File | null) => {
    setFile(selected);
    if (selected) setName(baseFileName(selected.name));
  }, []);

  const onInputChange = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => {
      pickFile(e.target.files?.[0] ?? null);
      e.target.value = "";
    },
    [pickFile],
  );

  const onDrop = useCallback(
    (e: DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      const dropped = e.dataTransfer.files?.[0];
      if (dropped) pickFile(dropped);
    },
    [pickFile],
  );

  const handleAdd = useCallback(() => {
    if (!file || tooLarge) return;
    const finalName = withExtension(
      name.trim() || baseFileName(file.name),
      fileExtension(file.name),
    );
    if (!finalName) return;
    onSelect(file, finalName);
    handleClose();
  }, [file, name, tooLarge, onSelect, handleClose]);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="csm-upload-attachment-title"
    >
      <DialogTitle id="csm-upload-attachment-title" sx={{ pr: 6 }}>
        Attach a file
        <IconButton
          aria-label="Close"
          onClick={handleClose}
          size="small"
          sx={{ position: "absolute", right: 12, top: 12 }}
        >
          <X size={20} aria-hidden />
        </IconButton>
      </DialogTitle>
      <DialogContent sx={{ pt: 1 }}>
        {tooLarge && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {file?.name} is {formatBytes(file.size)}. The maximum attachment size
            is {formatBytes(MAX_ATTACHMENT_SIZE_BYTES)}.
          </Alert>
        )}
        <input
          ref={fileInputRef}
          type="file"
          hidden
          onChange={onInputChange}
          aria-hidden
        />
        {!file ? (
          <Box
            role="button"
            tabIndex={0}
            aria-label="Choose a file to attach"
            onClick={() => fileInputRef.current?.click()}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                fileInputRef.current?.click();
              }
            }}
            onDrop={onDrop}
            onDragOver={(e) => {
              e.preventDefault();
              setDragOver(true);
            }}
            onDragLeave={() => setDragOver(false)}
            sx={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              gap: 1,
              py: 4,
              px: 2,
              borderRadius: 1,
              border: "1px dashed",
              borderColor: dragOver ? "primary.main" : "divider",
              backgroundColor: dragOver ? "action.hover" : "transparent",
              cursor: "pointer",
              textAlign: "center",
            }}
          >
            <Upload size={24} />
            <Typography variant="body2">
              Drag a file here, or click to choose
            </Typography>
            <Typography variant="caption" color="text.secondary">
              Up to {formatBytes(MAX_ATTACHMENT_SIZE_BYTES)}
            </Typography>
          </Box>
        ) : (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              p: 1,
              borderRadius: 1,
              border: 1,
              borderColor: "divider",
            }}
          >
            <Paperclip size={16} />
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <Typography variant="body2" noWrap>
                {file.name}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {formatBytes(file.size)} · {file.type || "unknown type"}
              </Typography>
            </Box>
            <Button size="small" variant="text" onClick={() => pickFile(null)}>
              Change
            </Button>
          </Box>
        )}

        <TextField
          fullWidth
          label="Attachment name"
          placeholder="Leave empty to use the file name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={!file}
          sx={{ mt: 2 }}
        />
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2, pt: 1 }}>
        <Button onClick={handleClose}>Cancel</Button>
        <Button
          variant="contained"
          color="primary"
          onClick={handleAdd}
          disabled={!canAdd}
        >
          Add
        </Button>
      </DialogActions>
    </Dialog>
  );
}
