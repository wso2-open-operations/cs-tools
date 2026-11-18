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

import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const authFetchMock = vi.fn();
vi.mock("@hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

import { useBackendApi } from "@api/backend/client";

describe("useBackendApi post headers", () => {
  beforeEach(() => {
    authFetchMock.mockReset();
    authFetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    });
    window.config = { CSM_PORTAL_BACKEND_BASE_URL: "https://be.test" } as typeof window.config;
  });

  it("sends Content-Type, the correlation id and extra headers, and lets none of the first two be overridden", async () => {
    const { result } = renderHook(() => useBackendApi());
    await result.current.post("/x", { a: 1 }, {
      headers: {
        "Idempotency-Key": "k1",
        "content-type": "text/plain",
        "x-csm-correlation-id": "spoofed",
      },
    });
    const [url, init] = authFetchMock.mock.calls[0];
    expect(url).toBe("https://be.test/x");
    const headers = init.headers as Record<string, string>;
    expect(headers["Idempotency-Key"]).toBe("k1");
    expect(headers["Content-Type"]).toBe("application/json");
    expect(headers["X-CSM-Correlation-ID"]).toBeTruthy();
    expect(headers["X-CSM-Correlation-ID"]).not.toBe("spoofed");
    expect(Object.keys(headers).map((h) => h.toLowerCase()).sort()).toEqual([
      "content-type",
      "idempotency-key",
      "x-csm-correlation-id",
    ]);
  });
});
