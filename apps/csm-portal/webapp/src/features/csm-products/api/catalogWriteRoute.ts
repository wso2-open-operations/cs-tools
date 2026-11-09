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


import type { CatalogChangePayload } from "@features/csm-products/types/productCatalog";

/** One catalogue write. Each maps to one route of the products API. */
export type CatalogWrite =
  | { kind: "createProduct"; payload: CatalogChangePayload }
  | { kind: "updateVersion"; id: string; payload: CatalogChangePayload }
  | { kind: "retireVersion"; id: string }
  | { kind: "createReleaseLine"; payload: CatalogChangePayload }
  | { kind: "updateReleaseLine"; id: string; payload: CatalogChangePayload }
  | { kind: "retireReleaseLine"; id: string };

/** The route a write goes to. Retires take no body. */
export function catalogWriteRoute(write: CatalogWrite): { method: "POST" | "PATCH"; path: string; body?: CatalogChangePayload } {
  switch (write.kind) {
    case "createProduct":
      return { method: "POST", path: "/products", body: write.payload };
    case "updateVersion":
      return { method: "PATCH", path: `/product-versions/${encodeURIComponent(write.id)}`, body: write.payload };
    case "retireVersion":
      return { method: "POST", path: `/product-versions/${encodeURIComponent(write.id)}/retire` };
    case "createReleaseLine":
      return { method: "POST", path: "/release-lines", body: write.payload };
    case "updateReleaseLine":
      return { method: "PATCH", path: `/release-lines/${encodeURIComponent(write.id)}`, body: write.payload };
    case "retireReleaseLine":
      return { method: "POST", path: `/release-lines/${encodeURIComponent(write.id)}/retire` };
  }
}
