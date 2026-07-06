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
import { fetchProjectCaseSearchResults } from "@features/support/api/fetchProjectCaseSearchResults";
import { SortOrder } from "@/types/common";

describe("fetchProjectCaseSearchResults", () => {
  it("should request subsequent pages until totalRecords are fetched", async () => {
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    });

    const authFetch = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            cases: [{ id: "case-1" }],
            totalRecords: 2,
            limit: 1,
            offset: 0,
          }),
      } as Response)
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            cases: [{ id: "case-2" }],
            totalRecords: 2,
            limit: 1,
            offset: 1,
          }),
      } as Response);

    const data = await fetchProjectCaseSearchResults(
      authFetch,
      "project-1",
      { filters: {}, sortBy: { field: "updatedOn", order: SortOrder.DESC } },
      1,
    );

    expect(data).toEqual([{ id: "case-1" }, { id: "case-2" }]);
    expect(authFetch).toHaveBeenNthCalledWith(
      1,
      "https://api.test/projects/project-1/cases/search",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          filters: {},
          sortBy: { field: "updatedOn", order: SortOrder.DESC },
          pagination: { offset: 0, limit: 1 },
        }),
      }),
    );
    expect(authFetch).toHaveBeenNthCalledWith(
      2,
      "https://api.test/projects/project-1/cases/search",
      expect.objectContaining({
        body: JSON.stringify({
          filters: {},
          sortBy: { field: "updatedOn", order: SortOrder.DESC },
          pagination: { offset: 1, limit: 1 },
        }),
      }),
    );
  });
});
