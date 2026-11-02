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
import type { JSX } from "react";
import { useNavigate, useParams } from "react-router";
import { useGetCustomDashboard } from "@features/csm-custom-dashboards/api/useCustomDashboards";
import GenerateDashboardPage from "@features/csm-custom-dashboards/pages/GenerateDashboardPage";

const MY_DASHBOARDS_PATH = "/dashboard/my";

/**
 * Dashboards > My Dashboards > edit (`/dashboard/my/:dashboardId/edit`),
 * opened by Edit with AI: loads the saved dashboard and hands it to the
 * generate page as its starting draft, so a chat message changes it and Save
 * replaces it in place. Keyed by id, so moving to another dashboard's edit
 * page starts fresh.
 *
 * While loading it shows a skeleton; a dashboard that cannot be loaded (gone,
 * another user's, or saved under rules that have since changed) shows why,
 * with the generate page's Back.
 */
export default function EditCustomDashboardPage(): JSX.Element {
  const { dashboardId } = useParams<{ dashboardId: string }>();
  const navigate = useNavigate();
  const saved = useGetCustomDashboard(dashboardId);

  if (saved.data) {
    return <GenerateDashboardPage key={saved.data.dashboard.id} initialDashboard={saved.data.dashboard} />;
  }
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
