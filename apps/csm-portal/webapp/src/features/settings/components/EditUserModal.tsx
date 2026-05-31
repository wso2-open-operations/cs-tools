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
  colors,
  useTheme,
} from "@wso2/oxygen-ui";
import { Crown, Monitor, Settings, Shield, X } from "@wso2/oxygen-ui-icons-react";
import { NULL_PLACEHOLDER } from "@features/settings/constants/settingsConstants";
import { getAvatarColor, getInitials } from "@features/settings/utils/settings";
import type { EditUserModalProps } from "@features/settings/types/settings";

const EDITABLE_ROLES = [
  {
    id: "admin" as const,
    label: "Admin",
    description: "Full administrative privileges and user management",
    Icon: Crown,
    color: "secondary" as const,
  },
  {
    id: "portal" as const,
    label: "Portal User",
    description: "Can log in to and access the Support Portal",
    Icon: Monitor,
    color: "primary" as const,
  },
  {
    id: "security" as const,
    label: "Security Contact",
    description: "Receives security bulletins and critical security announcements",
    Icon: Shield,
    color: "error" as const,
  },
] as const;

type EditableRoleId = (typeof EDITABLE_ROLES)[number]["id"];

/**
 * Admin-only modal for editing a user's membership roles.
 * Supports isCsAdmin, isPortalUser (web user), and isSecurityContact flags.
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
  const [selectedRoles, setSelectedRoles] = useState<Set<EditableRoleId>>(new Set());

  const getInitialRoles = useCallback(() => {
    if (!contact) return new Set<EditableRoleId>();
    const initial = new Set<EditableRoleId>();
    if (contact.isCsAdmin) initial.add("admin");
    // isPortalUser is the explicit flag; fall back to !isCsIntegrationUser if absent
    const portalUser = contact.isPortalUser ?? !contact.isCsIntegrationUser;
    if (portalUser) initial.add("portal");
    if (contact.isSecurityContact) initial.add("security");
    return initial;
  }, [contact]);

  useEffect(() => {
    if (open && contact) {
      setSelectedRoles(getInitialRoles());
    }
  }, [open, contact, getInitialRoles]);

  const handleClose = useCallback(() => {
    if (isSubmitting) return;
    onClose();
  }, [isSubmitting, onClose]);

  const handleRoleToggle = (roleId: EditableRoleId) => {
    setSelectedRoles((prev) => {
      const next = new Set(prev);
      if (next.has(roleId)) {
        next.delete(roleId);
      } else {
        next.add(roleId);
      }
      return next;
    });
  };

  const handleSave = useCallback(() => {
    onSubmit({
      isCsAdmin: selectedRoles.has("admin"),
      isPortalUser: selectedRoles.has("portal"),
      isSecurityContact: selectedRoles.has("security"),
    });
  }, [selectedRoles, onSubmit]);

  const initialRoles = getInitialRoles();
  const isDirty =
    selectedRoles.has("admin") !== initialRoles.has("admin") ||
    selectedRoles.has("portal") !== initialRoles.has("portal") ||
    selectedRoles.has("security") !== initialRoles.has("security");

  const displayName = contact
    ? contact.firstName && contact.lastName
      ? `${contact.firstName} ${contact.lastName}`
      : contact.firstName || contact.lastName || contact.email || NULL_PLACEHOLDER
    : NULL_PLACEHOLDER;

  const isSystemUser = !!contact?.isCsIntegrationUser;

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
      <DialogTitle id="edit-user-modal-title">Edit User Roles</DialogTitle>
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
            <Box sx={{ minWidth: 0, flex: 1 }}>
              <Typography variant="body2" sx={{ fontWeight: 600 }} noWrap>
                {displayName}
              </Typography>
              <Typography variant="caption" color="text.secondary" noWrap>
                {contact.email ?? NULL_PLACEHOLDER}
              </Typography>
            </Box>
          </Box>
        )}

        {/* System user indicator */}
        {isSystemUser && (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              p: 1.5,
              mb: 2,
              borderRadius: 1,
              border: "1px solid",
              borderColor: alpha(theme.palette.warning.main, 0.3),
              bgcolor: alpha(theme.palette.warning.main, 0.05),
            }}
          >
            <Settings size={16} color={theme.palette.warning.main} />
            <Box>
              <Typography variant="body2" sx={{ fontWeight: 500, color: "warning.dark" }}>
                System User
              </Typography>
              <Typography variant="caption" color="text.secondary">
                Used for API / machine-to-machine integrations.
              </Typography>
            </Box>
          </Box>
        )}

        <Typography variant="subtitle2" sx={{ mb: 1.5 }}>
          Membership Roles
        </Typography>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
          {EDITABLE_ROLES.map((role, index) => {
            const RoleIcon = role.Icon;
            const isChecked = selectedRoles.has(role.id);
            const resolvedColor =
              role.color === "secondary"
                ? (colors.purple?.[600] ?? "#7c3aed")
                : theme.palette[role.color]?.main ?? theme.palette.text.primary;

            return (
              <Box key={role.id}>
                {index > 0 && <Divider sx={{ my: 0.5 }} />}
                <FormControlLabel
                  control={
                    <Checkbox
                      checked={isChecked}
                      onChange={() => !isSubmitting && handleRoleToggle(role.id)}
                      disabled={isSubmitting}
                      size="small"
                      sx={{
                        color: resolvedColor,
                        "&.Mui-checked": { color: resolvedColor },
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
                          bgcolor: alpha(resolvedColor, 0.1),
                          color: resolvedColor,
                          flexShrink: 0,
                          mt: 0.25,
                        }}
                      >
                        <RoleIcon size={14} />
                      </Box>
                      <Box>
                        <Typography variant="body2" sx={{ fontWeight: 500 }}>
                          {role.label}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
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
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" onClick={handleClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="warning"
          onClick={handleSave}
          disabled={isSubmitting || !isDirty || selectedRoles.size === 0}
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
