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
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useRef, useState, type JSX } from "react";
import { usePostAttachments } from "@api/usePostAttachments";
import { useMockConfig } from "@providers/MockConfigProvider";
import { MAX_ATTACHMENT_SIZE_BYTES } from "@constants/supportConstants";
import UploadAttachmentDropZone from "@case-details-attachments/UploadAttachmentDropZone";
import SelectedFileDisplay from "@case-details-attachments/SelectedFileDisplay";

export { MAX_ATTACHMENT_SIZE_BYTES } from "@constants/supportConstants";

export interface UploadAttachmentModalProps {
  open: boolean;
  caseId: string;
  onClose: () => void;
  onSuccess?: () => void;
}

/**
 * Modal for uploading a case attachment: drag-and-drop or file picker, optional name (defaults to file name).
 * Upload is disabled when isMockEnabled. Max file size 15 MB; shows ErrorBanner when exceeded.
 *
 * @param {UploadAttachmentModalProps} props - open, caseId, onClose, optional onSuccess.
 * @returns {JSX.Element} The upload attachment modal.
 */
export default function UploadAttachmentModal({
  open,
  caseId,
  onClose,
  onSuccess,
}: UploadAttachmentModalProps): JSX.Element {
  const { isMockEnabled } = useMockConfig();
  const postAttachments = usePostAttachments();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState("");
  const [dragOver, setDragOver] = useState(false);
  const [fileSizeErrorVisible, setFileSizeErrorVisible] = useState(false);

  const fileTooLarge = file ? file.size > MAX_ATTACHMENT_SIZE_BYTES : false;
  const displayName = name.trim() || (file?.name ?? "");
  const canUpload =
    !isMockEnabled &&
    !!file &&
    !fileTooLarge &&
    !!displayName &&
    !postAttachments.isPending;

  const reset = useCallback(() => {
    setFile(null);
    setName("");
    setFileSizeErrorVisible(false);
  }, []);

  const handleClose = useCallback(() => {
    reset();
    onClose();
  }, [onClose, reset]);

  const setFileWithValidation = useCallback(
    (selected: File | null) => {
      setFile(selected);
      if (selected && selected.size > MAX_ATTACHMENT_SIZE_BYTES) {
        setFileSizeErrorVisible(true);
      } else {
        setFileSizeErrorVisible(false);
      }
      if (selected && !name.trim()) {
        setName(selected.name);
      }
    },
    [name],
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      const item = e.dataTransfer.files[0];
      if (item) setFileWithValidation(item);
    },
    [setFileWithValidation],
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const handleDragLeave = useCallback(() => {
    setDragOver(false);
  }, []);

  const handleFileSelect = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const selected = e.target.files?.[0] ?? null;
      setFileWithValidation(selected);
      e.target.value = "";
    },
    [setFileWithValidation],
  );

  const handleUpload = useCallback(() => {
    if (!file || fileTooLarge || isMockEnabled) return;
    const attachmentName = (name.trim() || file.name).trim();
    if (!attachmentName) return;
    const reader = new FileReader();
    reader.onload = () => {
      const base64 = typeof reader.result === "string" ? reader.result : "";
      const commaIndex = base64.indexOf(",");
      const content = commaIndex >= 0 ? base64.slice(commaIndex + 1) : base64;
      postAttachments.mutate(
        {
          caseId,
          body: {
            referenceType: "case",
            name: attachmentName,
            type: file.type || "application/octet-stream",
            content,
          },
        },
        {
          onSuccess: () => {
            handleClose();
            onSuccess?.();
          },
        },
      );
    };
    reader.readAsDataURL(file);
  }, [
    caseId,
    file,
    name,
    fileTooLarge,
    isMockEnabled,
    postAttachments,
    handleClose,
    onSuccess,
  ]);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="upload-attachment-dialog-title"
    >
      <DialogTitle
        id="upload-attachment-dialog-title"
        sx={{ pr: 6, position: "relative" }}
      >
        Upload Attachment
        <IconButton
          aria-label="close"
          onClick={handleClose}
          sx={{ position: "absolute", right: 12, top: 12 }}
          size="small"
        >
          <X size={20} aria-hidden />
        </IconButton>
      </DialogTitle>
      <DialogContent sx={{ pt: 1 }}>
        {fileSizeErrorVisible && (
          <Alert
            severity="error"
            onClose={() => setFileSizeErrorVisible(false)}
            sx={{ mb: 2 }}
          >
            File size exceeds 15 MB limit.
          </Alert>
        )}
        <input
          ref={fileInputRef}
          type="file"
          accept="*/*"
          onChange={handleFileChange}
          style={{ display: "none" }}
          aria-hidden
        />

        {!file ? (
          <UploadAttachmentDropZone
            dragOver={dragOver}
            onDrop={handleDrop}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onChooseFile={handleFileSelect}
          />
        ) : (
          <SelectedFileDisplay fileName={file.name} fileType={file.type} />
        )}

        <TextField
          fullWidth
          label="Attachment name"
          placeholder="Leave empty to use file name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={!file}
        />
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2, pt: 1 }}>
        <Button onClick={handleClose} disabled={postAttachments.isPending}>
          Cancel
        </Button>
        {postAttachments.isPending ? (
          <Button
            variant="outlined"
            startIcon={<CircularProgress color="inherit" size={16} />}
            disabled
          >
            Uploading
          </Button>
        ) : (
          <Button
            type="button"
            variant="contained"
            color="primary"
            onClick={handleUpload}
            disabled={!canUpload}
          >
            Upload
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
