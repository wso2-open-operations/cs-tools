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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useState, useMemo, useCallback, type JSX } from "react";
import {
  Box,
  Button,
  Chip,
  Grid,
  IconButton,
  InputAdornment,
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
  alpha,
  colors,
  useTheme,
} from "@wso2/oxygen-ui";
import {
  PencilLine,
  Plus,
  Search,
  Shield,
  Trash2,
} from "@wso2/oxygen-ui-icons-react";
import useGetProjectContacts from "@api/useGetProjectContacts";
import { usePostProjectContact } from "@api/usePostProjectContact";
import { useDeleteProjectContact } from "@api/useDeleteProjectContact";
import { usePatchProjectContact } from "@api/usePatchProjectContact";
import { NULL_PLACEHOLDER, ROLE_CONFIG } from "@constants/settingsConstants";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import AddUserModal from "@components/settings/AddUserModal";
import EditUserModal from "@components/settings/EditUserModal";
import RemoveUserModal from "@components/settings/RemoveUserModal";
import {
  getAvatarColor,
  getInitials,
  getRoleBadges,
  getRoleChipSx,
} from "@utils/settings";
import { getUserStatusColor } from "@utils/projectDetails";
import type { CreateProjectContactRequest } from "@models/requests";
import type { ProjectContact } from "@models/responses";

export interface SettingsUserManagementProps {
  projectId: string;
  /** When false, hide Add User and Delete user buttons. Default true for backward compatibility. */
  canAddOrRemoveUsers?: boolean;
}

/**
 * User management section: stat cards, search, table, role permissions.
 *
 * @param {SettingsUserManagementProps} props - Component props.
 * @returns {JSX.Element} The component.
 */
