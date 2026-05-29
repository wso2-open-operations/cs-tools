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

import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import {
  refreshCaseQueriesAfterCreation,
  triggerPostCreationApiCalls,
} from "@features/operations/utils/caseRefresh";
import { CaseType } from "@features/support/constants/supportConstants";
import { ApiQueryKeys } from "@constants/apiConstants";

describe("caseRefresh", () => {
  it("triggerPostCreationApiCalls hits stats and search endpoints", async () => {
    (
      window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" };

    const authFetch = vi.fn().mockResolvedValue({ ok: true });
    await triggerPostCreationApiCalls(authFetch, "proj-1", CaseType.SERVICE_REQUEST);

    expect(authFetch).toHaveBeenCalledTimes(3);
    expect(authFetch.mock.calls[2][0]).toContain("/cases/search");
  });

  it("refreshCaseQueriesAfterCreation invalidates case-related queries", async () => {
    const queryClient = new QueryClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const refetchSpy = vi.spyOn(queryClient, "refetchQueries");

    await refreshCaseQueriesAfterCreation(
      queryClient,
      "proj-1",
      CaseType.DEFAULT_CASE,
    );

    expect(invalidateSpy).toHaveBeenCalled();
    expect(refetchSpy).toHaveBeenCalledWith({
      queryKey: [ApiQueryKeys.CASES_STATS, "proj-1"],
    });
  });
});
