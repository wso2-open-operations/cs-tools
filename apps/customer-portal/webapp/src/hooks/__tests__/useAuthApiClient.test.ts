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
import { useAuthApiClient } from "@hooks/useAuthApiClient";

const getIdTokenMock = vi.fn();

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ getIdToken: getIdTokenMock }),
}));

describe("useAuthApiClient", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getIdTokenMock.mockResolvedValue("token-abc");
    globalThis.fetch = vi.fn().mockResolvedValue({ ok: true }) as typeof fetch;
  });

  it("adds bearer and JSON content-type for POST bodies", async () => {
    const { result } = renderHook(() => useAuthApiClient());
    await result.current("https://api.test/resource", {
      method: "POST",
      body: JSON.stringify({ ok: true }),
    });

    expect(getIdTokenMock).toHaveBeenCalled();
    expect(globalThis.fetch).toHaveBeenCalledWith(
      "https://api.test/resource",
      expect.objectContaining({
        method: "POST",
        headers: expect.any(Headers),
      }),
    );

    const headers = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1]
      .headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer token-abc");
    expect(headers.get("Content-Type")).toBe("application/json");
  });

  it("throws when ID token is missing", async () => {
    getIdTokenMock.mockResolvedValueOnce(undefined);
    const { result } = renderHook(() => useAuthApiClient());
    await expect(result.current("https://api.test")).rejects.toThrow(
      "Unable to retrieve ID token",
    );
  });
});
