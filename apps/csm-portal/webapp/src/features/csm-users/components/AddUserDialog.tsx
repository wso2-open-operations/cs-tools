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
  MenuItem,
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

/**
 * The two user types this form can create. entity-service's `user_type` has
 * no plain settable column -- it's derived by a DB trigger from role
 * membership (`recompute_user_type`, migration 0011) -- so picking one here
 * means sending the matching role (`internal`/`external`) in `roles`, not a
 * `type` field on the wire. `external` (not `customer`/`partner`/...) is the
 * role every externally-onboarded contact actually holds; the finer-grained
 * ones are refinements applied elsewhere, not choices this form makes.
 *
 * `external` is disabled for now -- the backend rejects it too (see
 * entity-service's own `requestsExternalUserType`) -- so this list only ever
 * offers one real, selectable choice until that's lifted.
 */
const USER_TYPE_OPTIONS = [
  { value: "internal", label: "Internal (WSO2 staff)", role: "internal", disabled: false },
  {
    value: "external",
    label: "External (customer/partner) — currently unavailable",
    role: "external",
    disabled: true,
  },
] as const;

type NewUserType = (typeof USER_TYPE_OPTIONS)[number]["value"];

const WSO2_EMAIL_DOMAIN = "@wso2.com";

function isWso2Email(email: string): boolean {
  return email.toLowerCase().endsWith(WSO2_EMAIL_DOMAIN);
}

const EMPTY_FORM: { firstName: string; lastName: string; email: string; userType: NewUserType | "" } = {
  firstName: "",
  lastName: "",
  email: "",
  userType: "",
};

/**
 * Admin-only "Add User" form (`POST /users`). Sets the new user's type by
 * granting the matching `internal`/`external` role (see `USER_TYPE_OPTIONS`'s
 * own doc comment) -- the only role picker this form has; there is still no
 * identity-provider-backed way to browse/assign a fuller role set at account-creation
 * time, so nothing beyond this one required choice is exposed here.
 *
 * An Internal user must have a `@wso2.com` email -- entity-service enforces
 * this as the real constraint (a non-wso2.com address must never resolve to
 * `user_type = INTERNAL`); this form blocks the same case up front so the
 * admin sees it immediately rather than after a round trip.
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
  const emailValid = isPlausibleEmail(trimmedEmail);
  const internalEmailViolation = form.userType === "internal" && emailValid && !isWso2Email(trimmedEmail);
  const canSubmit = hasName && emailValid && form.userType !== "" && !internalEmailViolation;

  const handleSubmit = (e: FormEvent<HTMLFormElement>): void => {
    e.preventDefault();
    if (!canSubmit) return;
    const selected = USER_TYPE_OPTIONS.find((o) => o.value === form.userType);
    mutate(
      {
        firstName: form.firstName.trim() || undefined,
        lastName: form.lastName.trim() || undefined,
        email: trimmedEmail,
        roles: selected ? [selected.role] : undefined,
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
              error={(form.email.trim() !== "" && !emailValid) || internalEmailViolation}
              helperText={
                form.email.trim() !== "" && !emailValid
                  ? "Enter a valid email address."
                  : internalEmailViolation
                    ? `An internal user must have a ${WSO2_EMAIL_DOMAIN} email address.`
                    : undefined
              }
            />
            <TextField
              select
              label="User type"
              value={form.userType}
              onChange={(e) => setForm({ ...form, userType: e.target.value as NewUserType })}
              fullWidth
              disabled={isPending}
              required
            >
              {USER_TYPE_OPTIONS.map((option) => (
                <MenuItem key={option.value} value={option.value} disabled={option.disabled}>
                  {option.label}
                </MenuItem>
              ))}
            </TextField>
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
