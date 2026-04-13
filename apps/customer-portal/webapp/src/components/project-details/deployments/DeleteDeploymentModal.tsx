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
  Typography,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useCallback, type JSX } from "react";
import type { ProjectDeploymentItem } from "@/types/deployments";

export interface DeleteDeploymentModalProps {
  open: boolean;
  deployment: ProjectDeploymentItem | null;
  onClose: () => void;
  onConfirm: () => void;
  isDeleting?: boolean;
}

/**
 * Confirmation modal before deleting a deployment.
 * Delete is implemented as PATCH with { active: false }.
 *
 * @param {DeleteDeploymentModalProps} props - open, deployment, onClose, onConfirm, isDeleting.
 * @returns {JSX.Element} The confirmation modal.
 */
export default function DeleteDeploymentModal({
  open,
  deployment,
  onClose,
  onConfirm,
  isDeleting = false,
}: DeleteDeploymentModalProps): JSX.Element {
  const handleDialogClose = useCallback(
    (_event: object, _reason: string) => {
      if (isDeleting) return;
      onClose();
    },
    [isDeleting, onClose],
  );

  return (
    <Dialog
      open={open}
      onClose={handleDialogClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="delete-deployment-modal-title"
      aria-describedby="delete-deployment-modal-description"
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
      <DialogTitle id="delete-deployment-modal-title">
        Confirm Action
      </DialogTitle>
      <DialogContent>
        <Typography id="delete-deployment-modal-description" color="text.secondary">
          {deployment
            ? `Are you sure you want to delete the deployment "${deployment.name ?? "Untitled"}"? This action cannot be undone.`
            : "Are you sure you want to delete this deployment? This action cannot be undone."}
        </Typography>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={onClose} disabled={isDeleting}>
          Go Back
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={onConfirm}
          disabled={isDeleting}
          startIcon={isDeleting ? <CircularProgress size={16} color="inherit" /> : undefined}
        >
          {isDeleting ? "Deleting..." : "Confirm"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
