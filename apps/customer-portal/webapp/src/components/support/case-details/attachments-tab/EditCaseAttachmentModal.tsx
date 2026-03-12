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

import type { CaseAttachment } from "@models/responses";
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
import { usePatchCaseAttachment } from "@api/usePatchCaseAttachment";

export interface EditCaseAttachmentModalProps {
  open: boolean;
  attachment: CaseAttachment | null;
  caseId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
}

/**
 * Modal for editing a case attachment (name only).
 *
 * @param {EditCaseAttachmentModalProps} props - open, attachment, caseId, onClose, optional onSuccess/onError.
 * @returns {JSX.Element} The edit case attachment modal.
 */
export default function EditCaseAttachmentModal({
  open,
  attachment,
  caseId,
  onClose,
  onSuccess,
  onError,
}: EditCaseAttachmentModalProps): JSX.Element {
  const patchAttachment = usePatchCaseAttachment();
  const [name, setName] = useState("");
  const prevAttachmentIdRef = useRef<string | null>(null);

  const isSubmitting = patchAttachment.isPending;
  const isValid = !!caseId && !!attachment?.id;

  useEffect(() => {
    if (!open) {
      setName("");
      prevAttachmentIdRef.current = null;
    } else if (attachment && attachment.id !== prevAttachmentIdRef.current) {
      prevAttachmentIdRef.current = attachment.id;
      setName(attachment.name ?? "");
    }
  }, [open, attachment]);

  const handleClose = useCallback(() => {
    setName("");
    onClose();
  }, [onClose]);

  const handleNameChange = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => setName(e.target.value),
    [],
  );

  const handleSubmit = useCallback(async () => {
    if (!isValid || !attachment) return;

    const newName = name.trim();
    const originalName = (attachment.name ?? "").trim();
    if (newName === originalName) {
      handleClose();
      return;
    }

    try {
      await patchAttachment.mutateAsync({
        caseId,
        attachmentId: attachment.id,
        body: { name: newName || undefined },
      });
      handleClose();
      onSuccess?.();
    } catch (error) {
      onError?.(
        error instanceof Error ? error.message : "Failed to update attachment",
      );
    }
  }, [
    isValid,
    attachment,
    caseId,
    name,
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
      aria-labelledby="edit-case-attachment-dialog-title"
    >
      <DialogTitle
        id="edit-case-attachment-dialog-title"
        sx={{
          pr: 6,
          position: "relative",
          textTransform: "capitalize",
          overflow: "hidden",
          textOverflow: "ellipsis",
        }}
      >
        Edit Attachment
        <IconButton
          aria-label="Close"
          size="small"
          onClick={handleClose}
          disabled={isSubmitting}
          sx={{ position: "absolute", right: 12, top: 12 }}
        >
          <X size={23} aria-hidden />
        </IconButton>
      </DialogTitle>
      <DialogContent sx={{ pt: 2, pb: 1 }}>
        <Box sx={{ mt: 2, mb: 2 }}>
          <TextField
            id="edit-attachment-name"
            label="Attachment Name"
            placeholder="e.g., screenshot.png"
            value={name}
            onChange={handleNameChange}
            fullWidth
            size="small"
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