export default function SettingsUserManagement({
  projectId,
  canAddOrRemoveUsers = true,
}: SettingsUserManagementProps): JSX.Element {
  const theme = useTheme();
  const [searchQuery, setSearchQuery] = useState("");
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<ProjectContact | null>(null);
  const [editTarget, setEditTarget] = useState<ProjectContact | null>(null);

  const { data: contacts = [], isLoading, isFetching, error } = useGetProjectContacts(projectId);
  const postContact = usePostProjectContact(projectId);
  const deleteContact = useDeleteProjectContact(projectId);
  const patchContact = usePatchProjectContact(projectId);
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();

  const filteredContacts = useMemo(() => {
    if (!searchQuery.trim()) return contacts;
    const normalizedQuery = searchQuery.toLowerCase();
    return contacts.filter((contact) => {
      const roleLabels = getRoleBadges(contact)
        .map((badge) => badge.label)
        .join(" ")
        .toLowerCase();
      return (
        contact.email?.toLowerCase().includes(normalizedQuery) ||
        contact.firstName?.toLowerCase().includes(normalizedQuery) ||
        contact.lastName?.toLowerCase().includes(normalizedQuery) ||
        roleLabels.includes(normalizedQuery)
      );
    });
  }, [contacts, searchQuery]);

  // const stats = useMemo(() => ({
  //   admins: contacts.filter((c) => c.isCsAdmin).length,
  //   developers: contacts.filter((c) => c.isCsIntegrationUser).length,
  //   security: contacts.filter((c) => c.isSecurityContact).length,
  // }), [contacts]);

  const handleAddUser = useCallback(
    (data: CreateProjectContactRequest) => {
      postContact.mutate(data, {
        onSuccess: () => {
          setIsAddModalOpen(false);
          showSuccess("Invitation sent successfully");
        },
        onError: (err) => {
          showError(err?.message ?? "Failed to add user. Please try again.");
        },
      });
    },
    [postContact, showSuccess, showError],
  );

  const handleRemoveUser = useCallback(() => {
    if (!removeTarget?.email) return;

    deleteContact.mutate(removeTarget.email, {
      onSuccess: () => {
        setRemoveTarget(null);
        showSuccess("User removed successfully");
      },
      onError: (err) => {
        showError(err?.message ?? "Failed to remove user. Please try again.");
      },
    });
  }, [removeTarget, deleteContact, showSuccess, showError]);

  const handleEditUser = useCallback(
    (next: { isSecurityContact: boolean }) => {
      if (!editTarget?.email) return;
      if (editTarget.isCsIntegrationUser) {
        showError("System Users cannot be security contacts.");
        return;
      }
      patchContact.mutate(
        { email: editTarget.email, isSecurityContact: next.isSecurityContact },
        {
          onSuccess: () => {
            setEditTarget(null);
            showSuccess("Security contact updated");
          },
          onError: (err) => {
            showError(err?.message ?? "Failed to update user. Please try again.");
          },
        },
      );
    },
    [editTarget, patchContact, showSuccess, showError],
  );

  const isEffectiveLoading = isLoading || isFetching;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {/* Stat cards */}
      {/* <Grid container spacing={2}>
        <Grid size={{ xs: 12, sm: 4 }}>
          <Card sx={{ p: 2, height: "100%" }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 1 }}>
              <Box
                sx={{
                  width: 40,
                  height: 40,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  color: colors.purple?.[600] ?? theme.palette.secondary.main,
                }}
              >
                {isEffectiveLoading ? (
                  <Skeleton variant="circular" width={20} height={20} />
                ) : (
                  <Crown size={20} />
                )}
              </Box> */}
              {/* <Box> */}
                {/* <Typography variant="h4">
                  {isEffectiveLoading ? (
                    <Skeleton variant="text" width={24} height={32} />
                  ) : error ? (
                    <ErrorIndicator entityName="admins" size="small" />
                  ) : (
                    stats.admins
                  )}
                </Typography> */}
                {/* <Typography variant="caption" color="text.secondary">
                  {isEffectiveLoading ? (
                    <Skeleton variant="text" width={48} height={16} />
                  ) : (
                    "Admins"
                  )}
                </Typography> */}
              {/* </Box> */}
            {/* </Box>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, sm: 4 }}>
          <Card sx={{ p: 2, height: "100%" }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 1 }}>
              <Box
                sx={{
                  width: 40,
                  height: 40,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  color: "info.main",
                }}
              >
                {isEffectiveLoading ? (
                  <Skeleton variant="circular" width={20} height={20} />
                ) : (
                  <Code size={20} />
                )}
              </Box>
              <Box>
                <Typography variant="h4">
                  {isEffectiveLoading ? (
                    <Skeleton variant="text" width={24} height={32} />
                  ) : error ? (
                    <ErrorIndicator entityName="developers" size="small" />
                  ) : (
                    stats.developers
                  )}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {isEffectiveLoading ? (
                    <Skeleton variant="text" width={64} height={16} />
                  ) : (
                    "Developers"
                  )}
                </Typography>
              </Box>
            </Box>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, sm: 4 }}>
          <Card sx={{ p: 2, height: "100%" }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 1 }}>
              <Box
                sx={{
                  width: 40,
                  height: 40,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  color: "error.main",
                }}
              >
                {isEffectiveLoading ? (
                  <Skeleton variant="circular" width={20} height={20} />
                ) : (
                  <ShieldCheck size={20} />
                )}
              </Box>
              <Box>
                <Typography variant="h4">
                  {isEffectiveLoading ? (
                    <Skeleton variant="text" width={24} height={32} />
                  ) : error ? (
                    <ErrorIndicator entityName="security" size="small" />
                  ) : (
                    stats.security
                  )}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {isEffectiveLoading ? (
                    <Skeleton variant="text" width={56} height={16} />
                  ) : (
                    "Security"
                  )}
                </Typography>
              </Box>
            </Box>
          </Card>
        </Grid>
      </Grid> */}

      {/* Search and Add User */}
      <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5 }}>
        <TextField
          fullWidth
          size="small"
          placeholder="Search users by name, email, or role..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <Search size={18} color={theme.palette.text.secondary} />
              </InputAdornment>
            ),
          }}
        />
        {canAddOrRemoveUsers && (
          <Button
            variant="contained"
            color="warning"
            startIcon={<Plus size={18} />}
            onClick={() => setIsAddModalOpen(true)}
            sx={{ whiteSpace: "nowrap" }}
          >
            Add User
          </Button>
        )}
      </Box>

      {/* Users table */}
      <TableContainer component={Paper} variant="outlined">
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>User</TableCell>
              <TableCell>Role</TableCell>
              <TableCell>Status</TableCell>
              {canAddOrRemoveUsers && (
                <TableCell align="right">Actions</TableCell>
              )}
            </TableRow>
          </TableHead>
          <TableBody>
            {isEffectiveLoading ? (
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                      <Skeleton variant="circular" width={40} height={40} />
                      <Box>
                        <Skeleton variant="text" width={100} height={20} />
                        <Skeleton variant="text" width={140} height={16} />
                      </Box>
                    </Box>
                  </TableCell>
                  <TableCell><Skeleton variant="rounded" width={70} height={24} /></TableCell>
                  <TableCell><Skeleton variant="rounded" width={60} height={24} /></TableCell>
                  {canAddOrRemoveUsers && (
                    <TableCell>
                      <Box sx={{ display: "flex", gap: 0.5, justifyContent: "flex-end" }}>
                        <Skeleton variant="circular" width={32} height={32} />
                      </Box>
                    </TableCell>
                  )}
                </TableRow>
              ))
            ) : error ? (
              <TableRow>
                <TableCell
                  colSpan={canAddOrRemoveUsers ? 4 : 3}
                  align="center"
                  sx={{ py: 3 }}
                >
                  <ErrorIndicator entityName="users" size="medium" />
                </TableCell>
              </TableRow>
            ) : filteredContacts.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={canAddOrRemoveUsers ? 4 : 3}
                  align="center"
                  sx={{ py: 3 }}
                >
                  <Typography variant="body2" color="text.secondary">
                    No users found.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              filteredContacts.map((contact) => (
                <TableRow key={contact.id} hover>
                  <TableCell>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                      <Box
                        sx={{
                          width: 40,
                          height: 40,
                          borderRadius: "50%",
                          bgcolor: getAvatarColor(contact.id ?? contact.email ?? ""),
                          color: "white",
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                          typography: "body2",
                          fontWeight: 600,
                        }}
                      >
                        {getInitials(contact.firstName, contact.lastName, contact.email)}
                      </Box>
                      <Box>
                        <Typography variant="body2">
                          {contact.firstName && contact.lastName
                            ? `${contact.firstName} ${contact.lastName}`
                            : contact.firstName || contact.lastName || NULL_PLACEHOLDER}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          {contact.email ?? NULL_PLACEHOLDER}
                        </Typography>
                      </Box>
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5 }}>
                      {getRoleBadges(contact).map((badge) => (
                        <Chip
                          key={badge.label}
                          size="small"
                          icon={<badge.Icon size={12} />}
                          label={badge.label}
                          variant="outlined"
                          color={badge.chipColor}
                          sx={getRoleChipSx(badge.chipColor)}
                        />
                      ))}
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Chip
                      size="small"
                      label={contact.membershipStatus ?? NULL_PLACEHOLDER}
                      variant="outlined"
                      color={getUserStatusColor(contact.membershipStatus ?? "")}
                      sx={{ typography: "caption" }}
                    />
                  </TableCell>
                  {canAddOrRemoveUsers && (
                    <TableCell align="right">
                      <Box sx={{ display: "flex", gap: 0.5, justifyContent: "flex-end" }}>
                        {!contact.isCsIntegrationUser && (
                          <Tooltip title="Edit user">
                            <span>
                              <IconButton
                                size="small"
                                aria-label="Edit user"
                                onClick={() => setEditTarget(contact)}
                              >
                                <PencilLine size={16} />
                              </IconButton>
                            </span>
                          </Tooltip>
                        )}
                        <Tooltip title="Remove user">
                          <span>
                            <IconButton
                              size="small"
                              color="error"
                              aria-label="Remove user"
                              onClick={() => setRemoveTarget(contact)}
                            >
                              <Trash2 size={16} />
                            </IconButton>
                          </span>
                        </Tooltip>
                      </Box>
                    </TableCell>
                  )}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Role Permissions */}
      <Paper
        sx={{
          p: 2.5,
          bgcolor: alpha(theme.palette.info.main, 0.06),
          border: "1px solid",
          borderColor: alpha(theme.palette.info.main, 0.2),
        }}
      >
        <Typography variant="h6" sx={{ mb: 4, display: "flex", alignItems: "center", gap: 1 }}>
          <Shield size={20} color={theme.palette.text.primary} />
          Role Permissions
        </Typography>
        <Grid container spacing={2}>
          {ROLE_CONFIG.map((role) => {
            const RoleIcon = role.Icon;
            const roleColor =
              role.paletteKey === "secondary"
                ? (colors.purple?.[600] ?? theme.palette.primary.main)
                : (theme.palette[role.paletteKey]?.main ?? theme.palette.text.primary);
            return (
              <Grid key={role.id} size={{ xs: 12, sm: 6, md: 3 }}>
                <Box>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 2 }}>
                    <RoleIcon size={18} color={roleColor} />
                    <Typography variant="subtitle2">{role.label}</Typography>
                  </Box>
                  <Box component="ul" sx={{ m: 0, pl: 2.5, listStyle: "disc", "& li": { mb: 0.5 } }}>
                    {role.permissions.map((p) => (
                      <li key={p}>
                        <Typography variant="caption" color="text.secondary" component="span">
                          {p}
                        </Typography>
                      </li>
                    ))}
                  </Box>
                </Box>
              </Grid>
            );
          })}
        </Grid>
      </Paper>

      <AddUserModal
        open={isAddModalOpen}
        projectId={projectId}
        onClose={() => setIsAddModalOpen(false)}
        onSubmit={handleAddUser}
        isSubmitting={postContact.isPending}
      />

      <EditUserModal
        open={editTarget !== null}
        contact={editTarget}
        isSubmitting={patchContact.isPending}
        onClose={() => setEditTarget(null)}
        onSubmit={handleEditUser}
      />

      <RemoveUserModal
        open={removeTarget !== null}
        contact={removeTarget}
        isDeleting={deleteContact.isPending}
        onClose={() => setRemoveTarget(null)}
        onConfirm={handleRemoveUser}
      />
    </Box>
  );
}
