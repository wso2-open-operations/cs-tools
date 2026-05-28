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
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Typography,
} from "@wso2/oxygen-ui";
import { FileArchive, Upload, X } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useRef, useState, type JSX } from "react";
import { usePostDeploymentUsagesImport } from "@features/usage-metrics/api/usePostDeploymentUsagesImport";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";

interface DeploymentUsageUploadDialogProps {
  open: boolean;
  onClose: () => void;
}

function isZipFile(file: File): boolean {
  return (
    file.name.toLowerCase().endsWith(".zip") ||
    file.type === "application/zip" ||
    file.type === "application/x-zip-compressed"
  );
}

/**
 * Dialog for uploading a deployment usage ZIP file to POST /deployment-usages.
 * Matches the attachment upload modal UI with drag-and-drop and file picker.
 *
 * @param {DeploymentUsageUploadDialogProps} props - open state and close handler.
 * @returns {JSX.Element} Upload dialog.
 */
export default function DeploymentUsageUploadDialog({
  open,
  onClose,
}: DeploymentUsageUploadDialogProps): JSX.Element {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const [fileError, setFileError] = useState<string>("");

  const { mutate, isPending, error, reset } = usePostDeploymentUsagesImport();
  const { showSuccess } = useSuccessBanner();

  const handleClose = useCallback(() => {
    setFile(null);
    setDragOver(false);
    setFileError("");
    reset();
    onClose();
  }, [onClose, reset]);

  const applyFile = useCallback((selected: File) => {
    setFileError("");
    reset();
    if (!isZipFile(selected)) {
      setFileError("Only ZIP files are accepted.");
      setFile(null);
      return;
    }
    setFile(selected);
  }, [reset]);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      const item = e.dataTransfer.files[0];
      if (item) applyFile(item);
    },
    [applyFile],
  );

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const handleDragLeave = useCallback(() => {
    setDragOver(false);
  }, []);

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const selected = e.target.files?.[0];
      if (selected) applyFile(selected);
      e.target.value = "";
    },
    [applyFile],
  );

  const handleUpload = useCallback(() => {
    if (!file) return;
    mutate(file, {
      onSuccess: () => {
        handleClose();
        showSuccess("Deployment usage data imported successfully.");
      },
    });
  }, [file, mutate, handleClose, showSuccess]);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="deployment-usage-upload-dialog-title"
    >
      <DialogTitle
        id="deployment-usage-upload-dialog-title"
        sx={{ pr: 6, position: "relative" }}
      >
        Import Deployment Usage
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
        {fileError && (
          <Alert
            severity="error"
            onClose={() => setFileError("")}
            sx={{ mb: 2 }}
          >
            {fileError}
          </Alert>
        )}
        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error.message}
          </Alert>
        )}

        <input
          ref={fileInputRef}
          type="file"
          accept=".zip,application/zip,application/x-zip-compressed"
          onChange={handleFileChange}
          style={{ display: "none" }}
          aria-hidden
        />

        {!file ? (
          <Box
            onDrop={handleDrop}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            sx={{
              border: 2,
              borderColor: dragOver ? "primary.main" : "divider",
              borderStyle: "dashed",
              minHeight: 180,
              width: "100%",
              p: 3,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              textAlign: "center",
              bgcolor: dragOver ? "action.hover" : "background.default",
              mb: 2,
            }}
          >
            <Upload
              size={48}
              aria-hidden
              style={{ color: "var(--oxygen-palette-text-secondary)" }}
            />
            <Typography variant="body2" color="text.secondary" sx={{ mt: 1.5 }}>
              Drag and drop a file here, or
            </Typography>
            <Button
              variant="outlined"
              size="small"
              onClick={() => fileInputRef.current?.click()}
              sx={{ mt: 1.5 }}
            >
              Choose file
            </Button>
            <Typography variant="caption" color="text.secondary" sx={{ mt: 2 }}>
              Only ZIP files are accepted.
            </Typography>
          </Box>
        ) : (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              p: 2,
              mb: 2,
              bgcolor: "action.hover",
              border: 1,
              borderColor: "divider",
            }}
          >
            <Box
              sx={{
                width: 40,
                height: 40,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                color: "text.secondary",
                flexShrink: 0,
              }}
              aria-hidden
            >
              <FileArchive size={24} aria-hidden />
            </Box>
            <Typography variant="body2" color="text.primary" noWrap sx={{ flex: 1 }}>
              {file.name}
            </Typography>
          </Box>
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
            variant="contained"
            color="primary"
            onClick={handleUpload}
            disabled={!file}
          >
            Upload
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
