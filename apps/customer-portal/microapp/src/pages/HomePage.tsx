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
import { MetricWidget, PieChartWidget, type PieDataItem } from "@components/features/dashboard";
import { useQuery } from "@tanstack/react-query";
import { cases } from "@src/services/cases";
import { useProject } from "@context/project";
import { changeRequests } from "../services/changes";
import { overrideOrDefault } from "../utils/others";
import { Fab } from "../components/core";
import { useNavigate } from "react-router-dom";

import { ENGAGEMENTS_TYPE_PIE_COLORS, PROJECT_SEVERITY_PIE_COLORS } from "../config/constants";
import type { ModeType } from "./AllItemsPage";

export default function HomePage() {
  const navigate = useNavigate();
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
        "security_report_analysis",
        ...(hasEngagementsReadAccess ? ["engagement"] : []),
        ...(hasServiceRequestReadAccess ? ["service_request"] : []),
      ],
    }),
  );

  const isInteractionsLoading =
    multipleCaseTypesStats === undefined || (hasChangeRequestReadAccess && changeRequestCaseTypeStats === undefined);

  const totalInteractions = isInteractionsLoading
    ? undefined
    : (multipleCaseTypesStats?.actionRequiredCount ?? 0) +
      (hasChangeRequestReadAccess ? (changeRequestCaseTypeStats?.actionRequiredCount ?? 0) : 0);

  const activeInteractions = isInteractionsLoading
    ? undefined
    : (multipleCaseTypesStats?.outstandingCount ?? 0) +
      (hasChangeRequestReadAccess ? (changeRequestCaseTypeStats?.outstandingCount ?? 0) : 0);

  const resolvedThisMonth =
    multipleCaseTypesStats?.resolvedCases?.pastThirtyDays === undefined &&
    (!hasChangeRequestReadAccess || changeRequestCaseTypeStats?.resolvedCount?.pastThirtyDays === undefined)
      ? undefined
      : (multipleCaseTypesStats?.resolvedCases?.pastThirtyDays ?? 0) +
        (hasChangeRequestReadAccess ? (changeRequestCaseTypeStats?.resolvedCount?.pastThirtyDays ?? 0) : 0);

  const averageResponseTime = multipleCaseTypesStats?.averageResponseTime;

  const outstandingSupportCasesPieData = defaultCaseTypeStats?.outstandingSeverityCount.map((item) => ({
    id: item.id,
    label: overrideOrDefault(item.label),
    value: item.count,
    color: PROJECT_SEVERITY_PIE_COLORS[item.id] || colors.grey[500],
  }));

  const outstandingEngagementsPieData: (PieDataItem & { id: string | number })[] =
    engagementCaseTypeStats?.outstandingEngagementTypeCount.map((item) => ({
      id: item.id,
      label: overrideOrDefault(item.label),
      value: item.count,
      color: ENGAGEMENTS_TYPE_PIE_COLORS[item.label] || colors.grey[500],
    })) ?? [];

  const data: (PieDataItem & { id: string | number })[] = [];

  if (hasServiceRequestReadAccess) {
    data.push({
      id: "service",
      label: "Service Requests",
      value: serviceRequestCaseTypeStats?.outstandingCount ?? 0,
      color: colors.orange[500],
    });
  }

  if (hasChangeRequestReadAccess) {
    data.push({
      id: "change",
      label: "Change Requests",
      value: changeRequestCaseTypeStats?.outstandingCount ?? 0,
      color: colors.blue[500],
    });
  }

  const outstandingOperationsPieData =
    serviceRequestCaseTypeStats?.outstandingCount != undefined ||
    changeRequestCaseTypeStats?.outstandingCount != undefined
      ? data
      : undefined;

  const navigateBySeverity = (id: string | number, label: string) => {
    console.log("severity id: ", id);
    navigate("/cases/all", {
      state: { mode: { type: "severity", id, title: `Outstanding ${label} Cases` } as ModeType },
    });
  };

  const navigateByServiceRequestOrChageRequest = (_: string | number, type: string) => {
    if (type === "Service Requests")
      navigate("/services/all", {
        state: { mode: { type: "status", status: "outstanding", title: "Outstanding Service Requests" } as ModeType },
      });

    if (type === "Change Requests")
      navigate("/changes/all", {
        state: { mode: { type: "status", status: "outstanding", title: "Outstanding Change Requests" } as ModeType },
      });
  };

  return (
    <>
      <Grid spacing={1.5} container>
        <Grid size={6}>
          <MetricWidget
            label="Action Required"
            value={totalInteractions}
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
            value={activeInteractions}
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
            value={resolvedThisMonth}
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
            value={averageResponseTime !== undefined ? `${averageResponseTime}h` : undefined}
            icon={<Activity size={pxToRem(18)} color={colors.cyan[500]} />}
          />
        </Grid>

        <Grid size={6}>
          <PieChartWidget
            title="Outstanding Support Cases"
            data={outstandingSupportCasesPieData}
            onClick={navigateBySeverity}
          />
        </Grid>

        {(hasServiceRequestReadAccess || hasChangeRequestReadAccess) && (
          <Grid size={6}>
            <PieChartWidget
              title="Outstanding Operations"
              data={outstandingOperationsPieData}
              onClick={navigateByServiceRequestOrChageRequest}
            />
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
