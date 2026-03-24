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

import { Link, useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { Card, Grid, Stack, Typography, Button, Divider, useTheme, SearchBar } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { MetricWidget } from "@components/features/dashboard";
import { UserListItem, UserListItemSkeleton } from "@components/features/users";
import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { users } from "@src/services/users";
import { useProject } from "@context/project";
import { ErrorBoundary } from "@components/core";
import { Suspense } from "react";

import { MOCK_ROLES } from "@src/mocks/data/users";
import EmptyState from "../components/shared/EmptyState";

export default function UsersPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const location = useLocation();
  const { projectId } = useProject();

  const baseRoute = location.pathname;
  const searchValue = searchParams.get("search") ?? "";

  const updateParams = (updates: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams);

    Object.entries(updates).forEach(([key, value]) => {
      if (!value) {
        next.delete(key);
      } else {
        next.set(key, value);
      }
    });

    navigate(`${baseRoute}?${next.toString()}`, { replace: true });
  };

  const { data } = useQuery(users.all(projectId!));
  const total = data?.length;
  const registered = data?.filter((user) => user.status === "registered").length;
  const invited = data?.filter((user) => user.status === "invited").length;
  const admins = data?.filter((user) => user.roles.includes("Admin")).length;

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
            value={searchValue}
            onChange={(e) => updateParams({ ["search"]: e.target.value || null })}
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
          <ErrorBoundary fallback={<UsersListContentSkeleton />}>
            <Suspense fallback={<UsersListContentSkeleton />}>
              <UsersListContent />
            </Suspense>
          </ErrorBoundary>
        </Stack>
      </Card>
      <UserRolesInfo />
    </>
  );
}

function UsersListContent() {
  const { projectId } = useProject();
  const { data } = useSuspenseQuery(users.all(projectId!));

  if (data.length === 0) return <EmptyState title="No users found" />;

  return (
    <>
      {data.map((props) => (
        <UserListItem key={props.id} {...props} />
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
      <Stack gap={0.3} mt={0.5}>
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
