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
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  LinearProgress,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useRef, useState, type JSX } from "react";
import { formatLimit } from "@config/attachmentLimitsConfig";
import useAttachmentLimits from "@api/useAttachmentLimits";
import { useStoredAttachmentUpload } from "@api/useStoredAttachments";
import {
  isAbortError,
  UploadConfirmPendingError,
  type StoredUploadStep,
} from "@api/storedAttachments";
import UploadAttachmentDropZone from "@case-details-attachments/UploadAttachmentDropZone";
import SelectedFileDisplay from "@case-details-attachments/SelectedFileDisplay";

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
 * Supports caseId (case attachments) or deploymentId (deployment documents). The file is sent to
 * the portal backend in chunks (start, chunks, confirm); the size limit is the configured
 * attachment limit (10 MB by default).
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
  const { upload: storedUpload, confirm: storedConfirm } =
    useStoredAttachmentUpload();
  const [uploadStep, setUploadStep] = useState<StoredUploadStep | null>(null);
  // Bytes sent so far, while the chunks upload.
  const [progress, setProgress] = useState<{ sent: number; total: number } | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const [storedError, setStoredError] = useState<string | null>(null);
  // Set when the chunks arrived but the final confirm failed: retry only that step.
  const [pendingConfirm, setPendingConfirm] = useState<{
    attachmentId: string;
    chunks: number;
  } | null>(null);
  const isDeploymentMode = !!deploymentId && !caseId;
  const { maxFileBytes: maxBytes } = useAttachmentLimits();
  const maxLabel = formatLimit(maxBytes);
  const isPending = storedUpload.isPending || storedConfirm.isPending;
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [file, setFile] = useState<File | null>(null);
  const [name, setName] = useState("");
  const [dragOver, setDragOver] = useState(false);
  const [fileSizeErrorDismissed, setFileSizeErrorDismissed] = useState(false);

  const fileTooLarge = file ? file.size > maxBytes : false;
  const fileSizeErrorVisible = fileTooLarge && !fileSizeErrorDismissed;
  const displayName = name.trim() || (file ? getBaseFileName(file.name) : "");
  const canUpload =
    !!file &&
    !fileTooLarge &&
    !!displayName &&
    (!!onSelect || (!isPending && (!!caseId || !!deploymentId)));

  const reset = useCallback(() => {
    setFile(null);
    setName("");
    setFileSizeErrorDismissed(false);
    setUploadStep(null);
    setProgress(null);
    setStoredError(null);
    setPendingConfirm(null);
  }, []);

  const handleClose = useCallback(() => {
    // Cancels an upload still running; the pending row is left to the
    // server's cleanup. No effect on a finished upload.
    abortRef.current?.abort();
    abortRef.current = null;
    reset();
    onClose();
  }, [onClose, reset]);

  const setFileWithValidation = useCallback(
    (selected: File | null) => {
      setFile(selected);
      setFileSizeErrorDismissed(false);
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

    setStoredError(null);
    setProgress(null);
    const controller = new AbortController();
    abortRef.current = controller;
    storedUpload.mutate(
      {
        type: isDeploymentMode ? "deployment" : "work_item",
        parentId: (isDeploymentMode ? deploymentId : caseId)!,
        file,
        body: { name: attachmentName },
        ...(isDeploymentMode ? { deploymentId } : { caseId }),
        onStep: setUploadStep,
        signal: controller.signal,
        onProgress: (sent, total) => setProgress({ sent, total }),
      },
      {
        onSuccess: () => {
          handleClose();
          onSuccess?.();
        },
        onError: (error) => {
          if (controller.signal.aborted || isAbortError(error)) return;
          abortRef.current = null;
          setUploadStep(null);
          setProgress(null);
          if (error instanceof UploadConfirmPendingError) {
            setPendingConfirm({
              attachmentId: error.attachmentId,
              chunks: error.chunks,
            });
          }
          setStoredError(error.message);
        },
      },
    );
  }, [
    caseId,
    deploymentId,
    file,
    name,
    fileTooLarge,
    isDeploymentMode,
    storedUpload,
    handleClose,
    onSuccess,
    onSelect,
  ]);

  // Retries only the final step; the chunks are already stored.
  const handleRetryConfirm = useCallback(() => {
    if (!pendingConfirm) return;
    setStoredError(null);
    storedConfirm.mutate(
      {
        type: isDeploymentMode ? "deployment" : "work_item",
        attachmentId: pendingConfirm.attachmentId,
        chunks: pendingConfirm.chunks,
        ...(isDeploymentMode ? { deploymentId } : { caseId }),
      },
      {
        onSuccess: () => {
          handleClose();
          onSuccess?.();
        },
        onError: (error) => setStoredError(error.message),
      },
    );
  }, [
    pendingConfirm,
    isDeploymentMode,
    deploymentId,
    caseId,
    storedConfirm,
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
        {fileSizeErrorVisible && (
          <Alert
            severity="error"
            onClose={() => setFileSizeErrorDismissed(true)}
            sx={{ mb: 2 }}
          >
            File size exceeds {maxLabel} limit.
          </Alert>
        )}
        {storedError && (
          <Alert
            severity="error"
            onClose={() => setStoredError(null)}
            sx={{ mb: 2 }}
          >
            {storedError}
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

        {progress && storedUpload.isPending && (
          <Box sx={{ mt: 2 }}>
            <LinearProgress
              variant="determinate"
              value={
                progress.total > 0
                  ? Math.min(100, (progress.sent / progress.total) * 100)
                  : 0
              }
              aria-label="Upload progress"
            />
            <Typography variant="caption" color="text.secondary">
              {progress.total > 0
                ? `${Math.min(100, Math.floor((progress.sent / progress.total) * 100))}%`
                : ""}
            </Typography>
          </Box>
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
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2, pt: 1 }}>
        <Button
          onClick={handleClose}
          disabled={
            isPending &&
            !(
              storedUpload.isPending &&
              (uploadStep === "preparing" ||
                (uploadStep === "uploading" && progress !== null))
            )
          }
        >
          Cancel
        </Button>
        {isPending ? (
          <Button
            variant="outlined"
            startIcon={<CircularProgress color="inherit" size={16} />}
            disabled
          >
            {uploadStep === "preparing"
              ? "Preparing"
              : uploadStep === "finishing" || storedConfirm.isPending
                ? "Finishing"
                : "Uploading"}
          </Button>
        ) : pendingConfirm ? (
          <Button
            type="button"
            variant="contained"
            color="primary"
            onClick={handleRetryConfirm}
          >
            Retry
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
