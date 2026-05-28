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
import { colors, Grid, pxToRem } from "@wso2/oxygen-ui";
import { Activity, CircleCheck, Clock4, OctagonAlert } from "@wso2/oxygen-ui-icons-react";

import { useDeclareLayout } from "@context/layout";
import { useProject } from "@context/project";

import { WidgetMetric, WidgetPieChart } from "@features/dashboard/components";
import { useDashboardStats } from "@features/dashboard/hooks";

import {
  CASE_TYPES,
  DASHBOARD_METRIC_ACTION_REQUIRED,
  DASHBOARD_METRIC_AVG_RESPONSE_TIME,
  DASHBOARD_METRIC_CLOSED,
  DASHBOARD_METRIC_OUTSTANDING,
  DASHBOARD_WIDGET_OUTSTANDING_ENGAGEMENTS,
  DASHBOARD_WIDGET_OUTSTANDING_OPERATIONS,
  DASHBOARD_WIDGET_OUTSTANDING_SUPPORT_CASES,
  Tab,
} from "@shared/constants";
import { useNavigation } from "@shared/hooks";

import { Fab } from "@components/core";

export default function HomePage() {
  useDeclareLayout({
    tabIndex: Tab.Home,
    visibility: {
      exitButton: true,
      projectSelector: true,
    },
  });

  const { features } = useProject();
  const {
    toActionRequiredItems,
    toOutstandingItems,
    toClosedItems,
    toOutstandingServiceRequests,
    toOutstandingChangeRequests,
    toBySeverity,
  } = useNavigation();
  const stats = useDashboardStats();

  const navigateByOperationsType = (type: string | number) => {
    if (type === CASE_TYPES.SERVICE_REQUEST) toOutstandingServiceRequests();
    if (type === CASE_TYPES.CHANGE_REQUEST) toOutstandingChangeRequests();
  };

  return (
    <>
      <Grid spacing={1.5} container>
        <Grid size={6}>
          <WidgetMetric
            label={DASHBOARD_METRIC_ACTION_REQUIRED}
            value={stats.actionRequired}
            icon={<OctagonAlert size={pxToRem(18)} color={colors.orange[500]} />}
            onClick={toActionRequiredItems}
          />
        </Grid>

        <Grid size={6}>
          <WidgetMetric
            label={DASHBOARD_METRIC_OUTSTANDING}
            value={stats.outstanding}
            icon={<Clock4 size={pxToRem(18)} color={colors.yellow[700]} />}
            onClick={toOutstandingItems}
          />
        </Grid>

        <Grid size={6}>
          <WidgetMetric
            label={DASHBOARD_METRIC_CLOSED}
            value={stats.resolvedThisMonth}
            icon={<CircleCheck size={pxToRem(18)} color={colors.green[600]} />}
            onClick={toClosedItems}
          />
        </Grid>

        <Grid size={6}>
          <WidgetMetric
            label={DASHBOARD_METRIC_AVG_RESPONSE_TIME}
            value={stats.averageResponseTime !== undefined ? `${stats.averageResponseTime}h` : undefined}
            icon={<Activity size={pxToRem(18)} color={colors.cyan[500]} />}
          />
        </Grid>

        <Grid size={6}>
          <WidgetPieChart
            title={DASHBOARD_WIDGET_OUTSTANDING_SUPPORT_CASES}
            data={stats.outstandingSupportCasesPieData}
            onClick={toBySeverity}
          />
        </Grid>

        {(features?.hasServiceRequestReadAccess || features?.hasChangeRequestReadAccess) && (
          <Grid size={6}>
            <WidgetPieChart
              title={DASHBOARD_WIDGET_OUTSTANDING_OPERATIONS}
              data={stats.outstandingOperationsPieData}
              onClick={navigateByOperationsType}
            />
          </Grid>
        )}

        {features?.hasEngagementsReadAccess && (
          <Grid size={6}>
            <WidgetPieChart
              title={DASHBOARD_WIDGET_OUTSTANDING_ENGAGEMENTS}
              data={stats.outstandingEngagementsPieData}
            />
          </Grid>
        )}
      </Grid>

      <Fab />
    </>
  );
}
