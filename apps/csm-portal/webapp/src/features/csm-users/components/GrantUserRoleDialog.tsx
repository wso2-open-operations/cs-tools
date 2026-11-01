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
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  FormGroup,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type FormEvent, type JSX } from "react";
import { useGetGrantableRoles } from "@features/csm-users/api/useGetGrantableRoles";
import { usePostGrantUserRoles } from "@features/csm-users/api/usePostGrantUserRoles";
import { grantableRoleLabel } from "@features/csm-users/utils/grantableRoleLabels";

export interface GrantUserRoleDialogProps {
  open: boolean;
  onClose: () => void;
  userId: string;
  /** Portal role keys this user already holds (`user.csmPlatformRoles`) —
   * offered unchecked and disabled rather than left off the list entirely, so
   * an admin can see at a glance which roles are already granted. */
  currentRoles: string[];
}

/**
 * Grants one or more existing portal roles to an already-existing user
 * (`POST /users/{id}/roles`) — the `UserProfilePage.tsx` counterpart to
 * `AddUserDialog.tsx`'s "Portal roles" section at creation time, for an admin
 * who needs to add a role after the fact rather than at creation. Reuses the
 * same `GET /roles/grantable` list and `grantableRoleLabel` mapping.
 *
 * Shown unconditionally (not gated behind `usePortalAccess` on this
 * frontend) per this app's standing "show the action, let the backend
 * reject" norm (see this app's own `CLAUDE.md`) — the backend's `PermAdmin`
 * gate on this route is the real boundary, same as it is for `GET
 * /roles/grantable` itself.
 */
export default function GrantUserRoleDialog({
  open,
  onClose,
  userId,
  currentRoles,
}: GrantUserRoleDialogProps): JSX.Element {
  const [selectedRoles, setSelectedRoles] = useState<string[]>([]);
  const { mutate, isPending, error, data, reset } = usePostGrantUserRoles();
  const {
    data: grantableRoles,
    isLoading: grantableRolesLoading,
    isError: grantableRolesErrored,
    refetch: refetchGrantableRoles,
  } = useGetGrantableRoles(open);

  const currentRoleSet = useMemo(() => new Set(currentRoles), [currentRoles]);

  const toggleRole = (key: string, checked: boolean): void => {
    setSelectedRoles((prev) => (checked ? [...prev, key] : prev.filter((k) => k !== key)));
  };

  const handleClose = (): void => {
    if (isPending) return;
    setSelectedRoles([]);
    reset();
    onClose();
  };

  const handleSubmit = (e: FormEvent<HTMLFormElement>): void => {
    e.preventDefault();
    if (selectedRoles.length === 0) return;
    mutate(
      { userId, roles: selectedRoles },
      {
        onSuccess: (result) => {
          // Only clear the roles that actually succeeded, so a partial
          // failure leaves the failed ones checked for a quick retry.
          setSelectedRoles((prev) => prev.filter((r) => !result.granted.includes(r)));
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>Grant a portal role</DialogTitle>
      <Box component="form" onSubmit={handleSubmit} noValidate>
        <DialogContent>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
            {error && (
              <Alert severity="error">{error.message || "Failed to grant the selected role(s)."}</Alert>
            )}
            {data && data.failed && data.failed.length > 0 && (
              <Alert severity="warning">
                Granted {data.granted.length === 0 ? "none" : data.granted.map(grantableRoleLabel).join(", ")};
                failed: {data.failed.map(grantableRoleLabel).join(", ")}.
              </Alert>
            )}
            {data && (!data.failed || data.failed.length === 0) && data.granted.length > 0 && (
              <Alert severity="success">Granted: {data.granted.map(grantableRoleLabel).join(", ")}.</Alert>
            )}
            {grantableRolesLoading && (
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <CircularProgress size={16} />
                <Typography variant="body2" color="text.secondary">
                  Loading portal roles…
                </Typography>
              </Box>
            )}
            {grantableRolesErrored && (
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Typography variant="body2" color="error">
                  Failed to load portal roles.
                </Typography>
                <Button type="button" size="small" onClick={() => refetchGrantableRoles()}>
                  Retry
                </Button>
              </Box>
            )}
            {grantableRoles && grantableRoles.length === 0 && !grantableRolesLoading && (
              <Typography variant="body2" color="text.secondary">
                No grantable portal roles are configured in this deployment.
              </Typography>
            )}
            {grantableRoles && grantableRoles.length > 0 && (
              <FormGroup>
                {grantableRoles.map((role) => {
                  const alreadyGranted = currentRoleSet.has(role.key);
                  return (
                    <FormControlLabel
                      key={role.key}
                      control={
                        <Checkbox
                          checked={alreadyGranted || selectedRoles.includes(role.key)}
                          onChange={(e) => toggleRole(role.key, e.target.checked)}
                          disabled={isPending || alreadyGranted}
                        />
                      }
                      label={alreadyGranted ? `${grantableRoleLabel(role.key)} (already granted)` : grantableRoleLabel(role.key)}
                    />
                  );
                })}
              </FormGroup>
            )}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button type="button" color="inherit" onClick={handleClose} disabled={isPending}>
            Close
          </Button>
          <Button
            type="submit"
            variant="contained"
            disabled={selectedRoles.length === 0 || isPending}
            loading={isPending}
          >
            Grant
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}
