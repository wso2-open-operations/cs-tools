// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License
// at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useCallback, useEffect, useState, type JSX } from "react";
import {
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  IconButton,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { Lock, Settings, Shield, User, X } from "@wso2/oxygen-ui-icons-react";
import { NULL_PLACEHOLDER } from "@features/settings/constants/settingsConstants";
import { getAvatarColor, getInitials } from "@features/settings/utils/settings";
import type { EditUserModalProps } from "@features/settings/types/settings";

const USER_ROLES = [
  {
    id: "portal",
    label: "Portal User",
    description: "Can access and manage general support cases",
    Icon: User,
    color: "primary" as const,
  },
  {
    id: "system",
    label: "System User",
    description: "API access only, does not have support portal access",
    Icon: Settings,
    color: "warning" as const,
  },
  {
    id: "admin",
    label: "Admin User",
    description: "Full administrative privileges and user management",
    Icon: Lock,
    color: "error" as const,
  },
  {
    id: "security",
    label: "Security User",
    description: "Receives security bulletins and critical security announcements",
    Icon: Shield,
    color: "error" as const,
  },
] as const;

type RoleId = (typeof USER_ROLES)[number]["id"];

/**
 * Admin-only modal for editing a user's roles.
 * System User selection makes other roles readonly.
 * Only the Security User role is wired to the API (isSecurityContact); others are UI-only.
 *
 * @param {EditUserModalProps} props - Modal props.
 * @returns {JSX.Element} The modal.
 */
export default function EditUserModal({
  open,
  contact,
  isSubmitting = false,
  onClose,
  onSubmit,
}: EditUserModalProps): JSX.Element {
  const theme = useTheme();
  const [selectedRoles, setSelectedRoles] = useState<Set<RoleId>>(new Set());

  useEffect(() => {
    if (open && contact) {
      const initial = new Set<RoleId>();
      if (contact.isCsIntegrationUser) {
        initial.add("system");
      } else {
        initial.add("portal");
        if (contact.isSecurityContact) initial.add("security");
      }
      setSelectedRoles(initial);
    }
  }, [open, contact]);

  const handleClose = useCallback(() => {
    if (isSubmitting) return;
    onClose();
  }, [isSubmitting, onClose]);

  const isSystemSelected = selectedRoles.has("system");

  const handleRoleToggle = (roleId: RoleId) => {
    setSelectedRoles((prev) => {
      const next = new Set(prev);
      if (roleId === "system") {
        if (next.has("system")) {
          next.delete("system");
          next.add("portal");
        } else {
          next.clear();
          next.add("system");
        }
      } else {
        if (next.has(roleId)) {
          next.delete(roleId);
        } else {
          next.add(roleId);
        }
      }
      return next;
    });
  };

  const handleSave = useCallback(() => {
    onSubmit({ isSecurityContact: selectedRoles.has("security") });
  }, [selectedRoles, onSubmit]);

  const initialIsSecurity = !!contact?.isSecurityContact;
  const isDirty = selectedRoles.has("security") !== initialIsSecurity;

  const displayName = contact
    ? contact.firstName && contact.lastName
      ? `${contact.firstName} ${contact.lastName}`
      : contact.firstName || contact.lastName || contact.email || NULL_PLACEHOLDER
    : NULL_PLACEHOLDER;

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="edit-user-modal-title"
      slotProps={{ paper: { sx: { position: "relative" } } }}
    >
      <IconButton
        aria-label="Close"
        size="small"
        onClick={handleClose}
        disabled={isSubmitting}
        sx={{ position: "absolute", right: 8, top: 8, zIndex: 1 }}
      >
        <X size={18} />
      </IconButton>
      <DialogTitle id="edit-user-modal-title">Edit User</DialogTitle>
      <DialogContent>
        {/* User info header */}
        {contact && (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              p: 1.5,
              mb: 2.5,
              borderRadius: 1,
              border: "1px solid",
              borderColor: "divider",
              bgcolor: alpha(theme.palette.text.primary, 0.02),
            }}
          >
            <Box
              sx={{
                width: 44,
                height: 44,
                borderRadius: "50%",
                bgcolor: getAvatarColor(contact.id ?? contact.email ?? ""),
                color: "white",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                typography: "body1",
                fontWeight: 600,
                flexShrink: 0,
              }}
            >
              {getInitials(contact.firstName, contact.lastName, contact.email)}
            </Box>
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="body2" sx={{ fontWeight: 600 }} noWrap>
                {displayName}
              </Typography>
              <Typography variant="caption" color="text.secondary" noWrap>
                {contact.email ?? NULL_PLACEHOLDER}
              </Typography>
            </Box>
          </Box>
        )}

        <Typography variant="subtitle2" sx={{ mb: 1.5 }}>
          User Roles
        </Typography>
        {isSystemSelected && (
          <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1.5 }}>
            System User is selected — other roles are not available for system users.
          </Typography>
        )}

        <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
          {USER_ROLES.map((role, index) => {
            const RoleIcon = role.Icon;
            const isSystem = role.id === "system";
            const isChecked = selectedRoles.has(role.id);
            const isReadonly = !isSystem && isSystemSelected;
            const paletteColor = theme.palette[role.color];

            return (
              <Box key={role.id}>
                {index > 0 && <Divider sx={{ my: 0.5 }} />}
                <FormControlLabel
                  control={
                    <Checkbox
                      checked={isChecked}
                      onChange={() => !isReadonly && !isSubmitting && handleRoleToggle(role.id)}
                      disabled={isReadonly || isSubmitting}
                      size="small"
                      sx={{
                        color: isReadonly ? "action.disabled" : paletteColor?.main,
                        "&.Mui-checked": { color: paletteColor?.main },
                      }}
                    />
                  }
                  label={
                    <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1, py: 0.5 }}>
                      <Box
                        sx={{
                          width: 28,
                          height: 28,
                          borderRadius: 0.75,
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                          bgcolor: isReadonly
                            ? alpha(theme.palette.action.disabled, 0.08)
                            : alpha(paletteColor?.main ?? "#000", 0.1),
                          color: isReadonly
                            ? "action.disabled"
                            : paletteColor?.main,
                          flexShrink: 0,
                          mt: 0.25,
                        }}
                      >
                        <RoleIcon size={14} />
                      </Box>
                      <Box>
                        <Typography
                          variant="body2"
                          sx={{
                            fontWeight: 500,
                            color: isReadonly ? "text.disabled" : "text.primary",
                          }}
                        >
                          {role.label}
                        </Typography>
                        <Typography
                          variant="caption"
                          color={isReadonly ? "text.disabled" : "text.secondary"}
                        >
                          {role.description}
                        </Typography>
                      </Box>
                    </Box>
                  }
                  sx={{ alignItems: "flex-start", mx: 0, width: "100%" }}
                />
              </Box>
            );
          })}
        </Box>

        {isSystemSelected && (
          <Box
            sx={{
              mt: 2,
              p: 1.5,
              borderRadius: 1,
              border: "1px solid",
              borderColor: alpha(theme.palette.warning.main, 0.3),
              bgcolor: alpha(theme.palette.warning.main, 0.05),
            }}
          >
            <Typography variant="caption" color="text.secondary">
              System Users are used for machine-to-machine integrations and cannot hold additional roles.
            </Typography>
          </Box>
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={handleClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="warning"
          onClick={handleSave}
          disabled={isSubmitting || isSystemSelected || !isDirty}
          startIcon={
            isSubmitting ? <CircularProgress size={16} color="inherit" /> : undefined
          }
        >
          {isSubmitting ? "Saving..." : "Save Changes"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
