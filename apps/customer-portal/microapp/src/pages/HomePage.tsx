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
import { Fab } from "../components/core";
import { ENGAGEMENTS_TYPE_PIE_COLORS, PROJECT_SEVERITY_PIE_COLORS } from "../config/constants";

export default function HomePage() {
  const {
    projectId,
    features: { hasServiceRequestReadAccess, hasChangeRequestReadAccess, hasEngagementsReadAccess } = {},
  } = useProject();
  const { data: defaultCaseTypeStats } = useQuery(cases.stats(projectId!, { caseTypes: ["default_case"] }));
  const { data: engagementCaseTypeStats } = useQuery({
    ...cases.stats(projectId!, { caseTypes: ["engagement"] }),
    enabled: hasEngagementsReadAccess,
  });

  const { data: serviceRequestCaseTypeStats } = useQuery({
    ...cases.stats(projectId!, { caseTypes: ["service_request"] }),
    enabled: hasServiceRequestReadAccess,
  });

  const { data: changeRequestCaseTypeStats } = useQuery({
    ...changeRequests.stats(projectId!),
    enabled: !!hasChangeRequestReadAccess,
  });

  const { data: multipleCaseTypesStats } = useQuery(
    cases.stats(projectId!, {
      caseTypes: [
        "default_case",
        ...(hasEngagementsReadAccess ? ["engagement"] : []),
        ...(hasServiceRequestReadAccess ? ["service_request"] : []),
      ],
    }),
  );

  const isInteractionsLoading =
    multipleCaseTypesStats === undefined || (hasChangeRequestReadAccess && changeRequestCaseTypeStats === undefined);

  // const totalInteractions =
  //   multipleCaseTypesStats?.totalCount != undefined && changeRequestCaseTypeStats?.totalCount != undefined
  //     ? multipleCaseTypesStats.totalCount + changeRequestCaseTypeStats.totalCount
  //     : undefined;

  const totalInteractions = isInteractionsLoading
    ? undefined
    : (multipleCaseTypesStats?.totalCount ?? 0) +
      (hasChangeRequestReadAccess ? (changeRequestCaseTypeStats?.totalCount ?? 0) : 0);

  // const activeInteractions =
  //   multipleCaseTypesStats?.activeCount != undefined && changeRequestCaseTypeStats?.activeCount != undefined
  //     ? multipleCaseTypesStats.activeCount + changeRequestCaseTypeStats.activeCount
  //     : undefined;

  const activeInteractions = isInteractionsLoading
    ? undefined
    : (multipleCaseTypesStats?.activeCount ?? 0) +
      (hasChangeRequestReadAccess ? (changeRequestCaseTypeStats?.activeCount ?? 0) : 0);

  const resolvedThisMonth = defaultCaseTypeStats?.resolvedCases.pastThirtyDays;
  const resolvedThisMonthChangeRate = defaultCaseTypeStats?.changeRate.resolvedEngagements;
  const averageResponseTime = defaultCaseTypeStats?.averageResponseTime;
  const averageResponseTimeChangeRate = defaultCaseTypeStats?.changeRate.averageResponseTime;

  const outstandingSupportCasesPieData = defaultCaseTypeStats?.outstandingSeverityCount.map((item) => ({
    label: overrideOrDefault(item.label),
    value: item.count,
    color: PROJECT_SEVERITY_PIE_COLORS[item.id] || colors.grey[500],
  }));

  const outstandingEngagementsPieData = engagementCaseTypeStats?.outstandingEngagementTypeCount.map((item) => ({
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
            label="Action Required"
            value={totalInteractions}
            icon={<OctagonAlert size={pxToRem(18)} color={colors.orange[500]} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="Outstanding"
            value={activeInteractions}
            icon={<OctagonAlert size={pxToRem(18)} color={colors.yellow[700]} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="Resolved"
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

        {(hasServiceRequestReadAccess || hasChangeRequestReadAccess) && (
          <Grid size={6}>
            <PieChartWidget title="Outstanding Operations" data={outstandingOperationsPieData} />
          </Grid>
        )}

        {hasEngagementsReadAccess && (
          <Grid size={6}>
            <PieChartWidget title="Outstanding Engagements" data={outstandingEngagementsPieData} />
          </Grid>
        )}
      </Grid>

      {/* Floating Action Button */}
      <Fab />
    </>
  );
}
