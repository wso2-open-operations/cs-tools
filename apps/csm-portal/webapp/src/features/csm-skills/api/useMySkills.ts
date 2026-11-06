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

import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { skillKeys } from "@features/csm-skills/api/skillKeys";
import type {
  MySkillsResponse,
  SkillHistoryResponse,
  UserSkill,
} from "@features/csm-skills/types";

/** `GET /skills/me`: the caller's own skills, inherited rows included. */
export function useMySkills(): UseQueryResult<MySkillsResponse, Error> {
  const api = useBackendApi();
  return useQuery<MySkillsResponse, Error>({
    queryKey: skillKeys.mine,
    queryFn: async () => (await api.get<MySkillsResponse>("/skills/me")) ?? { userId: "", skills: [] },
  });
}

/** The profile changed, so the profile and its history are stale. */
function useRefreshProfile(): () => Promise<unknown> {
  const qc = useQueryClient();
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: skillKeys.mine }),
      qc.invalidateQueries({ queryKey: [...skillKeys.root, "history"] }),
      // A manager's table lists these same rows.
      qc.invalidateQueries({ queryKey: [...skillKeys.root, "people"] }),
    ]);
}

export interface AddMySkillInput {
  skillId: string;
  levelId: string;
}

/** `POST /skills/me`: put a skill on the caller's profile. */
export function useAddMySkill(): UseMutationResult<UserSkill, Error, AddMySkillInput> {
  const api = useBackendApi();
  const refresh = useRefreshProfile();
  return useMutation<UserSkill, Error, AddMySkillInput>({
    mutationFn: (body) => api.post<AddMySkillInput, UserSkill>("/skills/me", body),
    onSuccess: refresh,
  });
}

export interface ChangeMySkillLevelInput {
  skillId: string;
  levelId: string;
}

/** `PUT /skills/me/{skillId}`: change the level of a skill the caller holds. */
export function useChangeMySkillLevel(): UseMutationResult<UserSkill, Error, ChangeMySkillLevelInput> {
  const api = useBackendApi();
  const refresh = useRefreshProfile();
  return useMutation<UserSkill, Error, ChangeMySkillLevelInput>({
    mutationFn: ({ skillId, levelId }) =>
      api.put<{ levelId: string }, UserSkill>(`/skills/me/${encodeURIComponent(skillId)}`, { levelId }),
    onSuccess: refresh,
  });
}

/** `DELETE /skills/me/{skillId}`: remove a skill from the caller's profile (204). */
export function useRemoveMySkill(): UseMutationResult<unknown, Error, string> {
  const api = useBackendApi();
  const refresh = useRefreshProfile();
  return useMutation<unknown, Error, string>({
    mutationFn: (skillId) => api.del<unknown>(`/skills/me/${encodeURIComponent(skillId)}`),
    onSuccess: refresh,
  });
}

/** `GET /skills/me/history`: level changes of one skill, newest first, paged by the backend. */
export function useMySkillHistory(
  skillId: string | undefined,
  limit: number,
  offset: number,
): UseQueryResult<SkillHistoryResponse, Error> {
  const api = useBackendApi();
  return useQuery<SkillHistoryResponse, Error>({
    queryKey: skillKeys.history(skillId ?? "", limit, offset),
    queryFn: async () =>
      (await api.get<SkillHistoryResponse>(
        `/skills/me/history?skillId=${encodeURIComponent(skillId as string)}&limit=${limit}&offset=${offset}`,
      )) ?? { history: [], total: 0, limit, offset },
    enabled: !!skillId,
    placeholderData: keepPreviousData,
  });
}
