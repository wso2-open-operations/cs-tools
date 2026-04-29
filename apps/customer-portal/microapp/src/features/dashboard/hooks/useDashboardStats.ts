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

import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { cases } from "@features/cases/api/cases.queries";
import { changeRequests } from "@features/changes/api/changes.queries";
import { useProject } from "@context/project";
import { computeDashboardStats, type DashboardStats } from "@features/dashboard/services/dashboardStats.service";
import type { ModeType } from "@shared/types";

export function useDashboardStats(): {
  stats: DashboardStats;
  features: ReturnType<typeof useProject>["features"];
  navigateBySeverity: (id: string | number, label: string) => void;
  navigateByOperationsType: (id: string | number, type: string) => void;
} {
  const navigate = useNavigate();
  const {
    projectId,
    features,
  } = useProject();

  const { data: defaultCaseTypeStats } = useQuery(cases.stats(projectId!, { caseTypes: ["default_case"] }));
  const { data: engagementCaseTypeStats } = useQuery({
    ...cases.stats(projectId!, { caseTypes: ["engagement"] }),
    enabled: features?.hasEngagementsReadAccess,
  });
  const { data: serviceRequestCaseTypeStats } = useQuery({
    ...cases.stats(projectId!, { caseTypes: ["service_request"] }),
    enabled: features?.hasServiceRequestReadAccess,
  });
  const { data: changeRequestCaseTypeStats } = useQuery({
    ...changeRequests.stats(projectId!),
    enabled: !!features?.hasChangeRequestReadAccess,
  });
  const { data: multipleCaseTypesStats } = useQuery(
    cases.stats(projectId!, {
      caseTypes: [
        "default_case",
        "security_report_analysis",
        ...(features?.hasEngagementsReadAccess ? ["engagement"] : []),
        ...(features?.hasServiceRequestReadAccess ? ["service_request"] : []),
      ],
    }),
  );

  const stats = computeDashboardStats(
    multipleCaseTypesStats,
    defaultCaseTypeStats,
    engagementCaseTypeStats,
    serviceRequestCaseTypeStats,
    changeRequestCaseTypeStats,
    features,
  );

  const navigateBySeverity = (id: string | number, label: string) => {
    navigate("/cases/all", {
      state: { mode: { type: "severity", id, title: `Outstanding ${label} Cases` } as ModeType },
    });
  };

  const navigateByOperationsType = (_: string | number, type: string) => {
    if (type === "Service Requests")
      navigate("/services/all", {
        state: { mode: { type: "status", status: "outstanding", title: "Outstanding Service Requests" } as ModeType },
      });
    if (type === "Change Requests")
      navigate("/changes/all", {
        state: { mode: { type: "status", status: "outstanding", title: "Outstanding Change Requests" } as ModeType },
      });
  };

  return { stats, features, navigateBySeverity, navigateByOperationsType };
}
