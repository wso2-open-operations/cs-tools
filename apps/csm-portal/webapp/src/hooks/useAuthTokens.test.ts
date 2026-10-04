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

import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getAccessTokenMock = vi.fn();
const getIdTokenMock = vi.fn();
const signInMock = vi.fn();
const signInSilentlyMock = vi.fn();
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getAccessToken: getAccessTokenMock,
    getIdToken: getIdTokenMock,
    signIn: signInMock,
    signInSilently: signInSilentlyMock,
  }),
}));

const debugMock = vi.fn();
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: debugMock,
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

const { useAuthTokens } = await import("./useAuthTokens");

// Matches IDP_UNAUTHENTICATED_CODE — a dead/expired refresh token.
const tokenExpiredError = { code: "SPA-AUTH_CLIENT-VM-IV02" };

describe("useAuthTokens", () => {
  beforeEach(() => {
    getAccessTokenMock.mockReset().mockResolvedValue("access-token");
    getIdTokenMock.mockReset().mockResolvedValue("id-token");
    signInMock.mockReset().mockResolvedValue(undefined);
    signInSilentlyMock.mockReset().mockResolvedValue(undefined);
    debugMock.mockReset();
  });

  it("resolves token and idToken on the happy path", async () => {
    const { result } = renderHook(() => useAuthTokens());
    await expect(result.current()).resolves.toEqual({
      token: "access-token",
      idToken: "id-token",
    });
    expect(signInMock).not.toHaveBeenCalled();
    expect(signInSilentlyMock).not.toHaveBeenCalled();
  });

  it("normalises an SDK-not-initialized race into the shared auth-not-ready message, without retrying", async () => {
    getAccessTokenMock.mockRejectedValue(
      Object.assign(new Error("The SDK must be initialized first"), {
        code: "SPA-AUTH_CLIENT-VM-NF01",
      }),
    );
    const { result } = renderHook(() => useAuthTokens());
    await expect(result.current()).rejects.toThrow("Authentication is not ready yet");
    expect(getAccessTokenMock).toHaveBeenCalledTimes(1);
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("propagates a non-token error immediately, without retrying or redirecting", async () => {
    getAccessTokenMock.mockRejectedValue(new Error("network down"));
    const { result } = renderHook(() => useAuthTokens());
    await expect(result.current()).rejects.toThrow("network down");
    expect(getAccessTokenMock).toHaveBeenCalledTimes(1);
    expect(signInMock).not.toHaveBeenCalled();
    expect(signInSilentlyMock).not.toHaveBeenCalled();
  });

  it("retries once on a token-expired error and succeeds if a concurrent refresh already fixed it", async () => {
    getAccessTokenMock
      .mockRejectedValueOnce(tokenExpiredError)
      .mockResolvedValue("access-token");
    const { result } = renderHook(() => useAuthTokens());
    await expect(result.current()).resolves.toEqual({
      token: "access-token",
      idToken: "id-token",
    });
    expect(getAccessTokenMock).toHaveBeenCalledTimes(2);
    expect(signInSilentlyMock).not.toHaveBeenCalled();
  });

  it("falls back to a silent re-auth after two consecutive token-expired failures, and succeeds if that revives the session", async () => {
    getAccessTokenMock
      .mockRejectedValueOnce(tokenExpiredError)
      .mockRejectedValueOnce(tokenExpiredError)
      .mockResolvedValue("access-token");
    signInSilentlyMock.mockResolvedValue({ idToken: "id-token" });

    const { result } = renderHook(() => useAuthTokens());
    await expect(result.current()).resolves.toEqual({
      token: "access-token",
      idToken: "id-token",
    });
    expect(signInSilentlyMock).toHaveBeenCalledTimes(1);
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("redirects to a full sign-in when silent re-auth fails, and never resolves", async () => {
    getAccessTokenMock.mockRejectedValue(tokenExpiredError);
    signInSilentlyMock.mockResolvedValue(false);

    const { result } = renderHook(() => useAuthTokens());
    const pending = result.current();

    await waitFor(() => expect(signInMock).toHaveBeenCalledTimes(1));

    let settled = false;
    void pending.then(() => {
      settled = true;
    });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(settled).toBe(false);
  });

  it("single-flights concurrent silent re-auth attempts across callers", async () => {
    getAccessTokenMock.mockRejectedValue(tokenExpiredError);
    let resolveSilent: (value: boolean) => void = () => {};
    signInSilentlyMock.mockReturnValue(
      new Promise((resolve) => {
        resolveSilent = resolve;
      }),
    );

    const { result: a } = renderHook(() => useAuthTokens());
    const { result: b } = renderHook(() => useAuthTokens());

    const pendingA = a.current();
    const pendingB = b.current();

    await waitFor(() => expect(signInSilentlyMock).toHaveBeenCalledTimes(1));

    resolveSilent(false);
    await waitFor(() => expect(signInMock).toHaveBeenCalledTimes(1));

    void pendingA;
    void pendingB;
  });
});
