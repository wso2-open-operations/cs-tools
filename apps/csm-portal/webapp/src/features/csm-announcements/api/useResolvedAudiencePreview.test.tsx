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

import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AUDIENCE_PREVIEW_MAX_PROJECTS,
  useResolvedAudiencePreview,
} from "@features/csm-announcements/api/useResolvedAudiencePreview";

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

const authFetchMock = vi.fn();
vi.mock("@hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

function projectResponse(id: string) {
  return {
    ok: true,
    status: 200,
    json: async () => ({ id, name: `Project ${id}`, key: id.toUpperCase(), account: { name: "Acme" } }),
  };
}

describe("useResolvedAudiencePreview", () => {
  beforeEach(() => {
    authFetchMock.mockReset();
  });

  it("resolves every id to a name/key via GET /projects/{id}", async () => {
    authFetchMock.mockImplementation((url: string) => {
      const id = decodeURIComponent(url.split("/").pop() as string);
      return Promise.resolve(projectResponse(id));
    });

    const { result } = renderHook(() => useResolvedAudiencePreview());
    await act(async () => {
      await result.current.resolve(["p-1", "p-2"]);
    });

    expect(result.current.projects).toEqual([
      { id: "p-1", name: "Project p-1", key: "P-1", accountName: "Acme" },
      { id: "p-2", name: "Project p-2", key: "P-2", accountName: "Acme" },
    ]);
    expect(result.current.total).toBe(2);
    expect(result.current.truncated).toBe(false);
    expect(result.current.isError).toBe(false);
  });

  it("skips a 404'd project (deleted since the audience was frozen) instead of failing the whole preview", async () => {
    authFetchMock.mockImplementation((url: string) => {
      const id = decodeURIComponent(url.split("/").pop() as string);
      if (id === "p-2") return Promise.resolve({ ok: false, status: 404 });
      return Promise.resolve(projectResponse(id));
    });

    const { result } = renderHook(() => useResolvedAudiencePreview());
    await act(async () => {
      await result.current.resolve(["p-1", "p-2"]);
    });

    expect(result.current.projects.map((p) => p.id)).toEqual(["p-1"]);
    expect(result.current.isError).toBe(false);
  });

  it("flags isError only when every project fails to resolve", async () => {
    authFetchMock.mockResolvedValue({ ok: false, status: 500 });

    const { result } = renderHook(() => useResolvedAudiencePreview());
    await act(async () => {
      await result.current.resolve(["p-1", "p-2"]);
    });

    expect(result.current.projects).toEqual([]);
    expect(result.current.isError).toBe(true);
  });

  it("caps the number of individual lookups at AUDIENCE_PREVIEW_MAX_PROJECTS but reports the true total", async () => {
    authFetchMock.mockImplementation((url: string) => {
      const id = decodeURIComponent(url.split("/").pop() as string);
      return Promise.resolve(projectResponse(id));
    });
    const manyIds = Array.from({ length: AUDIENCE_PREVIEW_MAX_PROJECTS + 25 }, (_, i) => `p-${i}`);

    const { result } = renderHook(() => useResolvedAudiencePreview());
    await act(async () => {
      await result.current.resolve(manyIds);
    });

    expect(authFetchMock).toHaveBeenCalledTimes(AUDIENCE_PREVIEW_MAX_PROJECTS);
    expect(result.current.projects).toHaveLength(AUDIENCE_PREVIEW_MAX_PROJECTS);
    expect(result.current.total).toBe(manyIds.length);
    expect(result.current.truncated).toBe(true);
  });

  it("sets isLoading true while resolving and false once settled", async () => {
    let resolveFetch: (v: unknown) => void = () => {};
    authFetchMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveFetch = resolve;
        }),
    );

    const { result } = renderHook(() => useResolvedAudiencePreview());
    let resolvePromise!: Promise<void>;
    act(() => {
      resolvePromise = result.current.resolve(["p-1"]);
    });
    expect(result.current.isLoading).toBe(true);

    await act(async () => {
      resolveFetch(projectResponse("p-1"));
      await resolvePromise;
    });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
  });
});
