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

import {
  useMutation,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";

/** The tiers a team member can hold on the Case Alerts chain. */
export type AlertTier = "T1" | "T2" | "T3";
/** The only roles settable here: a lead-type role is never made or changed on this page. */
export type EditableRole = "engineer" | "sub_lead";

export interface TeamMemberChange {
  userId: string;
  /** Omitted: unchanged. */
  role?: EditableRole;
  /** Omitted: unchanged. `null`: clear the tier. */
  alertTier?: AlertTier | null;
}

export interface TeamMembershipResult {
  teamKey: string;
  userId: string;
  role: string;
  alertTier: AlertTier | null;
}

export interface TeamMemberMove {
  userId: string;
  toTeamKey: string;
}

const memberPath = (teamKey: string, userId: string): string =>
  `/team-schedule/teams/${encodeURIComponent(teamKey)}/members/${encodeURIComponent(userId)}`;

/**
 * Refreshes everything that lists a team's people after a membership write:
 * the schedule's team list, the Case Alerts chain (it carries each tier), the
 * rota-access tab, this page's real roster, and the directory user list it
 * renders underneath.
 */
export function invalidateTeamRoster(qc: QueryClient): void {
  const keys: unknown[][] = [
    ["team-schedule", "catalogue"],
    ["team-schedule", "paging-chain"],
    ["team-schedule", "rota-access"],
    ["team-schedule", "my-lead-teams"],
    [ApiQueryKeys.CSM_ADMIN_TEAM_MEMBERS],
    ["csm-users-search"],
  ];
  for (const queryKey of keys) void qc.invalidateQueries({ queryKey });
}

/** `PATCH /team-schedule/teams/{teamKey}/members/{userId}`: role and/or alert tier. CSM admin only. */
export function useUpdateTeamMember(
  teamKey: string,
): UseMutationResult<TeamMembershipResult, Error, TeamMemberChange> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<TeamMembershipResult, Error, TeamMemberChange>({
    mutationFn: ({ userId, ...body }) =>
      api.patch<Omit<TeamMemberChange, "userId">, TeamMembershipResult>(memberPath(teamKey, userId), body),
    onSuccess: () => invalidateTeamRoster(qc),
  });
}

/**
 * `POST /team-schedule/teams/{teamKey}/members/{userId}/move`: `teamKey` is the
 * SOURCE team. The role is kept; the alert tier is cleared.
 */
export function useMoveTeamMember(
  teamKey: string,
): UseMutationResult<TeamMembershipResult, Error, TeamMemberMove> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<TeamMembershipResult, Error, TeamMemberMove>({
    mutationFn: ({ userId, toTeamKey }) =>
      api.post<{ toTeamKey: string }, TeamMembershipResult>(`${memberPath(teamKey, userId)}/move`, { toTeamKey }),
    onSuccess: () => invalidateTeamRoster(qc),
  });
}
