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
  Box,
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
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
} from "react";
import { usePatchDeploymentAttachment } from "@features/project-details/api/usePatchDeploymentAttachment";
import type { EditDeploymentAttachmentModalProps } from "@features/project-details/types/projectDetailsComponents";

/**
 * Modal for editing a deployment document (name and description).
 * Only sends changed fields to the API (PATCH behavior).
 *
 * @param {EditDeploymentAttachmentModalProps} props - open, document, deploymentId, onClose, optional onSuccess/onError.
 * @returns {JSX.Element} The edit deployment attachment modal.
 */
export default function EditDeploymentAttachmentModal({
  open,
  document,
  deploymentId,
  onClose,
  onSuccess,
  onError,
}: EditDeploymentAttachmentModalProps): JSX.Element {
  const patchAttachment = usePatchDeploymentAttachment();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const prevDocIdRef = useRef<string | null>(null);

  const isSubmitting = patchAttachment.isPending;
  const isValid = !!deploymentId && !!document?.id;

  useEffect(() => {
    if (!open) {
      setName("");
      setDescription("");
      prevDocIdRef.current = null;
    } else if (document && document.id !== prevDocIdRef.current) {
      prevDocIdRef.current = document.id;
      setName(document.name ?? "");
      setDescription(document.description ?? "");
    }
  }, [open, document]);

  const handleClose = useCallback(() => {
    setName("");
    setDescription("");
    onClose();
  }, [onClose]);

  const handleNameChange = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => setName(e.target.value),
    [],
  );

  const handleDescriptionChange = useCallback(
    (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      setDescription(e.target.value),
    [],
  );

  const handleSubmit = useCallback(async () => {
    if (!isValid || !document) return;

    const body: { name?: string; description?: string } = {};
    const newName = name.trim();
    const originalName = (document.name ?? "").trim();
    if (newName !== originalName) {
      body.name = newName || undefined;
    }

    const newDescription = description.trim();
    const originalDescription = (document.description ?? "").trim();
    if (newDescription !== originalDescription) {
      body.description = newDescription || undefined;
    }

    if (Object.keys(body).length === 0) {
      handleClose();
      return;
    }

    try {
      await patchAttachment.mutateAsync({
        deploymentId,
        attachmentId: document.id,
        body,
      });
      handleClose();
      onSuccess?.();
    } catch (error) {
      onError?.(
        error instanceof Error ? error.message : "Failed to update document",
      );
    }
  }, [
    isValid,
    document,
    deploymentId,
    name,
    description,
    patchAttachment,
    handleClose,
    onSuccess,
    onError,
  ]);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="edit-deployment-attachment-dialog-title"
    >
      <DialogTitle
        id="edit-deployment-attachment-dialog-title"
        sx={{
          pr: 6,
          position: "relative",
          textTransform: "capitalize",
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
        }}
      >
        Edit Document
        <IconButton
          aria-label="Close"
          size="small"
          onClick={handleClose}
          disabled={isSubmitting}
          sx={{ position: "absolute", right: 12, top: 12 }}
        >
          <X size={20} aria-hidden />
        </IconButton>
      </DialogTitle>
      <DialogContent sx={{ pt: 2, pb: 1 }}>
        <Box sx={{ mt: 2, mb: 2 }}>
          <TextField
            id="edit-document-name"
            label="Document Name"
            placeholder="e.g., deployment-guide.pdf"
            value={name}
            onChange={handleNameChange}
            fullWidth
            size="small"
            disabled={isSubmitting}
          />
        </Box>
        <Box sx={{ mt: 2, mb: 2 }}>
          <TextField
            id="edit-document-description"
            label="Description"
            placeholder="Add a short description..."
            value={description}
            onChange={handleDescriptionChange}
            fullWidth
            size="small"
            multiline
            rows={3}
            disabled={isSubmitting}
          />
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2.5, pt: 1.5 }}>
        <Button onClick={handleClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="primary"
          onClick={handleSubmit}
          disabled={isSubmitting}
          startIcon={
            isSubmitting ? (
              <CircularProgress color="inherit" size={16} />
            ) : undefined
          }
        >
          {isSubmitting ? "Saving..." : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
