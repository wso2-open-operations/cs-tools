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

import { useState, type ReactNode } from "react";
import {
  alpha,
  Avatar,
  Box,
  Button,
  Card,
  CircularProgress,
  pxToRem,
  Stack,
  TextField,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { InvitationSummaryContent, RoleSelector } from "@components/features/users";
import { useProject } from "@context/project";
import { Clock4, Info, Mail, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { stringAvatar } from "@utils/others";
import { getApiErrorMessage } from "@utils/ApiError";
import type { Role } from "@src/types";
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { users } from "../services/users";
import { projects } from "../services/projects";
import { useNotify } from "../context/snackbar";
import { ConfirmDialog } from "../components/shared/ConfirmDialog";

type UserActionFeedbackState = {
  action: "invite" | "edit" | "delete";
  email: string;
  firstName?: string;
  lastName?: string;
  roles?: Role[];
};

export default function EditUserPage({ mode = "invite" }: { mode?: "invite" | "edit" }) {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const state = location.state as {
    email?: string;
    role?: Role;
    roles?: Role[];
    firstName?: string;
    lastName?: string;
  };

  const defaultUserRole: Role[] = ["Portal User"];
  const initialRoles: Role[] =
    state?.roles && state.roles.length > 0
      ? state.roles
      : state?.role && state.role !== "Admin User"
        ? [state.role]
        : defaultUserRole;
  const isSystemUserReadOnly = mode === "edit" && initialRoles.includes("System User");
  const [roles, setRoles] = useState<Role[]>(initialRoles);
  const [email, setEmail] = useState(state?.email ?? "");
  const [firstName, setFirstName] = useState(state?.firstName ?? "");
  const [lastName, setLastName] = useState(state?.lastName ?? "");
  const rolesUnchanged =
    roles.length === initialRoles.length && roles.every((selectedRole) => initialRoles.includes(selectedRole));
  const hasAnyRole = roles.length > 0;

  const { projectId } = useProject();
  const project = useSuspenseQuery(projects.all()).data.find((project) => project.id === projectId);
  const notify = useNotify();
  const inviteFallbackMessage = "Failed to invite user. Please try again.";
  const editFallbackMessage = "Failed to edit user. Please try again.";
  const deleteFallbackMessage = "Failed to delete user. Please try again.";

  const createUserMutation = useMutation({
    ...users.create(projectId!),
    onSuccess: () => {
      queryClient.resetQueries({ queryKey: ["users", projectId] });
      navigate("/users", {
        state: {
          action: "invite",
          email,
          firstName,
          lastName,
          roles,
        } satisfies UserActionFeedbackState,
      });
    },
    onError: (error) => notify.error(getApiErrorMessage(error) ?? inviteFallbackMessage),
  });

  const editUserMutation = useMutation({
    ...users.edit(projectId!, email),
    onSuccess: () => {
      queryClient.resetQueries({ queryKey: ["users", projectId] });
      navigate("/users", {
        state: {
          action: "edit",
          email,
          firstName,
          lastName,
          roles,
        } satisfies UserActionFeedbackState,
      });
    },
    onError: (error) => notify.error(getApiErrorMessage(error) ?? editFallbackMessage),
  });

  const deleteUserMutation = useMutation({
    ...users.delete(projectId!, email),
    onSuccess: () => {
      queryClient.resetQueries({ queryKey: ["users", projectId] });
      navigate("/users", {
        state: {
          action: "delete",
          email,
          firstName,
          lastName,
        } satisfies UserActionFeedbackState,
      });
    },
    onError: (error) => notify.error(getApiErrorMessage(error) ?? deleteFallbackMessage),
  });

  const validateUserMutation = useMutation({
    ...users.validate(projectId!),
    onError: (error) => notify.error(getApiErrorMessage(error) ?? "Email validation failed. Please try again."),
  });

  const handleSubmit = async () => {
    try {
      if (mode === "invite") {
        const validationResponse = await validateUserMutation.mutateAsync({
          contactEmail: email,
        });

        if (!validationResponse.isContactValid) {
          notify.error(validationResponse.message || "This email cannot be added.");
          return;
        }

        await createUserMutation.mutateAsync({
          contactEmail: email,
          contactFirstName: firstName,
          contactLastName: lastName,
          isCsIntegrationUser: roles.includes("System User"),
          isCsAdmin: roles.includes("Admin User"),
          isPortalUser: roles.includes("Portal User"),
          isSecurityContact: roles.includes("Security User"),
        });
        return;
      }

      await editUserMutation.mutateAsync({
        isCsAdmin: roles.includes("Admin User"),
        isPortalUser: roles.includes("Portal User"),
        isSecurityContact: roles.includes("Security User"),
      });
    } catch {
      // Mutation onError handlers surface user-facing messages.
    }
  };

  return (
    <>
      <Stack gap={2}>
        {mode === "edit" && <UserSummaryCard firstName={firstName} lastName={lastName} email={email} />}
        {mode === "invite" && <InvitationNotice />}
        <SectionCard title="User Details">
          <Stack gap={2}>
            <TextField
              size="small"
              label="Email Address"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              helperText={mode === "edit" ? "Email cannot be edited" : undefined}
              slotProps={{
                htmlInput: { readOnly: mode === "edit" },
              }}
            />

            <TextField
              size="small"
              label="First Name"
              value={firstName}
              onChange={(event) => setFirstName(event.target.value)}
              helperText={mode === "edit" ? "First Name cannot be edited" : undefined}
              slotProps={{
                htmlInput: { readOnly: mode === "edit" },
              }}
            />

            <TextField
              size="small"
              label="Last Name"
              value={lastName}
              onChange={(event) => setLastName(event.target.value)}
              helperText={mode === "edit" ? "Last Name cannot be edited" : undefined}
              slotProps={{
                htmlInput: { readOnly: mode === "edit" },
              }}
            />
          </Stack>
        </SectionCard>

        <SectionCard title="User Role">
          {isSystemUserReadOnly && (
            <Box
              sx={(theme) => ({
                px: 1.5,
                py: 1,
                bgcolor: alpha(theme.palette.info.main, 0.15),
              })}
            >
              <Typography variant="caption" color="info.dark" fontWeight="medium">
                This contact is a CS Integration User. CS Integration Users cannot be assigned other roles.
              </Typography>
            </Box>
          )}
          <RoleSelector value={roles} onChange={setRoles} readOnly={isSystemUserReadOnly} />
        </SectionCard>

        {mode === "invite" && (
          <>
            <SectionCard title="Invitation Summary">
              <InvitationSummaryContent
                projectName={project?.name}
                email={email}
                name={firstName + " " + lastName}
                roles={roles}
              />
            </SectionCard>
            <ExpirationNotice />
          </>
        )}

        {mode === "edit" && (
          <>
            <DangerZone onDelete={deleteUserMutation.mutate} isPending={deleteUserMutation.isPending} />
          </>
        )}

        <Button
          disabled={
            mode === "edit"
              ? isSystemUserReadOnly || rolesUnchanged || !hasAnyRole || editUserMutation.isPending
              : !hasAnyRole ||
                !email.trim() ||
                !firstName.trim() ||
                createUserMutation.isPending ||
                validateUserMutation.isPending
          }
          variant="contained"
          startIcon={
            createUserMutation.isPending || editUserMutation.isPending || validateUserMutation.isPending ? (
              <CircularProgress size={16} color="inherit" />
            ) : undefined
          }
          onClick={() => void handleSubmit()}
        >
          {mode === "invite"
            ? createUserMutation.isPending || validateUserMutation.isPending
              ? "Sending..."
              : "Send Invitation"
            : editUserMutation.isPending
              ? "Saving..."
              : "Save Changes"}
        </Button>

        <Button
          variant="outlined"
          component={Link}
          sx={{ textTransform: "initial", bgcolor: "background.paper" }}
          to="/users"
        >
          Cancel
        </Button>
      </Stack>
    </>
  );
}

function SectionCard({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card component={Stack} gap={2} p={2}>
      <Typography variant="body2" color="text.secondary" fontWeight="medium">
        {title}
      </Typography>
      {children}
    </Card>
  );
}

function InvitationNotice() {
  const theme = useTheme();

  return (
    <Card
      component={Stack}
      direction="row"
      sx={(theme) => ({ bgcolor: alpha(theme.palette.info.main, 0.2), p: 1.5, gap: 2 })}
    >
      <Box color={theme.palette.info.main}>
        <Info size={pxToRem(18)} />
      </Box>
      <Stack>
        <Typography variant="body2" fontWeight="medium" color="info">
          Direct User Invitation
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
          Send an email invitation directly to a user to join this project. The invitation link will be valid for 7
          days.
        </Typography>
      </Stack>
    </Card>
  );
}

function ExpirationNotice() {
  const theme = useTheme();

  return (
    <Card
      component={Stack}
      direction="row"
      alignItems="center"
      px={2}
      py={1.5}
      gap={2}
      sx={{ bgcolor: "components.popover.state.active.background" }}
    >
      <Clock4 size={pxToRem(50)} color={theme.palette.primary.main} />
      <Typography variant="subtitle2" fontWeight="medium" color="text.secondary">
        Important: &nbsp;
        <Typography component="span" variant="subtitle2" fontWeight="regular">
          Invitation links expire after 7 days. If the user doesn't accept the invitation within this timeframe, you'll
          need to send a new invitation.
        </Typography>
      </Typography>
    </Card>
  );
}

function UserSummaryCard({ firstName, lastName, email }: { firstName: string; lastName: string; email: string }) {
  const theme = useTheme();

  return (
    <Card component={Stack} textAlign="center" alignItems="center" gap={2} p={3}>
      <Avatar
        sx={(theme) => ({
          width: 65,
          height: 65,
          bgcolor: "primary.main",
          fontSize: theme.typography.h3,
          fontWeight: "medium",
        })}
      >
        {stringAvatar(firstName)}
      </Avatar>
      <Stack textAlign="center" gap={0.5}>
        <Typography variant="h5" fontWeight="medium">
          {firstName + " " + lastName}
        </Typography>
        <Stack direction="row" justifyContent="center" alignItems="center" gap={1}>
          <Mail size={pxToRem(16)} color={theme.palette.text.secondary} />
          <Typography variant="body2" fontWeight="regular" color="text.secondary">
            {email}
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}

function DangerZone({ isPending, onDelete }: { isPending: boolean; onDelete: () => void }) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Card component={Stack} sx={(theme) => ({ bgcolor: alpha(theme.palette.error.main, 0.2), p: 1.5 })}>
        <Typography variant="body2" fontWeight="medium" color="error">
          Danger Zone
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
          Send an email invitation directly to a user to join this project. The invitation link will be valid for 7
          days.
        </Typography>
        <Button
          variant="contained"
          color="error"
          disabled={isPending}
          startIcon={isPending ? <CircularProgress size={16} color="inherit" /> : <Trash2 />}
          sx={{ mt: 3 }}
          onClick={() => setOpen(true)}
        >
          {isPending ? "Removing..." : "Remove User from Project"}
        </Button>
      </Card>

      <ConfirmDialog
        open={open}
        title="Remove User"
        description="Are you sure you want to remove this user?"
        confirmColor="error"
        confirmLabel="Remove"
        onClose={() => setOpen(false)}
        onConfirm={() => {
          setOpen(false);
          onDelete();
        }}
      />
    </>
  );
}
