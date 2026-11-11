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
  useQueries,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  AlertFieldValuesResponse,
  AlertMuteRuleField,
  AlertMuteRuleInput,
  AlertMuteRule,
  AlertMuteRuleMutedAlertsResponse,
  AlertMuteRuleSearchPayload,
  AlertMuteRuleSearchResponse,
  AlertMuteRuleStateFilter,
} from "@features/csm-operations/types/alertMuteRules";

/** One page of alert mute rules (`POST /alert-mute-rules/search`), newest first. */
export function useSearchAlertMuteRules(
  payload: AlertMuteRuleSearchPayload,
): UseQueryResult<AlertMuteRuleSearchResponse, Error> {
  const api = useBackendApi();
  return useQuery<AlertMuteRuleSearchResponse, Error>({
    queryKey: [ApiQueryKeys.ALERT_MUTE_RULES, "search", payload],
    queryFn: () => api.post<AlertMuteRuleSearchPayload, AlertMuteRuleSearchResponse>("/alert-mute-rules/search", payload),
    placeholderData: keepPreviousData,
  });
}

const COUNTED_STATES = ["active", "scheduled", "expired"] as const;

/**
 * How many rules each state filter would show for `searchQuery`, for the
 * filter buttons' counts: one one-row search per state ("all" is their sum).
 * `undefined` until every count has loaded.
 */
export function useAlertMuteRuleCounts(searchQuery: string): Record<AlertMuteRuleStateFilter, number> | undefined {
  const api = useBackendApi();
  const results = useQueries({
    queries: COUNTED_STATES.map((state) => {
      const payload: AlertMuteRuleSearchPayload = {
        filters: { states: [state], ...(searchQuery && { searchQuery }) },
        pagination: { offset: 0, limit: 1 },
      };
      return {
        queryKey: [ApiQueryKeys.ALERT_MUTE_RULES, "count", payload],
        queryFn: () => api.post<AlertMuteRuleSearchPayload, AlertMuteRuleSearchResponse>("/alert-mute-rules/search", payload),
        placeholderData: keepPreviousData,
      };
    }),
  });
  if (results.some((r) => r.data === undefined)) return undefined;
  const [active, scheduled, expired] = results.map((r) => r.data?.total ?? 0);
  return { all: active + scheduled + expired, active, scheduled, expired };
}

/**
 * One page of the alerts a rule muted (`GET /alert-mute-rules/{id}/muted-alerts`), newest first. They live in
 * alert-core's database, which keeps them for its retention period (30 days by default). `live` polls every 30 s,
 * for a rule that is still muting.
 */
export function useAlertMuteRuleMutedAlerts(
  id: string | undefined,
  page: { limit: number; offset: number },
  live = false,
): UseQueryResult<AlertMuteRuleMutedAlertsResponse, Error> {
  const api = useBackendApi();
  return useQuery<AlertMuteRuleMutedAlertsResponse, Error>({
    queryKey: [
      ApiQueryKeys.ALERT_MUTE_RULES,
      "muted-alerts",
      id ?? "",
      page.limit,
      page.offset,
    ],
    queryFn: async () => {
      const res = await api.get<AlertMuteRuleMutedAlertsResponse>(
        `/alert-mute-rules/${encodeURIComponent(id as string)}/muted-alerts?limit=${page.limit}&offset=${page.offset}`,
      );
      if (!res) throw new Error("No muted alerts response.");
      return res;
    },
    enabled: Boolean(id),
    placeholderData: keepPreviousData,
    refetchInterval: live ? 30_000 : false,
  });
}

export function useGetAlertMuteRule(id: string | undefined): UseQueryResult<AlertMuteRule, Error> {
  const api = useBackendApi();
  return useQuery<AlertMuteRule, Error>({
    queryKey: [ApiQueryKeys.ALERT_MUTE_RULES, "detail", id ?? ""],
    queryFn: async () => {
      const rule = await api.get<AlertMuteRule>(`/alert-mute-rules/${encodeURIComponent(id as string)}`);
      if (!rule) throw new Error("Alert mute rule not found.");
      return rule;
    },
    enabled: !!id,
    retry: false,
  });
}

function useInvalidate(): () => Promise<void> {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ALERT_MUTE_RULES] });
}

const path = (id: string, action = ""): string => `/alert-mute-rules/${encodeURIComponent(id)}${action}`;

export function usePostAlertMuteRule(): UseMutationResult<AlertMuteRule, Error, AlertMuteRuleInput> {
  const api = useBackendApi();
  const invalidate = useInvalidate();
  return useMutation<AlertMuteRule, Error, AlertMuteRuleInput>({
    mutationFn: (input) => api.post<AlertMuteRuleInput, AlertMuteRule>("/alert-mute-rules", input),
    onSuccess: invalidate,
  });
}

/** Changes a rule that hasn't started (the backend refuses any other). */
export function usePatchAlertMuteRule(): UseMutationResult<AlertMuteRule, Error, { id: string; input: AlertMuteRuleInput }> {
  const api = useBackendApi();
  const invalidate = useInvalidate();
  return useMutation<AlertMuteRule, Error, { id: string; input: AlertMuteRuleInput }>({
    mutationFn: ({ id, input }) => api.patch<AlertMuteRuleInput, AlertMuteRule>(path(id), input),
    onSuccess: invalidate,
  });
}

export function useExtendAlertMuteRule(): UseMutationResult<AlertMuteRule, Error, { id: string; endOn: string }> {
  const api = useBackendApi();
  const invalidate = useInvalidate();
  return useMutation<AlertMuteRule, Error, { id: string; endOn: string }>({
    mutationFn: ({ id, endOn }) => api.post<{ endOn: string }, AlertMuteRule>(path(id, "/extend"), { endOn }),
    onSuccess: invalidate,
  });
}

export function useEndAlertMuteRule(): UseMutationResult<AlertMuteRule, Error, string> {
  const api = useBackendApi();
  const invalidate = useInvalidate();
  return useMutation<AlertMuteRule, Error, string>({
    mutationFn: (id) => api.postEmpty<AlertMuteRule>(path(id, "/end")),
    onSuccess: invalidate,
  });
}

export function useDeleteAlertMuteRule(): UseMutationResult<void, Error, string> {
  const api = useBackendApi();
  const invalidate = useInvalidate();
  return useMutation<void, Error, string>({
    mutationFn: (id) => api.delete(path(id)),
    onSuccess: invalidate,
  });
}

/**
 * The values real alerts carry in `field` that contain `query`, most recently
 * seen first (`GET /alert-field-values`, read from the alerting service). A
 * rule matches these strings, so the form suggests them; when the alerting
 * service can't be reached the field stays free text.
 */
export function useAlertFieldValues(
  field: AlertMuteRuleField,
  query: string,
  enabled: boolean,
): UseQueryResult<string[], Error> {
  const api = useBackendApi();
  return useQuery<string[], Error>({
    queryKey: [ApiQueryKeys.ALERT_MUTE_RULES, "field-values", field, query],
    queryFn: async () => {
      const params = new URLSearchParams({ field, q: query, limit: "20" });
      const res = await api.get<AlertFieldValuesResponse>(`/alert-field-values?${params.toString()}`);
      return res?.values ?? [];
    },
    enabled,
    staleTime: 60_000,
    retry: false,
    placeholderData: keepPreviousData,
  });
}
