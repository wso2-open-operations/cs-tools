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
import type { ProjectContact } from "@features/settings/types/users";

export interface RemoveUserModalProps {
  open: boolean;
  contact: ProjectContact | null;
  isDeleting?: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

/**
 * Confirmation modal before removing a user from the project.
 *
 * @param {RemoveUserModalProps} props - open, contact, onClose, onConfirm.
 * @returns {JSX.Element} The confirmation modal.
 */
export default function RemoveUserModal({
  open,
  contact,
  isDeleting = false,
  onClose,
  onConfirm,
}: RemoveUserModalProps): JSX.Element {
  const handleDialogClose = useCallback(
    (_event: object, _reason: string) => {
      if (!isDeleting) onClose();
    },
    [onClose, isDeleting],
  );

  const displayName = contact
    ? contact.firstName && contact.lastName
      ? `${contact.firstName} ${contact.lastName}`
      : contact.firstName || contact.lastName || contact.email || "this user"
    : "this user";

  return (
    <Dialog
      open={open}
      onClose={handleDialogClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="remove-user-modal-title"
      aria-describedby="remove-user-modal-description"
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
      <DialogTitle id="remove-user-modal-title">Remove User</DialogTitle>
      <DialogContent>
        <Typography id="remove-user-modal-description" color="text.secondary">
          Are you sure you want to remove <strong>{displayName}</strong> from
          this project?
        </Typography>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={onClose} disabled={isDeleting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={onConfirm}
          disabled={isDeleting}
          startIcon={
            isDeleting ? (
              <CircularProgress size={16} color="inherit" />
            ) : undefined
          }
        >
          {isDeleting ? "Removing..." : "Yes, Remove"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
