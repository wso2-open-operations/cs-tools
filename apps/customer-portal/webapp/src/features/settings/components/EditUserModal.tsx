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
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Switch,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { Shield, X } from "@wso2/oxygen-ui-icons-react";
import { NULL_PLACEHOLDER } from "@features/settings/constants/settingsConstants";
import { getAvatarColor, getInitials } from "@features/settings/utils/settings";
import type { EditUserModalProps } from "@features/settings/types/settings";

/**
 * Admin-only modal for editing an existing contact.
 * Currently only toggles the Security Contact flag, and only for Portal Users.
 * System Users cannot be converted to security contacts.
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
  const [isSecurity, setIsSecurity] = useState(false);

  useEffect(() => {
    if (open && contact) {
      setIsSecurity(!!contact.isSecurityContact);
    }
  }, [open, contact]);

  const handleClose = useCallback(() => {
    if (isSubmitting) return;
    onClose();
  }, [isSubmitting, onClose]);

  const handleSave = useCallback(() => {
    onSubmit({ isSecurityContact: isSecurity });
  }, [isSecurity, onSubmit]);

  const isSystemUser = !!contact?.isCsIntegrationUser;
  const initialValue = !!contact?.isSecurityContact;
  const isDirty = isSecurity !== initialValue;

  const displayName = contact
    ? contact.firstName && contact.lastName
      ? `${contact.firstName} ${contact.lastName}`
      : contact.firstName ||
        contact.lastName ||
        contact.email ||
        NULL_PLACEHOLDER
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
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Manage this user&apos;s security contact status for the project.
        </Typography>

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

        {isSystemUser ? (
          <Box
            sx={{
              p: 2,
              borderRadius: 1,
              border: "1px solid",
              borderColor: alpha(theme.palette.error.main, 0.3),
              bgcolor: alpha(theme.palette.error.main, 0.06),
            }}
          >
            <Typography variant="subtitle2" color="error" sx={{ mb: 0.5 }}>
              Not available for System Users
            </Typography>
            <Typography variant="caption" color="text.secondary">
              System Users are used for machine-to-machine integrations and
              cannot receive security advisories. To designate a security
              contact, remove this user and invite them again as a Security
              User.
            </Typography>
          </Box>
        ) : (
          <Box
            sx={{
              p: 2,
              borderRadius: 1,
              border: "1px solid",
              borderColor: isSecurity
                ? alpha(theme.palette.error.main, 0.3)
                : "divider",
              bgcolor: isSecurity
                ? alpha(theme.palette.error.main, 0.04)
                : "transparent",
              transition: "background-color 0.2s, border-color 0.2s",
            }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                gap: 2,
              }}
            >
              <Box sx={{ display: "flex", gap: 1.25, flex: 1, minWidth: 0 }}>
                <Box
                  sx={{
                    width: 32,
                    height: 32,
                    borderRadius: 1,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    bgcolor: alpha(theme.palette.error.main, 0.12),
                    color: theme.palette.error.main,
                    flexShrink: 0,
                  }}
                >
                  <Shield size={18} />
                </Box>
                <Box>
                  <Typography variant="subtitle2">Security Contact</Typography>
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ display: "block", mt: 0.25 }}
                  >
                    Designate this user to receive WSO2 security advisories, CVE
                    notifications, and vulnerability reports for the project.
                    They will also be able to create security cases.
                  </Typography>
                </Box>
              </Box>
              <Switch
                checked={isSecurity}
                onChange={(e) => setIsSecurity(e.target.checked)}
                disabled={isSubmitting}
                color="error"
                inputProps={{ "aria-label": "Toggle security contact" }}
              />
            </Box>

            <Box
              sx={{
                mt: 1.5,
                pt: 1.5,
                borderTop: "1px dashed",
                borderColor: "divider",
              }}
            >
              <Typography variant="caption" color="text.secondary">
                {isSecurity && !initialValue && (
                  <>
                    <strong>{displayName}</strong> will be promoted to a
                    Security User and start receiving security notifications
                    once you save.
                  </>
                )}
                {!isSecurity && initialValue && (
                  <>
                    <strong>{displayName}</strong> will be reverted to a regular
                    Portal User and will no longer receive security
                    notifications.
                  </>
                )}
                {isSecurity === initialValue && (
                  <>
                    This user is currently{" "}
                    <strong>
                      {initialValue
                        ? "a Security Contact"
                        : "not a Security Contact"}
                    </strong>
                    . Toggle the switch to change it.
                  </>
                )}
              </Typography>
            </Box>
          </Box>
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button
          variant="outlined"
          onClick={handleClose}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        <Button
          variant="contained"
          color="warning"
          onClick={handleSave}
          disabled={isSubmitting || isSystemUser || !isDirty}
          startIcon={
            isSubmitting ? (
              <CircularProgress size={16} color="inherit" />
            ) : undefined
          }
        >
          {isSubmitting
            ? "Saving..."
            : isDirty
              ? isSecurity
                ? "Make Security Contact"
                : "Remove Security Contact"
              : "Save Changes"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
