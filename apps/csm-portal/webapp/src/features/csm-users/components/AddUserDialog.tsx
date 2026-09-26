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
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type FormEvent, type JSX } from "react";
import { usePostUser } from "@features/csm-users/api/usePostUser";
import { isPlausibleEmail } from "@features/csm-users/utils/isPlausibleEmail";

export interface AddUserDialogProps {
  open: boolean;
  onClose: () => void;
  /** Called once the user is created, so the caller can e.g. show a toast. */
  onCreated?: (userId: string | undefined) => void;
}

const EMPTY_FORM = { firstName: "", lastName: "", email: "" };

/**
 * Admin-only "Add User" form (`POST /users`). No role picker: `roles` is
 * accepted end-to-end by the backend and entity service, but there is no
 * Asgardeo-backed way to browse/assign roles at account-creation time yet, so
 * it's simply omitted from this form for now.
 */
export default function AddUserDialog({ open, onClose, onCreated }: AddUserDialogProps): JSX.Element {
  const [form, setForm] = useState(EMPTY_FORM);
  const { mutate, isPending, error, reset } = usePostUser();

  const handleClose = (): void => {
    if (isPending) return;
    setForm(EMPTY_FORM);
    reset();
    onClose();
  };

  const trimmedEmail = form.email.trim();
  const hasName = form.firstName.trim() !== "" || form.lastName.trim() !== "";
  const canSubmit = hasName && isPlausibleEmail(trimmedEmail);

  const handleSubmit = (e: FormEvent<HTMLFormElement>): void => {
    e.preventDefault();
    if (!canSubmit) return;
    mutate(
      {
        firstName: form.firstName.trim() || undefined,
        lastName: form.lastName.trim() || undefined,
        email: trimmedEmail,
      },
      {
        onSuccess: (created) => {
          setForm(EMPTY_FORM);
          onCreated?.(created.id);
          onClose();
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>Add user</DialogTitle>
      <Box component="form" onSubmit={handleSubmit} noValidate>
        <DialogContent>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
            {error && (
              <Typography variant="body2" color="error">
                {error.message || "Failed to create the user."}
              </Typography>
            )}
            <TextField
              label="First name"
              value={form.firstName}
              onChange={(e) => setForm({ ...form, firstName: e.target.value })}
              fullWidth
              disabled={isPending}
              autoFocus
            />
            <TextField
              label="Last name"
              value={form.lastName}
              onChange={(e) => setForm({ ...form, lastName: e.target.value })}
              fullWidth
              disabled={isPending}
            />
            <TextField
              label="Email"
              type="email"
              value={form.email}
              onChange={(e) => setForm({ ...form, email: e.target.value })}
              fullWidth
              disabled={isPending}
              required
              error={form.email.trim() !== "" && !isPlausibleEmail(trimmedEmail)}
              helperText={
                form.email.trim() !== "" && !isPlausibleEmail(trimmedEmail)
                  ? "Enter a valid email address."
                  : undefined
              }
            />
            {!hasName && (
              <Typography variant="caption" color="text.secondary">
                At least a first or last name is required.
              </Typography>
            )}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button type="button" color="inherit" onClick={handleClose} disabled={isPending}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="contained"
            disabled={!canSubmit || isPending}
            loading={isPending}
          >
            Add user
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
