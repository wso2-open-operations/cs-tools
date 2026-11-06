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

export type BannerStatus = "draft" | "published" | "unpublished";

export interface PortalBanner {
  id: string;
  status: BannerStatus;
  sortOrder: number;
  visibleInCustomerPortal: boolean;
  visibleInCsmPortal: boolean;
  html: string;
  startsAt: string | null;
  expiresAt: string | null;
}

export interface SavePortalBanner {
  status: BannerStatus;
  sortOrder: number;
  visibleInCustomerPortal: boolean;
  visibleInCsmPortal: boolean;
  html: string;
  startsAt: string | null;
  expiresAt: string | null;
}

const BANNERS_KEY = ["admin", "portal-banners"] as const;
// Same key as the CSM shell display query, so a save refreshes banners already on screen.
const DISPLAY_KEY = ["portal-banners"] as const;

interface BannerList {
  banners: PortalBanner[];
}

/** Every banner, including draft and unpublished, for the Settings page. */
export function usePortalBanners(): UseQueryResult<PortalBanner[], Error> {
  const api = useBackendApi();
  return useQuery<PortalBanner[], Error>({
    queryKey: BANNERS_KEY,
    queryFn: async () => (await api.get<BannerList>("/portal-banners/manage"))?.banners ?? [],
  });
}

function useBannerMutation<TVars>(
  fn: (vars: TVars) => Promise<PortalBanner>,
): UseMutationResult<PortalBanner, Error, TVars> {
  const qc = useQueryClient();
  return useMutation<PortalBanner, Error, TVars>({
    mutationFn: fn,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: BANNERS_KEY });
      void qc.invalidateQueries({ queryKey: DISPLAY_KEY });
    },
  });
}

export function useCreatePortalBanner(): UseMutationResult<PortalBanner, Error, SavePortalBanner> {
  const api = useBackendApi();
  return useBannerMutation((input) => api.post<SavePortalBanner, PortalBanner>("/portal-banners", input));
}

export function useUpdatePortalBanner(): UseMutationResult<PortalBanner, Error, { id: string; input: SavePortalBanner }> {
  const api = useBackendApi();
  return useBannerMutation(({ id, input }) =>
    api.put<SavePortalBanner, PortalBanner>(`/portal-banners/${encodeURIComponent(id)}`, input),
  );
}
