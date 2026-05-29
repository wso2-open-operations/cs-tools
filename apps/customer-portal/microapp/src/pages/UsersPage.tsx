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
import { Link } from "react-router-dom";

import { Button, Card, Divider, Grid, SearchBar, Stack, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";

import { useDeclareLayout } from "@context/layout";
import { useNotify } from "@context/snackbar";

import { WidgetMetric } from "@features/dashboard/components";
import { UsersList, UsersListSkeleton } from "@features/users/components";
import { useFilters, useUserStats } from "@features/users/hooks";

import { ErrorBoundary } from "@shared/components/core";

import { ROUTES, Tab } from "@shared/constants";

export default function UsersPage() {
  useDeclareLayout({
    tabIndex: Tab.Users,
    visibility: {
      exitButton: true,
      projectSelector: true,
    },
  });

  const notify = useNotify();
  const { total, registered, invited, admins } = useUserStats();
  const { set } = useFilters();

  return (
    <>
      <Grid spacing={1.5} container>
        <Grid size={3}>
          <WidgetMetric label="Total" value={total} />
        </Grid>
        <Grid size={3}>
          <WidgetMetric label="Registered" value={registered} />
        </Grid>
        <Grid size={3}>
          <WidgetMetric label="Invited" value={invited} />
        </Grid>
        <Grid size={3}>
          <WidgetMetric label="Admins" value={admins} />
        </Grid>

        <SearchBar
          fullWidth
          size="small"
          placeholder="Search Users"
          onChange={(e) => set({ search: e.target.value })}
        />
      </Grid>

      <Card component={Stack} p={2} mt={2} gap={0.5} divider={<Divider />}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" pb={1}>
          <Typography variant="h6">All Users</Typography>
          <Button component={Link} to={ROUTES.users.invite} startIcon={<Plus />} sx={{ textTransform: "none" }}>
            Add
          </Button>
        </Stack>

        <Stack gap={2} pt={1}>
          <ErrorBoundary
            fallback={<UsersListSkeleton />}
            onError={() => notify.error("Failed to load users. Try again later.")}
          >
            <UsersList />
          </ErrorBoundary>
        </Stack>
      </Card>
    </>
  );
}
