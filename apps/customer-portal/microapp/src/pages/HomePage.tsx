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
import { CircleCheck, Clock4, OctagonAlert } from "@wso2/oxygen-ui-icons-react";
import { MetricWidget, PieChartWidget } from "@components/features/dashboard";
import { useQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { useProject } from "@context/project";
import { changeRequests } from "../services/changes";
import { overrideOrDefault } from "../utils/others";

const PROJECT_SEVERITY_PIE_COLORS: Record<string, string> = {
  "10": colors.red[500],
  "11": colors.orange[500],
  "12": colors.yellow[600],
  "13": colors.blue[500],
  "14": colors.green[500],
};

const ENGAGEMENTS_TYPE_PIE_COLORS: Record<string, string> = {
  Migration: colors.blue[500],
  Consultancy: colors.green[500],
  "New Feature / Improvement": colors.teal[400],
  "Follow up": colors.cyan[500],
  Onboarding: colors.yellow[600],
};

export default function HomePage() {
  const { projectId } = useProject();
  const { data: defaultCaseTypeStats } = useQuery(cases.stats(projectId!, { caseTypes: ["default_case"] }));
  const { data: enagagementCaseTypeStats } = useQuery(cases.stats(projectId!, { caseTypes: ["engagement"] }));
  const { data: serviceRequestCaseTypeStats } = useQuery(cases.stats(projectId!, { caseTypes: ["service_request"] }));
  const { data: changeRequestCaseTypeStats } = useQuery(changeRequests.stats(projectId!));
  const { data: multipleCaseTypesStats } = useQuery(
    cases.stats(projectId!, { caseTypes: ["default_case", "engagement", "service_request"] }),
  );

  const totalInteractions =
    multipleCaseTypesStats?.totalCount != undefined && changeRequestCaseTypeStats?.totalCount != undefined
      ? multipleCaseTypesStats.totalCount + changeRequestCaseTypeStats.totalCount
      : undefined;

  const activeInteractions =
    multipleCaseTypesStats?.activeCount != undefined && changeRequestCaseTypeStats?.activeCount != undefined
      ? multipleCaseTypesStats.activeCount + changeRequestCaseTypeStats.activeCount
      : undefined;

  const resolvedThisMonth = defaultCaseTypeStats?.resolvedCases.currentMonth;
  const resolvedThisMonthChangeRate = defaultCaseTypeStats?.changeRate.resolvedEngagements;
  const averageResponseTime = defaultCaseTypeStats?.averageResponseTime;
  const averageResponseTimeChangeRate = defaultCaseTypeStats?.changeRate.averageResponseTime;

  const outstandingSupportCasesPieData = defaultCaseTypeStats?.outstandingSeverityCount.map((item) => ({
    label: item.label,
    value: item.count,
    color: PROJECT_SEVERITY_PIE_COLORS[item.id] || colors.grey[500],
  }));

  const outstandingEngagementsPieData = enagagementCaseTypeStats?.outstandingEngagementTypeCount.map((item) => ({
    label: overrideOrDefault(item.label),
    value: item.count,
    color: ENGAGEMENTS_TYPE_PIE_COLORS[item.label] || colors.grey[500],
  }));

  const outstandingOperationsPieData =
    serviceRequestCaseTypeStats?.outstandingCount != undefined ||
    changeRequestCaseTypeStats?.outstandingCount != undefined
      ? [
          {
            label: "Service Requests",
            value: serviceRequestCaseTypeStats?.outstandingCount ?? 0,
            color: colors.orange[500],
          },
          {
            label: "Change Requests",
            value: changeRequestCaseTypeStats?.outstandingCount ?? 0,
            color: colors.blue[500],
          },
        ]
      : undefined;

  return (
    <>
      <Grid spacing={1.5} container>
        <Grid size={6}>
          <MetricWidget
            label="Total Interactions"
            value={totalInteractions}
            icon={<OctagonAlert size={pxToRem(18)} color={colors.orange[500]} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="Active Interactions"
            value={activeInteractions}
            icon={<OctagonAlert size={pxToRem(18)} color={colors.yellow[700]} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="Resolved This Month"
            value={resolvedThisMonth}
            trend={{ direction: "up", value: `${resolvedThisMonthChangeRate ?? 0}%` }}
            icon={<CircleCheck size={pxToRem(18)} color={colors.green[700]} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="Average Response Time"
            value={averageResponseTime !== undefined ? `${averageResponseTime}h` : undefined}
            trend={{ direction: "down", value: `${averageResponseTimeChangeRate ?? 0}%` }}
            icon={<Clock4 size={pxToRem(18)} color={colors.purple[500]} />}
          />
        </Grid>
        <Grid size={6}>
          <PieChartWidget title="Outstanding Support Cases" data={outstandingSupportCasesPieData} />
        </Grid>
        <Grid size={6}>
          <PieChartWidget title="Outstanding Operations" data={outstandingOperationsPieData} />
        </Grid>
        <Grid size={6}>
          <PieChartWidget title="Outstanding Engagements" data={outstandingEngagementsPieData} />
        </Grid>
      </Grid>
    </>
  );
}
