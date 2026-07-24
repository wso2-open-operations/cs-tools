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

import { queryOptions } from "@tanstack/react-query";
import { PRODUCTS_SEARCH_ENDPOINT } from "@config/endpoints";
import apiClient from "./apiClient";

interface ProductDto {
  id: string;
  name?: string;
}

interface ProductSearchResponseDto {
  products: ProductDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore?: boolean;
}

// openapi.yaml declares this endpoint's pagination.limit maximum as 100, but
// services/cases.ts documents that the live upstream entity service actually
// rejects 100 with a 400 for the (same-shaped) comments-search endpoint; match
// that verified-working value instead of the doc.
const PRODUCTS_PAGE_LIMIT = 50;

/**
 * Distinct product **family names** for the engagements product filter.
 * `POST /products/search` returns one row per product model/version (the same
 * short `name` repeated many times, e.g. dozens of "API Manager" variants), so
 * this fetches the full (bounded) catalogue once and reduces it to the set of
 * distinct, non-empty names, sorted for a stable dropdown order — mirrors the
 * webapp's useProductNameOptions.ts.
 */
async function fetchProductNames(): Promise<string[]> {
  const names = new Set<string>();
  for (let offset = 0; ; offset += PRODUCTS_PAGE_LIMIT) {
    const { data } = await apiClient.post<ProductSearchResponseDto>(PRODUCTS_SEARCH_ENDPOINT, {
      pagination: { offset, limit: PRODUCTS_PAGE_LIMIT },
    });
    for (const p of data.products) {
      const name = p.name?.trim();
      if (name) names.add(name);
    }
    if (data.products.length < PRODUCTS_PAGE_LIMIT) break;
  }
  return [...names].sort((a, b) => a.localeCompare(b));
}

export const products = {
  names: () =>
    queryOptions({
      queryKey: ["products", "family-names"],
      queryFn: fetchProductNames,
      staleTime: 5 * 60_000,
    }),
};
