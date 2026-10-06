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

import { useMutation, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { ActionItemComment, RiskActionItem } from "./customerHealthTypes";

// GET /customer-health/accounts/{accountSysId}/action-items
export function useAccountActionItems(accountId: string | undefined): UseQueryResult<RiskActionItem[], Error> {
  const backendApi = useBackendApi();
  return useQuery<RiskActionItem[], Error>({
    queryKey: ["spl-customer-health-action-items", accountId],
    enabled: Boolean(accountId),
    queryFn: async () =>
      (await backendApi.get<RiskActionItem[]>(
        `/customer-health/accounts/${encodeURIComponent(accountId ?? "")}/action-items`,
      )) ?? [],
  });
}

export interface CreateActionItemPayload {
  title: string;
  description: string | null;
  priority: string;
  assignedToEmail: string | null;
  dueDate: string;
  projectSysId: string;
  accountSysId: string;
}

// POST /customer-health/risks/{riskId}/action-items — riskId is an INTEGER.
export function useCreateActionItem() {
  const backendApi = useBackendApi();
  return useMutation<RiskActionItem, Error, { riskId: number; payload: CreateActionItemPayload }>({
    mutationFn: ({ riskId, payload }) =>
      backendApi.post<CreateActionItemPayload, RiskActionItem>(
        `/customer-health/risks/${riskId}/action-items`,
        payload,
      ),
  });
}

// PUT /customer-health/action-items/{actionItemId}/status — actionItemId is an INTEGER.
export function useUpdateActionItemStatus() {
  const backendApi = useBackendApi();
  return useMutation<
    RiskActionItem,
    Error,
    { actionItemId: number; status: string; resolutionComment: string | null }
  >({
    mutationFn: ({ actionItemId, status, resolutionComment }) =>
      backendApi.put<{ status: string; resolutionComment: string | null }, RiskActionItem>(
        `/customer-health/action-items/${actionItemId}/status`,
        { status, resolutionComment },
      ),
  });
}

// GET/POST /customer-health/action-items/{actionItemId}/comments — actionItemId is an INTEGER.
export function useActionItemComments(actionItemId: number, enabled: boolean): UseQueryResult<ActionItemComment[], Error> {
  const backendApi = useBackendApi();
  return useQuery<ActionItemComment[], Error>({
    queryKey: ["spl-customer-health-action-item-comments", actionItemId],
    enabled,
    queryFn: async () =>
      (await backendApi.get<ActionItemComment[]>(
        `/customer-health/action-items/${actionItemId}/comments`,
      )) ?? [],
  });
}

export function useCreateActionItemComment() {
  const backendApi = useBackendApi();
  return useMutation<ActionItemComment, Error, { actionItemId: number; comment: string }>({
    mutationFn: ({ actionItemId, comment }) =>
      backendApi.post<{ comment: string }, ActionItemComment>(
        `/customer-health/action-items/${actionItemId}/comments`,
        { comment },
      ),
  });
}
