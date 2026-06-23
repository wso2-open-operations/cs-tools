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
import { ASGARDEO_UNAUTHENTICATED_CODE } from "@constants/apiConstants";

const getIdTokenMock = vi.fn();
const signInMock = vi.fn();

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: getIdTokenMock,
    signIn: signInMock,
  }),
}));

const unauthenticatedError = () =>
  Object.assign(new Error("unauthenticated"), {
    code: ASGARDEO_UNAUTHENTICATED_CODE,
  });

describe("useAuthApiClient", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getIdTokenMock.mockResolvedValue("token-abc");
    signInMock.mockResolvedValue(undefined);
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

  it("rethrows non-auth errors without attempting recovery", async () => {
    getIdTokenMock.mockRejectedValueOnce(new Error("network down"));
    const { result } = renderHook(() => useAuthApiClient());

    await expect(result.current("https://api.test")).rejects.toThrow(
      "network down",
    );
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("retries once and recovers when the token is refreshed between attempts", async () => {
    getIdTokenMock
      .mockRejectedValueOnce(unauthenticatedError())
      .mockResolvedValue("token-fresh");

    const { result } = renderHook(() => useAuthApiClient());
    await result.current("https://api.test/resource");

    expect(getIdTokenMock).toHaveBeenCalledTimes(2);
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("redirects to full sign-in when still unauthenticated after the retry", async () => {
    getIdTokenMock.mockRejectedValue(unauthenticatedError());

    const { result } = renderHook(() => useAuthApiClient());
    // The redirect path returns a never-resolving promise while navigating away,
    // so assert on the side effects rather than awaiting the call.
    void result.current("https://api.test/resource");
    await vi.waitFor(() => expect(signInMock).toHaveBeenCalledTimes(1));

    expect(getIdTokenMock).toHaveBeenCalledTimes(2);
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });
});
