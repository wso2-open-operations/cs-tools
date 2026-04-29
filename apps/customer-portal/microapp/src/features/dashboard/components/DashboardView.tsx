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

import { Grid, colors, pxToRem } from "@wso2/oxygen-ui";
import { Activity, CircleCheck, Clock4, OctagonAlert } from "@wso2/oxygen-ui-icons-react";
import { MetricWidget, PieChartWidget } from "@features/dashboard/components";
import { useNavigate } from "react-router-dom";
import type { ModeType } from "@shared/types";
import type { useDashboardStats } from "@features/dashboard/hooks/useDashboardStats";

type DashboardViewProps = ReturnType<typeof useDashboardStats>;

export function DashboardView({ stats, features, navigateBySeverity, navigateByOperationsType }: DashboardViewProps) {
  const navigate = useNavigate();

  return (
    <Grid spacing={1.5} container>
      <Grid size={6}>
        <MetricWidget
          label="Action Required"
          value={stats.totalInteractions}
          icon={<OctagonAlert size={pxToRem(18)} color={colors.orange[500]} />}
          onClick={() =>
            navigate("/multiple/all", {
              state: {
                mode: { type: "status", status: "action_required", title: "Action Required Items" } as ModeType,
              },
            })
          }
        />
      </Grid>
      <Grid size={6}>
        <MetricWidget
          label="Outstanding"
          value={stats.activeInteractions}
          icon={<Clock4 size={pxToRem(18)} color={colors.yellow[700]} />}
          onClick={() =>
            navigate("/multiple/all", {
              state: { mode: { type: "status", status: "outstanding", title: "Outstanding Items" } as ModeType },
            })
          }
        />
      </Grid>
      <Grid size={6}>
        <MetricWidget
          label="Closed (30d)"
          value={stats.resolvedThisMonth}
          icon={<CircleCheck size={pxToRem(18)} color={colors.green[600]} />}
          onClick={() =>
            navigate("/multiple/all", {
              state: { mode: { type: "status", status: "resolved", title: "Closed Items (30d)" } as ModeType },
            })
          }
        />
      </Grid>
      <Grid size={6}>
        <MetricWidget
          label="Average Response Time"
          value={stats.averageResponseTime !== undefined ? `${stats.averageResponseTime}h` : undefined}
          icon={<Activity size={pxToRem(18)} color={colors.cyan[500]} />}
        />
      </Grid>

      <Grid size={6}>
        <PieChartWidget
          title="Outstanding Support Cases"
          data={stats.outstandingSupportCasesPieData}
          onClick={navigateBySeverity}
        />
      </Grid>

      {(features?.hasServiceRequestReadAccess || features?.hasChangeRequestReadAccess) && (
        <Grid size={6}>
          <PieChartWidget
            title="Outstanding Operations"
            data={stats.outstandingOperationsPieData}
            onClick={navigateByOperationsType}
          />
        </Grid>
      )}

      {features?.hasEngagementsReadAccess && (
        <Grid size={6}>
          <PieChartWidget title="Outstanding Engagements" data={stats.outstandingEngagementsPieData} />
        </Grid>
      )}
    </Grid>
  );
}
