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
import {
  sharedCodeDashboardPath,
  useGetSharedCodeDashboard,
  useGetSharedCodeDashboards,
} from "@features/csm-custom-dashboards/api/useCodeDashboardShares";
import CodeDashboardFrame from "@features/csm-custom-dashboards/components/CodeDashboardFrame";
import DashboardSwitcher from "@features/csm-custom-dashboards/components/DashboardSwitcher";
import { useCanViewAdvancedDashboards } from "@features/csm-custom-dashboards/utils/advancedDashboardAccess";

const BASE_PATH = "/dashboard/shared";
// A switcher value that opens a shared advanced dashboard, not a custom one.
const CODE_VALUE_PREFIX = "code:";

/** The advanced dashboards shared with the user, as the page needs them. */
interface SharedCodeList {
  dashboards: SharedDashboardSummary[];
  isLoading: boolean;
  isFetching: boolean;
  isError: boolean;
}

/** While the user cannot view advanced dashboards: none, and nothing is read. */
const NO_CODE_DASHBOARDS: SharedCodeList = { dashboards: [], isLoading: false, isFetching: false, isError: false };

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
      <DashboardWidgetGrid widgets={shared.data.dashboard.widgets} linePointClickThrough />
    </Box>
  );
}

/**
 * One advanced dashboard shared with the user (`GET /dashboards/shared/code/{id}`):
 * the owner's current source, run in CodeDashboardFrame (the source check and
 * sandbox, with the viewer's own data), in a panel as tall as the saved
 * advanced dashboard page's. One that is no longer shared says so.
 */
function SharedCodeDashboardView({ id }: { id: string }): JSX.Element {
  const shared = useGetSharedCodeDashboard(id);

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
    <Box
      data-testid="shared-code-dashboard-panel"
      sx={{ ...frame, height: "calc(100dvh - 240px)", minHeight: 520, display: "flex", flexDirection: "column" }}
    >
      <CodeDashboardFrame key={shared.data.source} source={shared.data.source} />
    </Box>
  );
}

/**
 * Dashboards > Shared Dashboards (`/dashboard/shared`,
 * `/dashboard/shared/:dashboardId`): the dashboards other users shared with
 * the user (`GET /dashboards/shared`), view only: no edit, share or delete. A
 * switcher with search, as My Dashboards has, each entry naming who shared
 * it ("Shared by <owner>", or "Shared with your team <team> by <owner>" when
 * it reached the user through a team, "Opened from a link shared by <owner>"
 * when they have it only through its link; "a former user" once the owner's
 * user has been deleted); the open one says the same and "View only". No id in the URL,
 * or one that is not in the list, opens the first one; while the list is
 * being re-read an unknown id waits for it, as on My Dashboards.
 *
 * For a user who can view advanced dashboards (`canViewAdvancedDashboards`),
 * the advanced dashboards shared with them (`GET /dashboards/shared/code`)
 * follow in the switcher under "Advanced", and open at
 * `/dashboard/shared/code/:codeDashboardId`, run in CodeDashboardFrame. If
 * only that list fails, the custom ones still show, with a line saying so.
 * For anyone else nothing about advanced dashboards is read.
 */
export default function SharedDashboardsPage(): JSX.Element {
  return useCanViewAdvancedDashboards() ? <WithSharedCodeDashboards /> : <SharedDashboards code={NO_CODE_DASHBOARDS} />;
}

/** Reads the advanced dashboards shared with the user, then the page. */
function WithSharedCodeDashboards(): JSX.Element {
  const list = useGetSharedCodeDashboards();
  return (
    <SharedDashboards
      code={{ dashboards: list.data ?? [], isLoading: list.isLoading, isFetching: list.isFetching, isError: list.isError }}
    />
  );
}

function SharedDashboards({ code }: { code: SharedCodeList }): JSX.Element {
  const { dashboardId, codeDashboardId } = useParams<{ dashboardId?: string; codeDashboardId?: string }>();
  const navigate = useNavigate();
  const list = useGetSharedDashboards();
  const dashboards = list.data ?? [];
  const codeError = code.isError && (
    <Typography variant="body2" color="error">
      Could not load the advanced dashboards shared with you.
    </Typography>
  );

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

  const selectedCode = codeDashboardId
    ? code.dashboards.find((d) => d.id === codeDashboardId.toLowerCase())
    : undefined;
  const selected = selectedCode ?? (codeDashboardId ? undefined : dashboards.find((d) => d.id === dashboardId?.toLowerCase()));

  if (
    list.isLoading ||
    code.isLoading ||
    (dashboardId && !selected && list.isFetching) ||
    (codeDashboardId && !selected && code.isFetching)
  ) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }

  if (dashboards.length === 0 && code.dashboards.length === 0) {
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
        {codeError}
      </Box>
    );
  }

  if (!selected) {
    // No id, or one that is not shared with the user: open the first one.
    const first =
      dashboards.length > 0
        ? `${BASE_PATH}/${encodeURIComponent(dashboards[0].id)}`
        : sharedCodeDashboardPath(code.dashboards[0].id);
    return <Navigate to={first} replace />;
  }

  /**
   * How the dashboard reached the user, for the line under its name: through a team,
   * through the link, or shared directly. An owner who was deleted is "a former user".
   */
  const sharedBy = (d: SharedDashboardSummary): string => {
    const owner = d.owner ? d.owner.name || d.owner.email : "a former user";
    if (d.sharedViaTeam) return `Shared with your team ${d.sharedViaTeam.name} by ${owner}`;
    return d.sharedViaLink ? `Opened from a link shared by ${owner}` : `Shared by ${owner}`;
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
          entries={[
            ...dashboards.map((d) => ({ id: d.id, name: d.name, secondary: sharedBy(d) })),
            ...code.dashboards.map((d) => ({
              id: `${CODE_VALUE_PREFIX}${d.id}`,
              name: d.name,
              secondary: sharedBy(d),
              group: "Advanced",
            })),
          ]}
          selectedId={selectedCode ? `${CODE_VALUE_PREFIX}${selectedCode.id}` : selected.id}
          onSelect={(id) =>
            void navigate(
              id.startsWith(CODE_VALUE_PREFIX)
                ? sharedCodeDashboardPath(id.slice(CODE_VALUE_PREFIX.length))
                : `${BASE_PATH}/${encodeURIComponent(id)}`,
            )
          }
        />
      </Box>
      {codeError}
      {selectedCode ? <SharedCodeDashboardView id={selectedCode.id} /> : <SharedDashboardView id={selected.id} />}
    </Box>
  );
}
