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
import type { ABTTeamScheduleData } from "@features/sales-sa/schedule/scheduleTypes";

export interface TeamScheduleParams {
  teamId: string;
  duration: string;
  from: string;
  eventType: string;
}

/**
 * `GET /abt-team-schedule` — see internal/servicenow/schedule.go. All
 * query params are optional on the backend; empty strings are sent as-is
 * (matching the source app's own behavior) rather than omitted.
 */
export function useGetTeamSchedule(
  params: TeamScheduleParams,
): UseQueryResult<ABTTeamScheduleData, Error> {
  const api = useBackendApi();
  const { teamId, duration, from, eventType } = params;

  return useQuery<ABTTeamScheduleData, Error>({
    queryKey: ["spl-team-schedule", teamId, duration, from, eventType],
    queryFn: async () => {
      const query = new URLSearchParams({ teamId, from, duration, eventType });
      const result = await api.get<ABTTeamScheduleData>(
        `/abt-team-schedule?${query.toString()}`,
      );
      // GET never 404s on this endpoint (empty filters just return an empty
      // list), so a null here would only mean an unexpected 204 — treat it
      // as the same "nothing to show" shape callers already handle.
      return result ?? { list: [], metadata: [], snURL: "" };
    },
  });
}
