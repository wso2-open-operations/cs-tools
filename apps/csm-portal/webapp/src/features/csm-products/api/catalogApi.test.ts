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


import { describe, expect, it, vi } from "vitest";
import type { BackendApi } from "@api/backend/client";
import { fetchAllPages } from "@features/csm-products/api/fetchAllPages";
import { catalogWriteRoute } from "@features/csm-products/api/catalogWriteRoute";

describe("catalogWriteRoute", () => {
  it("sends each write to its products API route", () => {
    expect(catalogWriteRoute({ kind: "createProduct", payload: { name: "X" } })).toEqual({
      method: "POST",
      path: "/products",
      body: { name: "X" },
    });
    expect(catalogWriteRoute({ kind: "updateVersion", id: "v1", payload: { supportEolDate: "2030-01-31" } })).toEqual({
      method: "PATCH",
      path: "/product-versions/v1",
      body: { supportEolDate: "2030-01-31" },
    });
    expect(catalogWriteRoute({ kind: "retireVersion", id: "v1" })).toEqual({ method: "POST", path: "/product-versions/v1/retire" });
    expect(catalogWriteRoute({ kind: "createReleaseLine", payload: { name: "L" } }).path).toBe("/release-lines");
    expect(catalogWriteRoute({ kind: "updateReleaseLine", id: "l1", payload: {} })).toMatchObject({
      method: "PATCH",
      path: "/release-lines/l1",
    });
    expect(catalogWriteRoute({ kind: "retireReleaseLine", id: "l1" })).toEqual({ method: "POST", path: "/release-lines/l1/retire" });
  });

  it("escapes the id", () => {
    expect(catalogWriteRoute({ kind: "retireVersion", id: "a/b" }).path).toBe("/product-versions/a%2Fb/retire");
  });
});

describe("fetchAllPages", () => {
  it("reads pages until hasMore is false", async () => {
    const post = vi
      .fn()
      .mockResolvedValueOnce({ rows: [1, 2], total: 3, hasMore: true })
      .mockResolvedValueOnce({ rows: [3], total: 3, hasMore: false });
    const api = { post } as unknown as BackendApi;
    await expect(fetchAllPages<number>(api, "/x/search", { a: 1 }, "rows", 2)).resolves.toEqual([1, 2, 3]);
    expect(post).toHaveBeenNthCalledWith(1, "/x/search", { a: 1, pagination: { limit: 2, offset: 0 } });
    expect(post).toHaveBeenNthCalledWith(2, "/x/search", { a: 1, pagination: { limit: 2, offset: 2 } });
  });

  it("stops on an empty answer", async () => {
    const api = { post: vi.fn().mockResolvedValue(null) } as unknown as BackendApi;
    await expect(fetchAllPages(api, "/x/search", {}, "rows", 10)).resolves.toEqual([]);
  });
});
