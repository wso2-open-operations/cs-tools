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

import { Box, Button, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ReactNode } from "react";
import { Navigate, useNavigate, useParams } from "react-router";
import { useGetCodeDashboard, type SavedCodeDashboard } from "@features/csm-custom-dashboards/api/useCodeDashboards";
import GenerateCodeDashboardPage from "@features/csm-custom-dashboards/pages/GenerateCodeDashboardPage";
import { useCanGenerateAdvancedDashboards } from "@features/csm-custom-dashboards/utils/advancedDashboardAccess";

const MY_DASHBOARDS_PATH = "/dashboard/my";

/**
 * Loads the saved advanced dashboard named by the route's `:dashboardId` and
 * hands it to `children`. While loading it shows a skeleton; one that cannot
 * be loaded (gone, another user's) shows why, with Back to My Dashboards. A
 * user who cannot use advanced dashboards (`canGenerateAdvancedDashboards`)
 * gets the 404 page, as on `/dashboard/my/new-code`: the backend's routes do
 * not exist for them either.
 */
function WithSavedCodeDashboard({
  children,
}: {
  children: (dashboard: SavedCodeDashboard) => ReactNode;
}): JSX.Element {
  const allowed = useCanGenerateAdvancedDashboards();
  const { dashboardId } = useParams<{ dashboardId: string }>();
  const navigate = useNavigate();
  const saved = useGetCodeDashboard(allowed ? dashboardId : undefined);

  if (!allowed) return <Navigate to="/404" replace />;
  if (saved.data) return <>{children(saved.data)}</>;
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => void navigate(MY_DASHBOARDS_PATH)}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to My Dashboards
      </Button>
      {saved.isLoading ? (
        <Skeleton variant="rounded" height={320} />
      ) : (
        <Typography variant="body2" color="text.secondary">
          {saved.isError
            ? saved.error.message || "Could not load this dashboard."
            : "This dashboard no longer exists."}
        </Typography>
      )}
    </Box>
  );
}

/**
 * Dashboards > My Dashboards > a saved advanced dashboard > edit
 * (`/dashboard/my/code/:dashboardId/edit`): the advanced page started from
 * the saved dashboard, so a message changes it and Save replaces it in place.
 * Keyed by id, so moving to another dashboard's edit page starts fresh.
 */
export default function EditCodeDashboardPage(): JSX.Element {
  return (
    <WithSavedCodeDashboard>
      {(dashboard) => <GenerateCodeDashboardPage key={dashboard.id} initialDashboard={dashboard} />}
    </WithSavedCodeDashboard>
  );
}
