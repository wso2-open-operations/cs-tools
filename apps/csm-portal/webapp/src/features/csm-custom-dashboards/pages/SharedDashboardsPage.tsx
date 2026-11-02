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

import { Box, Chip, Skeleton, Typography } from "@wso2/oxygen-ui";
import { Eye, UsersRound } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Navigate, useNavigate, useParams } from "react-router";
import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";
import {
  useGetSharedDashboard,
  useGetSharedDashboards,
  type SharedDashboardSummary,
} from "@features/csm-custom-dashboards/api/useDashboardShares";
import DashboardSwitcher from "@features/csm-custom-dashboards/components/DashboardSwitcher";

const BASE_PATH = "/dashboard/shared";

/**
 * One dashboard shared with the user (`GET /dashboards/shared/{id}`): the
 * owner's current version, drawn with Dashboard's own widget grid, as My
 * Dashboards draws a saved one. One that is no longer shared (or was deleted)
 * says so; one saved under rules that have since changed shows the backend's
 * message, which asks to have the owner update it.
 */
function SharedDashboardView({ id }: { id: string }): JSX.Element {
  const shared = useGetSharedDashboard(id);

  const frame = { border: 1, borderColor: "divider", borderRadius: 1, p: 2 };
  if (shared.isLoading) {
    return (
      <Box sx={frame}>
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }
  if (shared.isError || !shared.data) {
    return (
      <Box sx={frame}>
        <Typography variant="body2" color="text.secondary">
          {shared.isError
            ? shared.error.message || "Could not load this dashboard."
            : "This dashboard is no longer shared with you."}
        </Typography>
      </Box>
    );
  }
  return (
    <Box sx={frame}>
      <DashboardWidgetGrid widgets={shared.data.dashboard.widgets} />
    </Box>
  );
}

/**
 * Dashboards > Shared Dashboards (`/dashboard/shared`,
 * `/dashboard/shared/:dashboardId`): the dashboards other users shared with
 * the user (`GET /dashboards/shared`), view only: no edit, share or delete. A
 * switcher with search, as My Dashboards has, each entry naming who shared
 * it ("Shared by <owner>", or "Shared with your team <team> by <owner>" when
 * it reached the user through a team; "a former user" once the owner's user
 * has been deleted); the open one says the same and "View only". No id in the URL,
 * or one that is not in the list, opens the first one; while the list is
 * being re-read an unknown id waits for it, as on My Dashboards.
 */
export default function SharedDashboardsPage(): JSX.Element {
  const { dashboardId } = useParams<{ dashboardId?: string }>();
  const navigate = useNavigate();
  const list = useGetSharedDashboards();
  const dashboards = list.data ?? [];

  if (list.isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Typography variant="h5">Shared Dashboards</Typography>
        <Typography variant="body2" color="text.secondary">
          Could not load the dashboards shared with you.
        </Typography>
      </Box>
    );
  }

  const selected = dashboards.find((d) => d.id === dashboardId?.toLowerCase());

  if (list.isLoading || (dashboardId && !selected && list.isFetching)) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }

  if (dashboards.length === 0) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Typography variant="h5">Shared Dashboards</Typography>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 1,
            py: 8,
            textAlign: "center",
          }}
        >
          <UsersRound size={32} />
          <Typography variant="h6">No shared dashboards yet</Typography>
          <Typography variant="body2" color="text.secondary">
            Dashboards that others share with you will appear here. You can view them, not change them.
          </Typography>
        </Box>
      </Box>
    );
  }

  if (!selected) {
    // No id, or one that is not shared with the user: open the first one.
    return <Navigate to={`${BASE_PATH}/${encodeURIComponent(dashboards[0].id)}`} replace />;
  }

  const sharedBy = (d: SharedDashboardSummary): string => {
    const owner = d.owner ? d.owner.name || d.owner.email : "a former user";
    return d.sharedViaTeam ? `Shared with your team ${d.sharedViaTeam.name} by ${owner}` : `Shared by ${owner}`;
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
        <Box>
          <Typography variant="h5">Shared Dashboards</Typography>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 0.5, flexWrap: "wrap" }}>
            <Typography variant="body2" color="text.secondary">
              {selected.name} &middot; {sharedBy(selected)}
            </Typography>
            <Chip size="small" variant="outlined" icon={<Eye size={14} />} label="View only" />
          </Box>
        </Box>
        <DashboardSwitcher
          entries={dashboards.map((d) => ({ id: d.id, name: d.name, secondary: sharedBy(d) }))}
          selectedId={selected.id}
          onSelect={(id) => void navigate(`${BASE_PATH}/${encodeURIComponent(id)}`)}
        />
      </Box>
      <SharedDashboardView id={selected.id} />
    </Box>
  );
}
