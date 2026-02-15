import { useState, type ReactNode } from "react";
import { Avatar, Button, Card, pxToRem, Stack, TextField, Typography, useTheme } from "@wso2/oxygen-ui";
import { Link, useLocation } from "react-router-dom";
import { InvitationSummaryContent, RoleSelector, type RoleName } from "@components/features/users";
import { useProject } from "@context/project";

import { MOCK_PROJECTS } from "@src/mocks/data/projects";
import { Clock4, Mail } from "@wso2/oxygen-ui-icons-react";
import { stringAvatar } from "../utils/others";

export default function EditUserPage({ mode = "invite" }: { mode?: "invite" | "edit" }) {
  const theme = useTheme();
  const location = useLocation();
  const state = location.state as { email?: string; role?: string; name?: string };

  const [role, setRole] = useState<RoleName>(state?.role ?? "Admin");
  const [email, setEmail] = useState(state?.email ?? "");
  const [name, setName] = useState(state?.name ?? "");

  const { projectId } = useProject();
  const project = MOCK_PROJECTS.find((project) => project.id === projectId);

  return (
    <Stack gap={2}>
      {mode === "edit" && (
        <Card component={Stack} textAlign="center" alignItems="center" gap={2} p={3}>
          <Avatar
            sx={(theme) => ({
              width: 65,
              height: 65,
              bgcolor: "primary.main",
              color: "primary.contrastText",
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
              <Mail size={pxToRem(18)} color={theme.palette.text.secondary} />
              <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                {email}
              </Typography>
            </Stack>
            <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
              Last Active: 2 hours ago
            </Typography>
          </Stack>
        </Card>
      )}
      <SectionCard title="User Details">
        <Stack gap={2}>
          <TextField
            size="small"
            label="Email Address"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
          <TextField size="small" label="Full Name" value={name} onChange={(event) => setName(event.target.value)} />
        </Stack>
      </SectionCard>

      <SectionCard title="User Role">
        <RoleSelector value={role} onChange={setRole} />
      </SectionCard>

      <SectionCard title="Invitation Summary">
        <InvitationSummaryContent projectName={project?.name} email={email} name={name} role={role} />
      </SectionCard>

      <ExpirationNotice />

      <SendButton />
      <CancelButton />
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

function SendButton() {
  return (
    <Button variant="contained" component={Link} to="/users" sx={{ textTransform: "initial" }}>
      Send Invitation
    </Button>
  );
}

function CancelButton() {
  return (
    <Button component={Link} sx={{ textTransform: "initial", bgcolor: "background.paper" }} to="/users">
      Cancel
    </Button>
  );
}

function ExpirationNotice() {
  const theme = useTheme();

  return (
    <Card
      component={Stack}
      direction="row"
      alignItems="top"
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
