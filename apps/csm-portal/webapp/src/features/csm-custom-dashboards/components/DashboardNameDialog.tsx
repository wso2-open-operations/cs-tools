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

import { Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField } from "@wso2/oxygen-ui";
import { type FormEvent, type JSX, useState } from "react";

/** The longest name a saved dashboard can have, in characters (entity-service's limit). */
export const MAX_DASHBOARD_NAME_LENGTH = 255;

interface DashboardNameDialogProps {
  open: boolean;
  title: string;
  /** The name the field starts with each time the dialog opens. */
  initialName: string;
  /** The confirm button's label, and the one it shows while `busy`. */
  confirmLabel: string;
  busyLabel: string;
  /** While the request is in flight: the field and both buttons are disabled. */
  busy: boolean;
  /**
   * Why the last confirm was refused, shown under the field until the name is
   * changed; `null` for none.
   */
  error: string | null;
  /** Confirm stays off until the name differs from `initialName`, ignoring spaces at the ends. */
  requireChange?: boolean;
  onCancel: () => void;
  /** Called with the name, trimmed. */
  onConfirm: (name: string) => void;
}

/**
 * Asks for a saved dashboard's name: the same shape as
 * DeleteCustomDashboardDialog, with one field. The name is trimmed; confirm
 * is off while it is empty or longer than MAX_DASHBOARD_NAME_LENGTH, and
 * Enter in the field confirms. A refusal (`error`) shows under the field, so
 * the user fixes the name where they typed it; editing the name hides it.
 */
export default function DashboardNameDialog({
  open,
  title,
  initialName,
  confirmLabel,
  busyLabel,
  busy,
  error,
  requireChange = false,
  onCancel,
  onConfirm,
}: DashboardNameDialogProps): JSX.Element {
  const [name, setName] = useState(initialName);
  // The name the shown error is about: once the field differs, it is hidden.
  const [submitted, setSubmitted] = useState<string | null>(null);
  // Each opening starts again from initialName (adjusted during render, not
  // in an effect, so the field never shows the previous opening's text).
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setName(initialName);
      setSubmitted(null);
    }
  }

  const trimmed = name.trim();
  const tooLong = [...trimmed].length > MAX_DASHBOARD_NAME_LENGTH;
  const unchanged = requireChange && trimmed === initialName.trim();
  const canConfirm = !busy && trimmed !== "" && !tooLong && !unchanged;
  const shownError = error !== null && submitted === trimmed ? error : null;

  const handleSubmit = (e: FormEvent): void => {
    e.preventDefault();
    if (!canConfirm) return;
    setSubmitted(trimmed);
    onConfirm(trimmed);
  };

  return (
    <Dialog open={open} onClose={busy ? undefined : onCancel} maxWidth="xs" fullWidth>
      <form onSubmit={handleSubmit} noValidate>
        <DialogTitle>{title}</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            fullWidth
            size="small"
            label="Name"
            value={name}
            disabled={busy}
            onChange={(e) => setName(e.target.value)}
            error={tooLong || shownError !== null}
            helperText={tooLong ? `Use at most ${MAX_DASHBOARD_NAME_LENGTH} characters.` : shownError}
            sx={{ mt: 1 }}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={onCancel} disabled={busy}>
            Cancel
          </Button>
          <Button type="submit" variant="contained" disabled={!canConfirm}>
            {busy ? busyLabel : confirmLabel}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
