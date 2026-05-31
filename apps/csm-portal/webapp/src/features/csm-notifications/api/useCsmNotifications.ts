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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import {
  getMockCsmNotifications,
  markAllMockNotificationsRead,
  markMockNotificationRead,
} from "@features/csm-notifications/api/mocks/notificationsMocks";
import type { CsmNotification } from "@features/csm-notifications/types/csmNotifications";

const MOCK_LATENCY_MS = 120;

export function useGetCsmNotifications(): UseQueryResult<
  CsmNotification[],
  Error
> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmNotification[], Error>({
    queryKey: [ApiQueryKeys.CSM_NOTIFICATIONS],
    queryFn: async (): Promise<CsmNotification[]> => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug("[useGetCsmNotifications] mock");
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmNotifications();
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/notifications`;
      const response = await authFetch(url, { method: "GET" });
      if (!response.ok) {
        throw new Error(
          `Error fetching notifications: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmNotification[];
    },
    staleTime: 30_000,
  });
}

export function useMarkNotificationRead(): UseMutationResult<
  void,
  Error,
  string
> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: async (id) => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(`[useMarkNotificationRead] mock id=${id}`);
        markMockNotificationRead(id);
        return;
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/notifications/${encodeURIComponent(id)}/read`;
      const response = await authFetch(url, { method: "POST" });
      if (!response.ok) {
        throw new Error(`Failed to mark read: ${response.statusText}`);
      }
    },
    onSuccess: (_void, id) => {
      queryClient.setQueryData<CsmNotification[] | undefined>(
        [ApiQueryKeys.CSM_NOTIFICATIONS],
        (prev) =>
          prev?.map((n) => (n.id === id ? { ...n, isRead: true } : n)),
      );
    },
  });
}

export function useMarkAllNotificationsRead(): UseMutationResult<
  void,
  Error,
  void
> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();

  return useMutation<void, Error, void>({
    mutationFn: async () => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug("[useMarkAllNotificationsRead] mock");
        markAllMockNotificationsRead();
        return;
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/notifications/read-all`;
      const response = await authFetch(url, { method: "POST" });
      if (!response.ok) {
        throw new Error(`Failed to mark all read: ${response.statusText}`);
      }
    },
    onSuccess: () => {
      queryClient.setQueryData<CsmNotification[] | undefined>(
        [ApiQueryKeys.CSM_NOTIFICATIONS],
        (prev) => prev?.map((n) => ({ ...n, isRead: true })),
      );
    },
  });
}
