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

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { SlaReportDetails, SlaReportPeriodParams } from "@features/csm-reports/api/reportTypes";

/** Builds the request path; the project may be given as its id or its key. */
export function buildSlaReportPath(projectId: string, period: SlaReportPeriodParams): string {
  const query = new URLSearchParams({ projectId, from: period.from, to: period.to });
  return `/generate-sla-report?${query.toString()}`;
}

/**
 * `GET /generate-sla-report` — see the backend's internal/handler/reports.go.
 * The previous period's report stays visible while a new period loads.
 */
export function useGetSlaReport(
  projectId: string | undefined,
  period: SlaReportPeriodParams,
): UseQueryResult<SlaReportDetails, Error> {
  const api = useBackendApi();

  return useQuery<SlaReportDetails, Error>({
    queryKey: ["sla-report", projectId ?? "", period.from, period.to],
    queryFn: () => api.get<SlaReportDetails>(buildSlaReportPath(projectId ?? "", period)) as Promise<SlaReportDetails>,
    enabled: Boolean(projectId && period.from && period.to),
    placeholderData: keepPreviousData,
  });
}
