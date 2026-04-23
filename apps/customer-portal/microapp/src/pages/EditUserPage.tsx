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
  Avatar,
  Box,
  Button,
  Card,
  Chip,
  colors,
  FormControlLabel,
  pxToRem,
  Radio,
  RadioGroup,
  Stack,
  TextField,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import { Link, useLocation } from "react-router-dom";
import { InvitationSummaryContent, RoleSelector, type RoleName } from "@components/features/users";
import { useProject } from "@context/project";

import { MOCK_PROJECTS } from "@src/mocks/data/projects";
import { Clock4, Info, Mail, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { stringAvatar } from "@utils/others";

export default function EditUserPage({ mode = "invite" }: { mode?: "invite" | "edit" }) {
  const location = useLocation();
  const state = location.state as { email?: string; role?: RoleName; name?: string };

  const [role, setRole] = useState<RoleName>(state?.role ?? "Admin");
  const [email, setEmail] = useState(state?.email ?? "");
  const [name, setName] = useState(state?.name ?? "");
  const [userStatus, setUserStatus] = useState("active");

  const { projectId } = useProject();
  const project = MOCK_PROJECTS.find((project) => project.id === projectId);

  return (
    <Stack gap={2}>
      {/* TODO: Replace hardcoded `lastActive` value with backend-provided data once user activity tracking is integrated. */}
      {mode === "edit" && <UserSummaryCard name={name} email={email} lastActive="2 hours ago" />}
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
            label="Full Name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            helperText={mode === "edit" ? "Name cannot be edited" : undefined}
            slotProps={{
              htmlInput: { readOnly: mode === "edit" },
            }}
          />
        </Stack>
      </SectionCard>

      <SectionCard title="User Role">
        <RoleSelector value={role} onChange={setRole} />
      </SectionCard>

      {mode === "edit" && (
        <SectionCard title="User Status">
          <UserStatusSelector value={userStatus} onChange={setUserStatus} />
        </SectionCard>
      )}

      {mode === "invite" && (
        <>
          <SectionCard title="Invitation Summary">
            <InvitationSummaryContent projectName={project?.name} email={email} name={name} role={role} />
          </SectionCard>
          <ExpirationNotice />
        </>
      )}

      {mode === "edit" && (
        <>
          <PermissionDetails />
          <DangerZone />
        </>
      )}

      {/* TODO: Implement proper submission handling */}
      <Button variant="contained" component={Link} to="/users" sx={{ textTransform: "initial" }}>
        {mode === "invite" ? "Send Invitation" : "Save Changes"}
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
  return (
    <Card component={Stack} direction="row" gap={2} sx={{ bgcolor: colors.blue[50], p: 1.5 }}>
      <Box color={colors.indigo[500]}>
        <Info size={pxToRem(18)} />
      </Box>
      <Stack>
        <Typography variant="body2" fontWeight="medium" color={colors.indigo[500]}>
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

function PermissionDetails() {
  return (
    <Card component={Stack} sx={{ bgcolor: colors.blue[50], p: 1.5 }}>
      <Typography variant="body2" fontWeight="medium" color={colors.indigo[500]}>
        Permission Details
      </Typography>
      <ul style={{ margin: 0, marginTop: 3, paddingLeft: 20 }}>
        <li>
          <Typography variant="subtitle2" color="text.secondary">
            Create and manage own cases
          </Typography>
        </li>
        <li>
          <Typography variant="subtitle2" color="text.secondary">
            Participate in chats
          </Typography>
        </li>
        <li>
          <Typography variant="subtitle2" color="text.secondary">
            Submit service requests
          </Typography>
        </li>
        <li>
          <Typography variant="subtitle2" color="text.secondary">
            View project analytics
          </Typography>
        </li>
      </ul>
    </Card>
  );
}

function UserSummaryCard({ name, email, lastActive }: { name: string; email: string; lastActive: string }) {
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
        {stringAvatar(name)}
      </Avatar>
      <Stack textAlign="center" gap={0.5}>
        <Typography variant="h5" fontWeight="medium">
          {name}
        </Typography>
        <Stack direction="row" justifyContent="center" alignItems="center" gap={1}>
          <Mail size={pxToRem(16)} color={theme.palette.text.secondary} />
          <Typography variant="body2" fontWeight="regular" color="text.secondary">
            {email}
          </Typography>
        </Stack>
        <Typography variant="caption" fontWeight="regular" color="text.secondary">
          Last Active: {lastActive}
        </Typography>
      </Stack>
    </Card>
  );
}

function DangerZone() {
  return (
    <Card component={Stack} sx={{ bgcolor: colors.red[50], p: 1.5 }}>
      <Typography variant="body2" fontWeight="medium" color="error">
        Danger Zone
      </Typography>
      <Typography variant="subtitle2" color="text.secondary">
        Send an email invitation directly to a user to join this project. The invitation link will be valid for 7 days.
      </Typography>
      <Button variant="contained" color="error" startIcon={<Trash2 />} sx={{ mt: 3 }}>
        Remove User from Project
      </Button>
    </Card>
  );
}

function UserStatusSelector({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <RadioGroup value={value} onChange={(event) => onChange(event.target.value)}>
      <FormControlLabel
        value="active"
        control={<Radio />}
        labelPlacement="start"
        sx={{
          m: 0,
          justifyContent: "space-between",
        }}
        label={
          <Stack direction="row" alignItems="center" gap={1}>
            <Chip size="small" label="Active" color={value === "active" ? "primary" : "default"} />
          </Stack>
        }
      ></FormControlLabel>
      <FormControlLabel
        value="inactive"
        control={<Radio />}
        labelPlacement="start"
        sx={{
          m: 0,
          justifyContent: "space-between",
        }}
        label={
          <Stack direction="row" alignItems="center" gap={1}>
            <Chip size="small" label="Inactive" color={value === "inactive" ? "primary" : "default"} />
          </Stack>
        }
      ></FormControlLabel>
    </RadioGroup>
  );
}
