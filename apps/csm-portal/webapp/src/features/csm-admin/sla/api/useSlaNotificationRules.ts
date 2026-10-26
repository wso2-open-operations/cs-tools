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
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { SlaNotificationRule, UpsertSlaNotificationRuleRequest } from "../types";

const SLA_NOTIFICATION_RULES_KEY = ["sla", "notification-rules"] as const;

export function useSlaNotificationRules(): UseQueryResult<SlaNotificationRule[], Error> {
  const api = useBackendApi();
  return useQuery<SlaNotificationRule[], Error>({
    queryKey: SLA_NOTIFICATION_RULES_KEY,
    queryFn: async () => {
      const res = await api.get<{ rules: SlaNotificationRule[] }>("/sla/notification-rules");
      return res?.rules ?? [];
    },
    staleTime: 30 * 1000,
  });
}

export function useCreateSlaNotificationRule(): UseMutationResult<
  SlaNotificationRule,
  Error,
  UpsertSlaNotificationRuleRequest
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SlaNotificationRule, Error, UpsertSlaNotificationRuleRequest>({
    mutationFn: (body) =>
      api.post<UpsertSlaNotificationRuleRequest, SlaNotificationRule>("/sla/notification-rules", body),
    onSettled: () => void qc.invalidateQueries({ queryKey: SLA_NOTIFICATION_RULES_KEY }),
  });
}

export function useUpdateSlaNotificationRule(): UseMutationResult<
  SlaNotificationRule,
  Error,
  { id: string; body: UpsertSlaNotificationRuleRequest }
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SlaNotificationRule, Error, { id: string; body: UpsertSlaNotificationRuleRequest }>({
    mutationFn: ({ id, body }) =>
      api.patch<UpsertSlaNotificationRuleRequest, SlaNotificationRule>(
        `/sla/notification-rules/${encodeURIComponent(id)}`,
        body,
      ),
    onSettled: () => void qc.invalidateQueries({ queryKey: SLA_NOTIFICATION_RULES_KEY }),
  });
}
