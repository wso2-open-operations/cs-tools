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
import type { TimeLogBreakdownDetails, TimelogsPeriodParams } from "@features/csm-reports/api/reportTypes";

/** Builds the request path; only set period params are sent so the backend default applies otherwise. */
export function buildTimelogsBreakdownPath(projectId: string, period: TimelogsPeriodParams = {}): string {
  const query = new URLSearchParams({ projectId });
  if (period.allTime) {
    query.set("allTime", "true");
  } else {
    if (period.from) query.set("from", period.from);
    if (period.to) query.set("to", period.to);
  }
  return `/generate-timelogs-breakdown-report?${query.toString()}`;
}

/**
 * `GET /generate-timelogs-breakdown-report` — see the backend's internal/handler/reports.go.
 * The previous period's data stays visible while a new period loads.
 */
export function useGetTimelogsBreakdown(
  projectId: string | undefined,
  period: TimelogsPeriodParams = {},
): UseQueryResult<TimeLogBreakdownDetails, Error> {
  const api = useBackendApi();
  const from = period.allTime ? "" : (period.from ?? "");
  const to = period.allTime ? "" : (period.to ?? "");
  const allTime = Boolean(period.allTime);

  return useQuery<TimeLogBreakdownDetails, Error>({
    queryKey: ["spl-timelogs-breakdown", projectId ?? "", from, to, allTime],
    queryFn: () =>
      api.get<TimeLogBreakdownDetails>(
        buildTimelogsBreakdownPath(projectId ?? "", { from, to, allTime }),
      ) as Promise<TimeLogBreakdownDetails>,
    enabled: Boolean(projectId),
    placeholderData: keepPreviousData,
  });
}
