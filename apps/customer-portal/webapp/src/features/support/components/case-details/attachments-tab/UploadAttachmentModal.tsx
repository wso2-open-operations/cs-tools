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

import type { UploadAttachmentModalProps } from "@features/support/types/supportComponents";
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
import { usePostAttachments } from "@features/support/api/usePostAttachments";
import { usePostDeploymentAttachment } from "@features/project-details/api/usePostDeploymentAttachment";
import {
  MAX_ATTACHMENT_SIZE_BYTES,
  ALLOWED_ATTACHMENT_ACCEPT,
  isAllowedAttachment,
} from "@features/support/constants/supportConstants";
import UploadAttachmentDropZone from "@case-details-attachments/UploadAttachmentDropZone";
import SelectedFileDisplay from "@case-details-attachments/SelectedFileDisplay";

export { MAX_ATTACHMENT_SIZE_BYTES } from "@features/support/constants/supportConstants";

function getFileExtension(fileName: string): string {
  const dotIndex = fileName.lastIndexOf(".");
  if (dotIndex < 0 || dotIndex === fileName.length - 1) {
    return "";
  }
  return fileName.slice(dotIndex);
}

function getBaseFileName(fileName: string): string {
  const dotIndex = fileName.lastIndexOf(".");
  if (dotIndex < 0 || dotIndex === fileName.length - 1) {
    return fileName;
  }
  return fileName.slice(0, dotIndex);
}

function ensureExtension(fileName: string, extension: string): string {
  if (!extension) return fileName;
  if (fileName.toLowerCase().endsWith(extension.toLowerCase())) {
    return fileName;
  }
  return `${fileName}${extension}`;
}

/**
 * Modal for uploading a case attachment or deployment document: drag-and-drop or file picker.
 * Supports caseId (case attachments) or deploymentId (deployment documents). Max file size 10 MB.
 *
 * @param {UploadAttachmentModalProps} props - open, caseId/deploymentId, onClose, optional onSuccess.
 * @returns {JSX.Element} The upload attachment modal.
 */
export default function UploadAttachmentModal({
  open,
  caseId,
  deploymentId,
  onClose,
  onSuccess,
  onSelect,
}: UploadAttachmentModalProps): JSX.Element {
  const postAttachments = usePostAttachments();
  const postDeploymentAttachment = usePostDeploymentAttachment();
  const isDeploymentMode = !!deploymentId && !caseId;
  const showDescription = isDeploymentMode;
  const isPending = isDeploymentMode
    ? postDeploymentAttachment.isPending
    : postAttachments.isPending;
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [dragOver, setDragOver] = useState(false);
  const [fileSizeErrorVisible, setFileSizeErrorVisible] = useState(false);
  const [fileTypeErrorVisible, setFileTypeErrorVisible] = useState(false);
  const [readErrorVisible, setReadErrorVisible] = useState(false);

  const fileTooLarge = file ? file.size > MAX_ATTACHMENT_SIZE_BYTES : false;
  const displayName = name.trim() || (file ? getBaseFileName(file.name) : "");
  const canUpload =
    !!file &&
    !fileTooLarge &&
    !!displayName &&
    (!!onSelect || (!isPending && (!!caseId || !!deploymentId)));

  const reset = useCallback(() => {
    setFile(null);
    setName("");
    setDescription("");
    setFileSizeErrorVisible(false);
    setFileTypeErrorVisible(false);
    setReadErrorVisible(false);
  }, []);

  const handleClose = useCallback(() => {
    reset();
    onClose();
  }, [onClose, reset]);

  const setFileWithValidation = useCallback(
    (selected: File | null) => {
      if (selected && !isAllowedAttachment(selected)) {
        setFileTypeErrorVisible(true);
        setFileSizeErrorVisible(false);
        setFile(null);
        return;
      }
      setFileTypeErrorVisible(false);
      setFile(selected);
      if (selected && selected.size > MAX_ATTACHMENT_SIZE_BYTES) {
        setFileSizeErrorVisible(true);
      } else {
        setFileSizeErrorVisible(false);
      }
      if (selected && !name.trim()) {
        setName(getBaseFileName(selected.name));
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
    if (!file || fileTooLarge) return;
    const normalizedName = (name.trim() || getBaseFileName(file.name)).trim();
    const attachmentName = ensureExtension(
      normalizedName,
      getFileExtension(file.name),
    );
    if (!attachmentName) return;

    if (onSelect) {
      onSelect(file, attachmentName);
      handleClose();
      return;
    }

    if (!caseId && !deploymentId) return;

    const reader = new FileReader();
    reader.onload = () => {
      const base64 = typeof reader.result === "string" ? reader.result : "";
      const commaIndex = base64.indexOf(",");
      const content = commaIndex >= 0 ? base64.slice(commaIndex + 1) : base64;
      const body = {
        name: attachmentName,
        type: file.type || "application/octet-stream",
        content,
        ...(description.trim() && { description: description.trim() }),
      };

      if (isDeploymentMode) {
        postDeploymentAttachment.mutate(
          { deploymentId: deploymentId!, body },
          {
            onSuccess: () => {
              handleClose();
              onSuccess?.();
            },
          },
        );
      } else if (caseId) {
        postAttachments.mutate(
          { caseId, body },
          {
            onSuccess: () => {
              handleClose();
              onSuccess?.();
            },
          },
        );
      }
    };
    reader.onerror = () => {
      setReadErrorVisible(true);
    };
    reader.readAsDataURL(file);
  }, [
    caseId,
    deploymentId,
    file,
    name,
    description,
    fileTooLarge,
    isDeploymentMode,
    postAttachments,
    postDeploymentAttachment,
    handleClose,
    onSuccess,
    onSelect,
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
        {isDeploymentMode ? "Add Document" : "Upload Attachment"}
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
        {fileTypeErrorVisible && (
          <Alert
            severity="error"
            onClose={() => setFileTypeErrorVisible(false)}
            sx={{ mb: 2 }}
          >
            This file type is not supported. Supported types: pdf, doc, docx,
            xls, xlsx, ppt, pptx, zip, json, xml, txt, csv, jpg, jpeg, png,
            webp, sh, har.
          </Alert>
        )}
        {fileSizeErrorVisible && (
          <Alert
            severity="error"
            onClose={() => setFileSizeErrorVisible(false)}
            sx={{ mb: 2 }}
          >
            File size exceeds 10 MB limit.
          </Alert>
        )}
        {readErrorVisible && (
          <Alert
            severity="error"
            onClose={() => setReadErrorVisible(false)}
            sx={{ mb: 2 }}
          >
            Failed to read file. Please try again.
          </Alert>
        )}
        <input
          ref={fileInputRef}
          type="file"
          accept={ALLOWED_ATTACHMENT_ACCEPT}
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
          label={isDeploymentMode ? "Document name" : "Attachment name"}
          placeholder="Leave empty to use file name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={!file}
          sx={{ mt: 2 }}
        />
        {showDescription && (
          <TextField
            fullWidth
            label="Description (optional)"
            placeholder="Add a short description for this file"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={!file}
            multiline
            minRows={2}
            sx={{ mt: 2 }}
          />
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2, pt: 1 }}>
        <Button onClick={handleClose} disabled={isPending}>
          Cancel
        </Button>
        {isPending ? (
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
            {onSelect ? "Add" : "Upload"}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
