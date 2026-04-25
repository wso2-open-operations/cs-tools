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

import { Link, useLocation, useNavigate } from "react-router-dom";
import { Card, Grid, Stack, Typography, Button, Divider, useTheme, SearchBar } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { MetricWidget } from "@components/features/dashboard";
import { UserListItem, UserListItemSkeleton } from "@components/features/users";
import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { users } from "@src/services/users";
import { useProject } from "@context/project";
import { ErrorBoundary } from "@components/core";
import { Suspense, useEffect, useMemo, useState } from "react";

import { MOCK_ROLES } from "@src/mocks/data/users";
import type { Role } from "@src/types";
import EmptyState from "../components/shared/EmptyState";
import { useNotify } from "../context/snackbar";

type UserActionFeedbackState = {
  action?: "invite" | "edit" | "delete";
  email?: string;
  firstName?: string;
  lastName?: string;
  roles?: Role[];
};

export default function UsersPage() {
  const notify = useNotify();
  const location = useLocation();
  const navigate = useNavigate();
  const { projectId } = useProject();
  const { data } = useQuery(users.all(projectId!));
  const [highlightedUserEmail, setHighlightedUserEmail] = useState<string>();

  const total = data?.length;
  const registered = data?.filter((user) => user.status === "registered").length;
  const invited = data?.filter((user) => user.status === "invited").length;
  const admins = data?.filter((user) => user.roles.includes("Admin User")).length;

  const [search, setSearch] = useState("");

  useEffect(() => {
    const feedback = (location.state ?? {}) as UserActionFeedbackState;
    if (!feedback.action || !feedback.email) return;

    const userDisplayName =
      feedback.firstName && feedback.lastName ? `${feedback.firstName} ${feedback.lastName}` : feedback.email;

    if (feedback.action === "invite") {
      notify.success(`Invitation sent to ${feedback.email}.`);
    } else if (feedback.action === "edit") {
      const rolesMessage = feedback.roles?.length ? ` Roles: ${feedback.roles.join(", ")}.` : "";
      notify.success(`Updated ${userDisplayName}.${rolesMessage}`);
    } else if (feedback.action === "delete") {
      notify.success(`Removed ${userDisplayName} from project.`);
    }

    setHighlightedUserEmail(feedback.email);
    const timeoutId = window.setTimeout(() => setHighlightedUserEmail(undefined), 4000);

    navigate(location.pathname, { replace: true, state: null });
    return () => window.clearTimeout(timeoutId);
  }, [location.pathname, location.state, navigate, notify]);

  return (
    <>
      <Grid spacing={1.5} container>
        <Grid size={3}>
          <MetricWidget base size="small" label="Total" value={total} />
        </Grid>
        <Grid size={3}>
          <MetricWidget base size="small" label="Registered" value={registered} />
        </Grid>
        <Grid size={3}>
          <MetricWidget base size="small" label="Invited" value={invited} />
        </Grid>
        <Grid size={3}>
          <MetricWidget base size="small" label="Admins" value={admins} />
        </Grid>
        <Grid size={12}>
          <SearchBar
            size="small"
            placeholder="Search Users"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            sx={{ mt: 1, bgcolor: "background.paper" }}
            fullWidth
          />
        </Grid>
      </Grid>
      <Card component={Stack} p={2} mt={2} gap={0.5}>
        <Stack direction="row" justifyContent="space-between" pb={1}>
          <Typography variant="h6">All Users</Typography>
          <AddButton />
        </Stack>
        <Divider />
        <Stack gap={2} pt={1}>
          <ErrorBoundary
            fallback={<UsersListContentSkeleton />}
            onError={() => notify.error("Failed to load users. Try again later.")}
          >
            <Suspense fallback={<UsersListContentSkeleton />}>
              <UsersListContent search={search} highlightedUserEmail={highlightedUserEmail} />
            </Suspense>
          </ErrorBoundary>
        </Stack>
      </Card>
      <UserRolesInfo />
    </>
  );
}

function UsersListContent({ search, highlightedUserEmail }: { search: string; highlightedUserEmail?: string }) {
  const { projectId } = useProject();
  const { data: usersData } = useSuspenseQuery(users.all(projectId!));

  const data = useMemo(() => {
    if (!search) return usersData;

    const normalizedSearch = search.toLowerCase();

    return usersData.filter(
      (user) =>
        user.firstName.toLowerCase().includes(normalizedSearch) ||
        user.lastName.toLowerCase().includes(normalizedSearch),
    );
  }, [usersData, search]);

  if (data.length === 0) return <EmptyState title="No users found" />;

  return (
    <>
      {data.map((props) => (
        <UserListItem key={props.id} isHighlighted={props.email === highlightedUserEmail} {...props} />
      ))}
    </>
  );
}

function UsersListContentSkeleton() {
  return (
    <>
      {Array.from({ length: 10 }).map((_, index) => (
        <UserListItemSkeleton key={index} />
      ))}
    </>
  );
}

function AddButton() {
  const theme = useTheme();

  return (
    <Button component={Link} to="/users/invite" sx={{ textTransform: "initial" }}>
      <Stack direction="row" gap={1}>
        <Typography variant="body1" color="primary" fontWeight="medium">
          Add
        </Typography>
        <Plus color={theme.palette.primary.main} />
      </Stack>
    </Button>
  );
}

function UserRolesInfo() {
  return (
    <Card component={Stack} p={2} mt={2}>
      <Typography variant="subtitle1" fontWeight="medium">
        User Roles
      </Typography>
      <Stack gap={1} mt={0.5}>
        {MOCK_ROLES.map((role) => (
          <Typography variant="subtitle2" fontWeight="medium">
            {role.name}:&nbsp;
            <Typography component="span" variant="subtitle2" fontWeight="regular">
              {role.description}
            </Typography>
          </Typography>
        ))}
      </Stack>
    </Card>
  );
}
