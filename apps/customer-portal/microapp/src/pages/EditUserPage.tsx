import { useState, type ReactNode } from "react";
import { ButtonBase as Button, Card, InputAdornment, Stack, Typography } from "@mui/material";
import { TextField } from "@components/features/create";
import { AccessTime, Email, Person } from "@mui/icons-material";
import { Link, useLocation } from "react-router-dom";
import { InvitationSummaryContent, RoleSelector, type RoleName } from "@components/features/users";
import { useProject } from "@context/project";

import { MOCK_PROJECTS } from "@src/mocks/data/projects";

export default function EditUserPage() {
  const location = useLocation();
  const state = location.state as { email?: string; role?: string; name?: string };

  const [role, setRole] = useState<RoleName>(state?.role ?? "Admin");
  const [email, setEmail] = useState(state?.email ?? "");
  const [name, setName] = useState(state?.name ?? "");

  const { projectId } = useProject();
  const project = MOCK_PROJECTS.find((project) => project.id === projectId);

  return (
    <Stack gap={2}>
      <SectionCard title="User Details">
        <Stack gap={2}>
          <TextField
            label="Email Address"
            placeholder="user@example.com"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            startAdornment={
              <InputAdornment position="start">
                <Email />
              </InputAdornment>
            }
          />
          <TextField
            label="Full Name"
            placeholder="John Doe"
            value={name}
            onChange={(event) => setName(event.target.value)}
            startAdornment={
              <InputAdornment position="start">
                <Person />
              </InputAdornment>
            }
          />
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
    <Card component={Stack} gap={2} p={2} elevation={0}>
      <Typography variant="body2" color="text.secondary" fontWeight="medium">
        {title}
      </Typography>
      {children}
    </Card>
  );
}

function SendButton() {
  return (
    <Button component={Link} variant="contained" sx={{ fontWeight: "medium" }} to="/users">
      Send Invitation
    </Button>
  );
}

function CancelButton() {
  return (
    <Button component={Link} variant="outlined" sx={{ fontWeight: "medium", bgcolor: "background.paper" }} to="/users">
      Cancel
    </Button>
  );
}

function ExpirationNotice() {
  return (
    <Card
      component={Stack}
      direction="row"
      alignItems="top"
      px={2}
      py={1.5}
      gap={2}
      elevation={0}
      sx={{ bgcolor: "components.popover.state.active.background" }}
    >
      <AccessTime color="primary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(20) })} />
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
