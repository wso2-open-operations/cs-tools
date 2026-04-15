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
import { Clock } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

export type MissingTimezoneDialogVariant = "informational" | "required";

export interface MissingTimezoneDialogProps {
  open: boolean;
  onClose: () => void;
  /** Called when user clicks the "Set Time Zone" button. */
  onSetTimeZone: () => void;
  variant?: MissingTimezoneDialogVariant;
}

/**
 * Dialog shown when the user has no time zone configured on their profile.
 * Prompts the user to open their profile modal to set a time zone.
 *
 * @param {MissingTimezoneDialogProps} props - open, onClose, onSetTimeZone, variant.
 * @returns {JSX.Element} The missing timezone dialog.
 */
export default function MissingTimezoneDialog({
  open,
  onClose,
  onSetTimeZone,
  variant = "informational",
}: MissingTimezoneDialogProps): JSX.Element {
  const isRequired = variant === "required";

  const handleDialogClose = (
    _event: unknown,
    reason: "backdropClick" | "escapeKeyDown",
  ) => {
    if (
      isRequired &&
      (reason === "backdropClick" || reason === "escapeKeyDown")
    ) {
      return;
    }
    onClose();
  };

  return (
    <Dialog
      open={open}
      onClose={handleDialogClose}
      maxWidth="xs"
      fullWidth
      aria-labelledby="missing-timezone-dialog-title"
      aria-describedby="missing-timezone-dialog-description"
    >
      <DialogTitle
        id="missing-timezone-dialog-title"
        sx={{ display: "flex", alignItems: "center", gap: 1, pb: 1 }}
      >
        <Clock size={20} aria-hidden />
        Time Zone Not Set
      </DialogTitle>
      <DialogContent>
        <Typography
          id="missing-timezone-dialog-description"
          variant="body2"
          color="text.secondary"
        >
          {isRequired
            ? "Set your time zone first to request or reschedule a call. Go to your profile to choose your time zone."
            : "Your profile does not have a time zone configured. Please set your time zone so call request times are displayed accurately in your local time."}
        </Typography>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2.5, gap: 1 }}>
        {!isRequired && (
          <Button variant="outlined" onClick={onClose}>
            Later
          </Button>
        )}
        <Button variant="contained" color="primary" onClick={onSetTimeZone}>
          Set Time Zone
        </Button>
      </DialogActions>
    </Dialog>
  );
}
