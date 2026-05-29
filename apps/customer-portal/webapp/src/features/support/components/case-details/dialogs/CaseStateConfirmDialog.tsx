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

import { type JSX } from "react";
import {
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";

export interface CaseStateConfirmDialogProps {
  open: boolean;
  actionLabel: string;
  isPending: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

export default function CaseStateConfirmDialog({
  open,
  actionLabel,
  isPending,
  onClose,
  onConfirm,
}: CaseStateConfirmDialogProps): JSX.Element {
  return (
    <Dialog open={open} onClose={isPending ? undefined : onClose} maxWidth="xs" fullWidth>
      <DialogTitle
        sx={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}
      >
        Confirm State Change
        <IconButton size="small" onClick={onClose} aria-label="close" disabled={isPending}>
          <X size={18} />
        </IconButton>
      </DialogTitle>
      <DialogContent>
        <DialogContentText>
          Are you sure you want to{" "}
          <strong>{actionLabel.toLowerCase()}</strong> this case?
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isPending}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={onConfirm}
          disabled={isPending}
          startIcon={
            isPending ? <CircularProgress size={16} color="inherit" /> : undefined
          }
        >
          {isPending ? "Updating…" : "Confirm"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
