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
import { useCallback, useState, type ChangeEvent, type JSX } from "react";
import type { CallRequest } from "@features/support/types/calls";
import { formatCallRequestPromptScheduledTime } from "@features/support/utils/support";

export interface RejectCallRequestModalProps {
  open: boolean;
  call: CallRequest | null;
  onClose: () => void;
  onConfirm: (reason: string) => void;
  isRejecting?: boolean;
}

/**
 * Confirmation modal for rejecting a "Pending on Customer" call request.
 * Implemented as PATCH with the Customer Rejected state key.
 * User must enter a mandatory reason before confirming.
 *
 * @param {RejectCallRequestModalProps} props - open, call, onClose, onConfirm, isRejecting.
 * @returns {JSX.Element} The reject call request modal.
 */
export default function RejectCallRequestModal({
  open,
  call,
  onClose,
  onConfirm,
  isRejecting = false,
}: RejectCallRequestModalProps): JSX.Element {
  const [reason, setReason] = useState("");

  const resetAndClose = useCallback(() => {
    setReason("");
    onClose();
  }, [onClose]);

  const handleDialogClose = useCallback(() => {
    if (isRejecting) return;
    resetAndClose();
  }, [isRejecting, resetAndClose]);

  const handleClose = useCallback(() => {
    if (isRejecting) return;
    resetAndClose();
  }, [isRejecting, resetAndClose]);

  const handleReasonChange = useCallback(
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setReason(event.target.value);
    },
    [],
  );

  const handleConfirm = useCallback(() => {
    if (reason.trim() === "" || isRejecting) return;
    onConfirm(reason.trim());
  }, [reason, isRejecting, onConfirm]);

  const canConfirm = reason.trim() !== "";

  const promptWhen =
    call != null
      ? formatCallRequestPromptScheduledTime(
          call.preferredTimes,
          call.scheduleTime,
        )
      : "--";
  const rejectDescription =
    call == null
      ? "Are you sure you want to reject this call request?"
      : promptWhen !== "--"
        ? `Are you sure you want to reject the call request scheduled for ${promptWhen}?`
        : "Are you sure you want to reject this call request?";

  return (
    <Dialog
      open={open}
      onClose={handleDialogClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="reject-call-request-modal-title"
      aria-describedby="reject-call-request-modal-description"
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
        disabled={isRejecting}
        sx={{
          position: "absolute",
          right: 8,
          top: 8,
          zIndex: 1,
        }}
      >
        <X size={18} />
      </IconButton>
      <DialogTitle id="reject-call-request-modal-title">
        Reject Call Request
      </DialogTitle>
      <DialogContent>
        <Typography
          id="reject-call-request-modal-description"
          color="text.secondary"
        >
          {rejectDescription}
        </Typography>
        <TextField
          id="reject-call-reason"
          label="Reason *"
          placeholder="Enter reason for rejection..."
          value={reason}
          onChange={handleReasonChange}
          fullWidth
          size="small"
          multiline
          rows={3}
          sx={{ mt: 2 }}
          disabled={isRejecting}
          required
        />
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={handleClose} disabled={isRejecting}>
          Go Back
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={handleConfirm}
          disabled={isRejecting || !canConfirm}
          startIcon={
            isRejecting ? (
              <CircularProgress size={16} color="inherit" />
            ) : undefined
          }
        >
          {isRejecting ? "Rejecting..." : "Reject"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
