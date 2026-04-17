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

import type { PaginationResponse, SearchRequestBase } from "@/types/common";

// Item type for a product from GET /products.
export type ProductItem = {
  id: string;
  label?: string;
  name?: string;
  class?: string;
};

// Item type for a product version from POST /products/:productId/versions/search.
export type ProductVersionItem = {
  id: string;
  version: string;
  currentSupportStatus?: string | null;
  releaseDate?: string;
  supportEolDate?: string;
  earliestPossibleSupportEolDate?: string;
  product?: { id: string; label: string };
};

// Response type for products list.
export type ProductsResponse = PaginationResponse & {
  products: ProductItem[];
};

// Response type for product versions search results.
export type ProductVersionsSearchResponse = PaginationResponse & {
  versions: ProductVersionItem[];
};

// Model type for a product update.
export type ProductUpdate = {
  updateLevel: number;
  date: string;
  details?: string | null;
};

// Request type for searching product versions.
export type ProductVersionsSearchRequest = SearchRequestBase;
