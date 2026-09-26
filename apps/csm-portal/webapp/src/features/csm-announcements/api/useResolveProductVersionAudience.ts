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
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeProjectsByProductVersionSearchPayload,
  BeProjectsByProductVersionSearchResponse,
} from "@api/backend/types";
import type { ResolvedAudienceProject } from "@features/csm-announcements/api/useResolveAnnouncementAudience";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Safety bound against a wrong/always-true `hasMore`, same convention as
 * {@link useResolveAnnouncementAudience}'s own paged loop.
 */
const MAX_AUDIENCE_PAGES = 100;

export interface ResolvedProductVersionAudience {
  projects: ResolvedAudienceProject[];
  /** The definitive resolved count — every matching project has been fetched, not estimated from one page. */
  total: number;
  isLoading: boolean;
  isError: boolean;
}

/**
 * Resolves the EOL/product-version announcement flow's audience: pages
 * through every match of `POST /deployed-products/projects/search` for the
 * given product + version. Unlike {@link useResolveAnnouncementAudience},
 * there are no caller-supplied exclusion filters here — the endpoint always
 * excludes Restricted/Suspended projects and Cloud Support/Cloud Evaluation
 * Support subscriptions itself (mirroring the real ServiceNow flow this
 * replaces), so this hook has nothing to pass beyond the product/version.
 *
 * Disabled until both `productId` and `productVersionId` are chosen.
 */
export function useResolveProductVersionAudience(
  productId: string | undefined,
  productVersionId: string | undefined,
): ResolvedProductVersionAudience {
  const api = useBackendApi();

  const query: UseQueryResult<ResolvedAudienceProject[], Error> = useQuery({
    queryKey: [ApiQueryKeys.PROJECTS_BY_PRODUCT_VERSION, productId ?? "", productVersionId ?? ""],
    queryFn: async (): Promise<ResolvedAudienceProject[]> => {
      const collected: ResolvedAudienceProject[] = [];
      let offset = 0;

      for (let page = 0; ; page++) {
        // Fail loudly rather than truncate — same reasoning as
        // useResolveAnnouncementAudience's own paged loop: silently
        // returning `collected` here would report an incomplete EOL
        // audience as the complete, definitive one.
        if (page === MAX_AUDIENCE_PAGES) {
          throw new Error(
            `Too many matching projects to resolve the audience safely (exceeded ${MAX_AUDIENCE_PAGES} pages of ${PAGE_LIMIT}).`,
          );
        }
        const res = await api.post<
          BeProjectsByProductVersionSearchPayload,
          BeProjectsByProductVersionSearchResponse
        >("/deployed-products/projects/search", {
          pagination: { offset, limit: PAGE_LIMIT },
          productId: productId as string,
          productVersionId: productVersionId as string,
        });
        const page_ = res.projects ?? [];
        for (const p of page_) {
          collected.push({ id: p.id, name: p.name ?? p.id });
        }
        if (!res.hasMore || page_.length === 0) break;
        offset += page_.length;
      }

      return collected;
    },
    enabled: !!productId && !!productVersionId,
    staleTime: 30_000,
  });

  return {
    projects: query.data ?? [],
    total: query.data?.length ?? 0,
    isLoading: query.isFetching,
    isError: query.isError,
  };
}
