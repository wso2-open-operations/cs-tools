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

interface DeleteCustomDashboardDialogProps {
  open: boolean;
  /** The dashboard's name, shown in the question. */
  name: string;
  /** While the delete is in flight: both buttons are disabled. */
  deleting: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

/**
 * Confirms deleting a saved dashboard from My Dashboards. The same shape as
 * DiscardGeneratedDashboardDialog. A deleted dashboard cannot be brought
 * back: the only way to have it again is to generate it again.
 */
export default function DeleteCustomDashboardDialog({
  open,
  name,
  deleting,
  onCancel,
  onConfirm,
}: DeleteCustomDashboardDialogProps): JSX.Element {
  return (
    <Dialog open={open} onClose={deleting ? undefined : onCancel} maxWidth="xs" fullWidth>
      <DialogTitle>Delete this dashboard?</DialogTitle>
      <DialogContent>
        <Typography variant="body2">
          &ldquo;{name}&rdquo; will be removed from My Dashboards. This cannot be undone.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} disabled={deleting}>
          Cancel
        </Button>
        <Button variant="contained" color="error" onClick={onConfirm} disabled={deleting}>
          {deleting ? "Deleting…" : "Delete"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
