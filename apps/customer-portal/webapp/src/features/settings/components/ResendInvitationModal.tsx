// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License
// at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useCallback, type JSX } from "react";
import {
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { ResendInvitationModalProps } from "@features/settings/types/settings";
import { stripStaffNameMarker } from "@utils/staffNameMarker";

/**
 * Confirmation modal before re-sending a contact's invitation. Mirrors
 * RemoveUserModal, with the primary (not error) action colour since a resend
 * is not destructive.
 *
 * @param {ResendInvitationModalProps} props - open, contact, isResending, onClose, onConfirm.
 * @returns {JSX.Element} The confirmation modal.
 */
export default function ResendInvitationModal({
  open,
  contact,
  isResending = false,
  onClose,
  onConfirm,
}: ResendInvitationModalProps): JSX.Element {
  const handleDialogClose = useCallback(() => {
    if (!isResending) onClose();
  }, [onClose, isResending]);

  const name = contact
    ? stripStaffNameMarker(
        contact.firstName && contact.lastName
          ? `${contact.firstName} ${contact.lastName}`
          : contact.firstName || contact.lastName,
      )
    : "";
  const email = contact?.email ?? "";

  return (
    <Dialog
      open={open}
      onClose={handleDialogClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="resend-invitation-modal-title"
      aria-describedby="resend-invitation-modal-description"
      slotProps={{
        paper: {
          sx: { position: "relative" },
        },
      }}
    >
      <IconButton
        aria-label="Close"
        size="small"
        onClick={onClose}
        disabled={isResending}
        sx={{
          position: "absolute",
          right: 8,
          top: 8,
          zIndex: 1,
        }}
      >
        <X size={18} />
      </IconButton>
      <DialogTitle id="resend-invitation-modal-title">Resend Invitation</DialogTitle>
      <DialogContent>
        <Typography id="resend-invitation-modal-description" color="text.secondary">
          Send the invitation to{" "}
          {name ? (
            <>
              <strong>{name}</strong> ({email})
            </>
          ) : (
            <strong>{email || "this user"}</strong>
          )}{" "}
          again? They will get a reminder email with a link to sign in to the
          Support Portal.
        </Typography>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={onClose} disabled={isResending}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="primary"
          onClick={onConfirm}
          disabled={isResending}
          startIcon={
            isResending ? (
              <CircularProgress size={16} color="inherit" />
            ) : undefined
          }
        >
          {isResending ? "Resending..." : "Yes, Resend"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
