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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { SLAReportResponse } from "@features/sales-sa/reports/api/reportTypes";

export interface SlaReportParams {
  projectSysId: string;
  from: string;
  to: string;
  /** Query stays disabled until this flips true (the source app's own
   * form-first / report-after-submit flow — see SlaReportPage). */
  enabled: boolean;
}

/** `GET /generate-sla-report` — see internal/servicenow/reports.go. */
export function useGetSlaReport(
  params: SlaReportParams,
): UseQueryResult<SLAReportResponse, Error> {
  const api = useBackendApi();
  const { projectSysId, from, to, enabled } = params;

  return useQuery<SLAReportResponse, Error>({
    queryKey: ["spl-sla-report", projectSysId, from, to],
    queryFn: () => {
      const query = new URLSearchParams({ projectSysId, from, to });
      return api.get<SLAReportResponse>(
        `/generate-sla-report?${query.toString()}`,
      ) as Promise<SLAReportResponse>;
    },
    enabled,
  });
}
