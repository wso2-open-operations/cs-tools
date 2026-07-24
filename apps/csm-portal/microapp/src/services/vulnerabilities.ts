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

import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import { PRODUCT_VULNERABILITIES_SEARCH_ENDPOINT, PRODUCT_VULNERABILITY_ENDPOINT } from "@config/endpoints";
import type { VulnerabilityDto, VulnerabilityPriority, VulnerabilitySearchResponseDto } from "@src/types";
import { toVulnerability, type Vulnerability } from "@src/types";
import apiClient from "./apiClient";

export interface VulnerabilityFilters {
  search: string;
  priority: VulnerabilityPriority | null;
}

export const EMPTY_VULNERABILITY_FILTERS: VulnerabilityFilters = { search: "", priority: null };

export interface VulnerabilitySearchResult {
  items: Vulnerability[];
  total: number;
  offset: number;
  limit: number;
  hasMore: boolean;
}

// openapi.yaml declares this endpoint's pagination.limit maximum as 100, but every other search
// endpoint on this backend that claims the same (comments, products, deployments) actually
// rejects it with a 400 — see services/cases.ts's COMMENTS_PAGE_LIMIT and
// services/deployments.ts's SEARCH_PAGE_LIMIT for the confirmed cases. Match that real, working
// value defensively here too rather than waiting to hit the same 400 live.
const VULNERABILITIES_PAGE_LIMIT = 50;

async function searchVulnerabilities(
  filters: VulnerabilityFilters,
  offset: number,
): Promise<VulnerabilitySearchResult> {
  const q = filters.search.trim();
  const { data } = await apiClient.post<VulnerabilitySearchResponseDto>(PRODUCT_VULNERABILITIES_SEARCH_ENDPOINT, {
    pagination: { offset, limit: VULNERABILITIES_PAGE_LIMIT },
    filters: {
      ...(q ? { searchQuery: q } : {}),
      ...(filters.priority ? { priority: filters.priority } : {}),
    },
  });
  const items = data.productVulnerabilities.map(toVulnerability);
  return {
    items,
    total: data.total,
    offset: data.offset,
    limit: data.limit,
    // The search response carries no hasMore field at all (unlike every other search endpoint in
    // this app), so this is always derived, with the same empty-page guard used elsewhere against
    // looping forever on a stale/inconsistent total.
    hasMore: items.length > 0 && data.offset + items.length < data.total,
  };
}

const getVulnerability = async (id: string): Promise<Vulnerability> => {
  const { data } = await apiClient.get<VulnerabilityDto>(PRODUCT_VULNERABILITY_ENDPOINT(id));
  return toVulnerability(data);
};

export const vulnerabilities = {
  infinite: (filters: VulnerabilityFilters) =>
    infiniteQueryOptions({
      queryKey: ["vulnerabilities", "infinite", filters],
      queryFn: ({ pageParam }) => searchVulnerabilities(filters, pageParam),
      initialPageParam: 0,
      getNextPageParam: (last) => (last.hasMore ? last.offset + last.limit : undefined),
    }),

  get: (id: string) =>
    queryOptions({
      queryKey: ["vulnerability", id],
      queryFn: () => getVulnerability(id),
    }),
};
