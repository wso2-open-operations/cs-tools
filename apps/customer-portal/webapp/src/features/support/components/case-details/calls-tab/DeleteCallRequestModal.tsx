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
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useState,
  type ChangeEvent,
  type JSX,
} from "react";
import type { CallRequest } from "@features/support/types/calls";
import { formatUtcToLocal } from "@features/support/utils/support";

export interface DeleteCallRequestModalProps {
  open: boolean;
  call: CallRequest | null;
  userTimeZone?: string;
  onClose: () => void;
  onConfirm: (reason: string) => void;
  isDeleting?: boolean;
}

/**
 * Confirmation modal before deleting (cancelling) a call request.
 * Delete is implemented as PATCH with CALL_REQUEST_STATE_CANCELLED.
 * User must enter a mandatory reason before confirming.
 *
 * @param {DeleteCallRequestModalProps} props - open, call, onClose, onConfirm, isDeleting.
 * @returns {JSX.Element} The confirmation modal.
 */
export default function DeleteCallRequestModal({
  open,
  call,
  userTimeZone,
  onClose,
  onConfirm,
  isDeleting = false,
}: DeleteCallRequestModalProps): JSX.Element {
  const [reason, setReason] = useState("");

  const handleDialogClose = useCallback(
    (_event: object, _reason: string) => {
      if (isDeleting) return;
      onClose();
      setReason("");
    },
    [isDeleting, onClose],
  );

  const handleClose = useCallback(() => {
    onClose();
    setReason("");
  }, [onClose]);

  const handleReasonChange = useCallback(
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setReason(event.target.value);
    },
    [],
  );

  const handleConfirm = useCallback(() => {
    if (reason.trim() === "" || isDeleting) return;
    onConfirm(reason.trim());
  }, [reason, isDeleting, onConfirm]);

  useEffect(() => {
    if (!open) setReason("");
  }, [open]);

  const canConfirm = reason.trim() !== "";

  const firstPreferredTime = call?.preferredTimes
    ?.find((t) => t?.trim())
    ?.trim();
  const promptWhen =
    userTimeZone && firstPreferredTime
      ? formatUtcToLocal(firstPreferredTime, "short", false, userTimeZone)
      : "--";
  const cancelDescription =
    call == null
      ? "Are you sure you want to cancel this call request? This action cannot be undone."
      : promptWhen !== "--"
        ? `Are you sure you want to cancel the call request scheduled for ${promptWhen}? This action cannot be undone.`
        : "Are you sure you want to cancel this call request? This action cannot be undone.";

  return (
    <Dialog
      open={open}
      onClose={handleDialogClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="delete-call-request-modal-title"
      aria-describedby="delete-call-request-modal-description"
      slotProps={{
        paper: {
          sx: { position: "relative" },
        },
      }}
    >
      <IconButton
        aria-label="Close"
        size="small"
        onClick={handleClose}
        disabled={isDeleting}
        sx={{
          position: "absolute",
          right: 8,
          top: 8,
          zIndex: 1,
        }}
      >
        <X size={18} />
      </IconButton>
      <DialogTitle id="delete-call-request-modal-title">
        Confirm Action
      </DialogTitle>
      <DialogContent>
        <Typography
          id="delete-call-request-modal-description"
          color="text.secondary"
        >
          {cancelDescription}
        </Typography>
        <TextField
          id="cancel-call-reason"
          label="Reason"
          placeholder="Enter reason for cancellation..."
          value={reason}
          onChange={handleReasonChange}
          fullWidth
          size="small"
          multiline
          rows={3}
          sx={{ mt: 2 }}
          disabled={isDeleting}
          required
        />
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={handleClose} disabled={isDeleting}>
          Go Back
        </Button>
        <Button
          variant="contained"
          color="warning"
          onClick={handleConfirm}
          disabled={isDeleting || !canConfirm}
          startIcon={
            isDeleting ? (
              <CircularProgress size={16} color="inherit" />
            ) : undefined
          }
        >
          {isDeleting ? "Cancelling..." : "Confirm"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
