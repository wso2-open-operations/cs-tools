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
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { usePatchUser } from "@features/csm-users/api/usePatchUser";

export interface MakeUserInternalDialogProps {
  open: boolean;
  onClose: () => void;
  userId: string;
  userEmail: string;
}

/**
 * Confirms, then flips an existing @wso2.com user from external to internal
 * via `PATCH /users/{id}` with `{makeInternal: true}` (`usePatchUser`, the
 * same hook/route the team-lead phone-number update already uses — a
 * dedicated `POST /users/{id}/internalize` was the first design, folded into
 * this existing endpoint once it was clear it could serve the same request)
 * — grants the "internal" role and revokes the external-side role set
 * (external/customer/partner/customer_admin/partner_admin), the only way to
 * actually change `userType` (trigger-derived from role membership on the
 * entity-service side, not a plain settable column). Only shown for a
 * @wso2.com target that isn't internal already (`UserProfilePage.tsx`'s own
 * gating), and only to an admin (`usePortalAccess().canMakeUserInternal`) —
 * a deliberate frontend-hide exception, same shape as `AddUserDialog.tsx`'s
 * "Add user" gating, since the backend genuinely enforces the identical
 * restriction (`PermAdmin`, see `apps/csm-portal/backend`'s own CLAUDE.md).
 *
 * The caller (`UserProfilePage.tsx`) keys the component that owns this
 * dialog's `open` state by `user.id`, so navigating to a different user's
 * profile remounts both the dialog's open flag and this mutation's result —
 * without that, a success/error message from one user's confirmation could
 * still be showing (misleadingly) after switching to another user's profile.
 */
export default function MakeUserInternalDialog({
  open,
  onClose,
  userId,
  userEmail,
}: MakeUserInternalDialogProps): JSX.Element {
  const { mutate, isPending, error, isSuccess, reset } = usePatchUser();

  const handleClose = (): void => {
    if (isPending) return;
    reset();
    onClose();
  };

  const handleConfirm = (): void => {
    mutate({ id: userId, patch: { makeInternal: true } });
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>Make this user internal</DialogTitle>
      <DialogContent>
        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error.message || "Failed to make this user internal."}
          </Alert>
        )}
        {isSuccess && (
          <Alert severity="success" sx={{ mb: 2 }}>
            {userEmail} is now internal.
          </Alert>
        )}
        {!isSuccess && (
          <Typography variant="body2" color="text.secondary">
            This grants <strong>{userEmail}</strong> the Internal role and removes any
            External/Customer/Partner role they currently hold. This action cannot be undone
            from here.
          </Typography>
        )}
      </DialogContent>
      <DialogActions>
        <Button type="button" color="inherit" onClick={handleClose} disabled={isPending}>
          {isSuccess ? "Close" : "Cancel"}
        </Button>
        {!isSuccess && (
          <Button
            type="button"
            variant="contained"
            onClick={handleConfirm}
            disabled={isPending}
            loading={isPending}
          >
            Make internal
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
