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

import { beforeEach, describe, expect, it, vi } from "vitest";
import { fetchChangeRequestSearchResults } from "@features/operations/api/fetchChangeRequestSearchResults";
import { SortOrder } from "@/types/common";
import { ChangeRequestSortField } from "@features/operations/types/changeRequests";

describe("fetchChangeRequestSearchResults", () => {
  beforeEach(() => {
    (
      window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" };
  });

  it("aggregates paginated change request search results", async () => {
    const authFetch = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          changeRequests: [{ id: "1", number: "CHG1" }],
          totalRecords: 2,
          offset: 0,
          limit: 1,
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          changeRequests: [{ id: "2", number: "CHG2" }],
          totalRecords: 2,
          offset: 1,
          limit: 1,
        }),
      });

    const results = await fetchChangeRequestSearchResults(
      authFetch,
      "proj-1",
      {
        sortBy: { field: ChangeRequestSortField.CreatedOn, order: SortOrder.DESC },
      },
      1,
    );

    expect(results).toHaveLength(2);
    expect(authFetch).toHaveBeenCalledTimes(2);
  });

  it("throws when response is not ok", async () => {
    const authFetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: "Server Error",
      text: async () => "failed",
    });

    await expect(
      fetchChangeRequestSearchResults(authFetch, "proj-1", {
        sortBy: { field: ChangeRequestSortField.CreatedOn, order: SortOrder.DESC },
      }),
    ).rejects.toThrow("failed");
  });
});
