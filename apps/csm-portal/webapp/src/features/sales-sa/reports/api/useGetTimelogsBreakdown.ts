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
import type { TimeLogBreakdownDetails } from "@features/sales-sa/reports/api/reportTypes";

/** `GET /generate-timelogs-breakdown-report` — see internal/servicenow/reports.go. */
export function useGetTimelogsBreakdown(
  projectId: string | undefined,
): UseQueryResult<TimeLogBreakdownDetails, Error> {
  const api = useBackendApi();

  return useQuery<TimeLogBreakdownDetails, Error>({
    queryKey: ["spl-timelogs-breakdown", projectId ?? ""],
    queryFn: () =>
      api.get<TimeLogBreakdownDetails>(
        `/generate-timelogs-breakdown-report?projectId=${encodeURIComponent(projectId ?? "")}`,
      ) as Promise<TimeLogBreakdownDetails>,
    enabled: Boolean(projectId),
  });
}
