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

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useParallelPostApi } from "@features/spl/usage-metrics/api/useUsageMetricsApi";

describe("useParallelPostApi — per-item failures", () => {
  beforeEach(() => postMock.mockReset());

  it("records a failed item in failedIds instead of treating it as empty data", async () => {
    postMock.mockImplementation((_url: string, payload?: { id: string }) => {
      return payload?.id === "b" ? Promise.reject(new Error("500")) : Promise.resolve({ ok: payload?.id });
    });
    const { result } = renderHook(() => useParallelPostApi<{ ok: string }>());

    await act(async () => {
      await result.current.postAll(
        [
          { id: "a", payload: { id: "a" } },
          { id: "b", payload: { id: "b" } },
        ],
        "/x",
      );
    });

    expect([...result.current.dataMap.keys()]).toEqual(["a"]);
    expect([...result.current.failedIds]).toEqual(["b"]);
  });

  it("clears a failure once a later merge call succeeds for that item", async () => {
    let calls = 0;
    postMock.mockImplementation(() =>
      ++calls === 1 ? Promise.reject(new Error("500")) : Promise.resolve({ ok: "b" }),
    );
    const { result } = renderHook(() => useParallelPostApi<{ ok: string }>());
    const items = [{ id: "b", payload: { id: "b" } }];

    await act(async () => {
      await result.current.postAll(items, "/x", true);
    });
    expect(result.current.failedIds.has("b")).toBe(true);

    await act(async () => {
      await result.current.postAll(items, "/x", true);
    });
    expect(result.current.failedIds.has("b")).toBe(false);
    expect(result.current.dataMap.get("b")).toEqual({ ok: "b" });
  });
});
