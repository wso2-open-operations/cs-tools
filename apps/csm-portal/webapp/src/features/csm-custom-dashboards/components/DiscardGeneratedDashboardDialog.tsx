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

import { Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface DiscardGeneratedDashboardDialogProps {
  open: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  /** Leaving an edit of a saved dashboard: only the changes are lost. */
  editing?: boolean;
}

/**
 * Confirms leaving the generate page with work on it. The same shape as the
 * builder's DiscardDraftDialog, with its own wording: a generated draft has
 * no deployed version to fall back to, and it is not kept anywhere yet.
 */
export default function DiscardGeneratedDashboardDialog({
  open,
  onCancel,
  onConfirm,
  editing = false,
}: DiscardGeneratedDashboardDialogProps): JSX.Element {
  return (
    <Dialog open={open} onClose={onCancel} maxWidth="xs" fullWidth>
      <DialogTitle>{editing ? "Discard your changes?" : "Discard this dashboard?"}</DialogTitle>
      <DialogContent>
        <Typography variant="body2">
          {editing
            ? "Your changes and the chat will be lost. The saved dashboard stays as it was."
            : "The dashboard and the chat will be lost. It has not been saved, and this cannot be undone."}
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel}>Cancel</Button>
        <Button variant="contained" color="error" onClick={onConfirm}>
          Discard
        </Button>
      </DialogActions>
    </Dialog>
  );
}
