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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";

export interface SessionWarningDialogProps {
  open: boolean;
  onContinue: () => void;
  onLogout: () => void;
}

/**
 * Dialog that asks "Are you still there?" when the user has been idle.
 * Continue resets the idle timer; Logout signs the user out.
 *
 * @param {SessionWarningDialogProps} props - open, onContinue, onLogout.
 * @returns {JSX.Element} The session warning dialog.
 */
export default function SessionWarningDialog({
  open,
  onContinue,
  onLogout,
}: SessionWarningDialogProps): JSX.Element {
  return (
    <Dialog
      open={open}
      onClose={() => {
        // Explicit dialog actions only (Continue / Logout).
      }}
      maxWidth="sm"
      fullWidth
      aria-labelledby="session-warning-dialog-title"
    >
      <DialogTitle id="session-warning-dialog-title">
        Are you still there?
      </DialogTitle>
      <DialogContent>
        <Typography color="text.secondary">
          It looks like you&apos;ve been inactive for a while. Would you like to
          continue?
        </Typography>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={onLogout}>
          Logout
        </Button>
        <Button variant="contained" color="primary" onClick={onContinue}>
          Continue
        </Button>
      </DialogActions>
    </Dialog>
  );
}
