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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import {
  clearPersistedRecentApprovers,
  invalidateTimecards,
  mapTimeCard,
  searchTimeCards,
  useBulkApproveCards,
  useRecentApprovers,
} from "@features/csm-timecards/api/useTimeSheets";
import { BackendApiError, type BackendApi } from "@api/backend/client";
import type {
  BeSearchTimeCardsPayload,
  BeSearchTimeCardsResponse,
  BeTimeCardState,
  BeTimeCardView,
} from "@api/backend/types";

// useTimeSheets.ts pulls in @config/apiConfig transitively (via useGetUsersMe)
// and @api/backend/client directly; both read window.config at module load
// and throw outside a configured runtime. Mock both so importing
// searchTimeCards below doesn't trip either (vi.mock calls are hoisted above
// these imports by vitest). searchTimeCards takes its own BackendApi instance
// directly, so neither the hook nor the real config is ever exercised by
// those tests — only the useBulkApproveCards tests further down actually
// exercise this mocked patch.
const patchMock = vi.fn();
const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ patch: patchMock, post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

// useRecentApprovers resolves the signed-in engineer via useCurrentEngineer,
// which pulls GET /users/me and the ID-token claims — mocked so tests can
// control `me.id` directly without a real auth/query round trip.
let usersMeId: string | undefined = "eng-1";
vi.mock("@features/settings/api/useGetUsersMe", () => ({
  useGetUsersMe: () => ({ data: usersMeId ? { id: usersMeId, firstName: "Jane" } : undefined }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({ useIdTokenClaims: () => undefined }));

function beCard(id: string, state: BeTimeCardState): BeTimeCardView {
  return {
    id,
    totalTime: 30,
    workDate: "2026-07-13",
    createdOn: "2026-07-13",
    hasBillable: true,
    state,
    user: { id: "user-1", name: "Jane Doe" },
    project: { id: "proj-1", name: "Acme" },
    case: { id: "case-1", number: "CS0000001", name: "Issue" },
  };
}

function bePage(timeCards: BeTimeCardView[], total: number, offset: number, limit: number): BeSearchTimeCardsResponse {
  return { timeCards, total, offset, limit };
}

/** A mock `BackendApi` whose `post` resolves to `page` and records every
 * payload it was called with, so a test can assert on exactly what
 * `searchTimeCards` forwarded on the wire. */
function mockApi(page: BeSearchTimeCardsResponse): { api: BackendApi; post: ReturnType<typeof vi.fn> } {
  const post = vi.fn(async () => page);
  const api: BackendApi = {
    get: vi.fn(),
    post: post as unknown as BackendApi["post"],
    patch: vi.fn(),
    postEmpty: vi.fn(),
    del: vi.fn(),
    getBlob: vi.fn(),
  };
  return { api, post };
}

function lastPayload(post: ReturnType<typeof vi.fn>): BeSearchTimeCardsPayload {
  return post.mock.calls.at(-1)![1] as BeSearchTimeCardsPayload;
}

describe("mapTimeCard", () => {
  // Confirmed live against the real entity-service (POST /time-cards/search):
  // the per-activity minute breakdown and issue complexity ARE echoed back on
  // read, both for a just-created card and a pre-existing one — a previously
  // documented "write-only" claim that turned out to be wrong.
  it("maps the per-activity minute breakdown and issue complexity through from the wire, when present", () => {
    const withBreakdown: BeTimeCardView = {
      ...beCard("a", "submitted"),
      timeAnalyzing: 11,
      timeSettingUp: 22,
      timeReproducingDebugging: 33,
      timeProvidingSolution: 44,
      timePatching: 55,
      issueComplexity: "High",
    };

    const result = mapTimeCard(withBreakdown);

    expect(result.breakdown).toEqual({
      analysisDebugging: 11,
      settingUp: 22,
      reproduce: 33,
      providingSolution: 44,
      answering: 55,
    });
    expect(result.issueComplexity).toBe("High");
  });

  it("leaves breakdown/issueComplexity undefined when the wire response has none (a card logged before these fields were mapped)", () => {
    const result = mapTimeCard(beCard("a", "submitted"));

    expect(result.breakdown).toBeUndefined();
    expect(result.issueComplexity).toBeUndefined();
  });

  it("treats an unrecognized issueComplexity value as undefined rather than passing it through uncast", () => {
    const result = mapTimeCard({
      ...beCard("a", "submitted"),
      issueComplexity: "Some Future Value",
    });

    expect(result.issueComplexity).toBeUndefined();
  });
});

describe("searchTimeCards", () => {
  it("makes a single request and returns the response as-is", async () => {
    const cards = [beCard("a", "submitted"), beCard("b", "approved")];
    const { api, post } = mockApi(bePage(cards, 2, 0, 20));

    const result = await searchTimeCards(api, undefined, { page: 0, rowsPerPage: 20 });

    expect(post).toHaveBeenCalledTimes(1);
    expect(result.cards.map((c) => c.id)).toEqual(["a", "b"]);
    expect(result.total).toBe(2);
  });

  it("maps workLogComment through from the wire response, when present", async () => {
    const withComment: BeTimeCardView = {
      ...beCard("a", "submitted"),
      workLogComment: "<p>Investigated the reported latency issue.</p>",
    };
    const { api } = mockApi(bePage([withComment, beCard("b", "submitted")], 2, 0, 20));

    const result = await searchTimeCards(api, undefined, { page: 0, rowsPerPage: 20 });

    expect(result.cards[0].workLogComment).toBe(
      "<p>Investigated the reported latency issue.</p>",
    );
    expect(result.cards[1].workLogComment).toBeUndefined();
  });

  // states used to be filtered client-side over one raw page (with a
  // walk-forward workaround for the 500 that combining it with projectIds
  // used to cause); both are fixed upstream now, so this just forwards.
  it("forwards states directly on the wire, in a single request", async () => {
    const { api, post } = mockApi(bePage([beCard("a", "approved")], 1, 0, 20));

    await searchTimeCards(api, { states: ["approved", "rejected"] }, { page: 0, rowsPerPage: 20 });

    expect(post).toHaveBeenCalledTimes(1);
    expect(lastPayload(post).filters?.states).toEqual(["approved", "rejected"]);
  });

  // caseId used to always return total: 0 (worked around via projectIds +
  // client-side filtering in useCaseTimeCards); fixed upstream now.
  it("forwards caseId directly on the wire", async () => {
    const { api, post } = mockApi(bePage([], 0, 0, 20));

    await searchTimeCards(api, { caseId: "case-123" }, { page: 0, rowsPerPage: 20 });

    expect(lastPayload(post).filters?.caseId).toBe("case-123");
  });

  it("forwards userIds directly on the wire, alongside the single-user userId", async () => {
    const { api, post } = mockApi(bePage([], 0, 0, 20));

    await searchTimeCards(
      api,
      { userId: "me", userIds: ["eng-1", "eng-2"] },
      { page: 0, rowsPerPage: 20 },
    );

    const filters = lastPayload(post).filters;
    expect(filters?.userId).toBe("me");
    expect(filters?.userIds).toEqual(["eng-1", "eng-2"]);
  });

  it("omits caseId/states/userIds from the payload when unset", async () => {
    const { api, post } = mockApi(bePage([], 0, 0, 20));

    await searchTimeCards(api, undefined, { page: 0, rowsPerPage: 20 });

    const filters = lastPayload(post).filters;
    expect(filters?.caseId).toBeUndefined();
    expect(filters?.states).toBeUndefined();
    expect(filters?.userIds).toBeUndefined();
  });
});

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useBulkApproveCards", () => {
  beforeEach(() => {
    patchMock.mockReset();
  });

  it("PATCHes every given card id in parallel with state: approved", async () => {
    patchMock.mockResolvedValue({ timeCard: beCard("ignored", "approved") });
    const { result } = renderHook(() => useBulkApproveCards(), { wrapper });

    act(() => {
      result.current.mutate(["tc-1", "tc-2", "tc-3"]);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(patchMock).toHaveBeenCalledTimes(3);
    expect(patchMock).toHaveBeenCalledWith("/time-cards/tc-1", { state: "approved" });
    expect(patchMock).toHaveBeenCalledWith("/time-cards/tc-2", { state: "approved" });
    expect(patchMock).toHaveBeenCalledWith("/time-cards/tc-3", { state: "approved" });
    expect(result.current.data?.succeededIds).toEqual(["tc-1", "tc-2", "tc-3"]);
    expect(result.current.data?.failed).toEqual([]);
  });

  // One approver-ineligible card in an otherwise-fine batch must not sink the
  // whole request — Promise.allSettled is the whole point here (see the
  // hook's own doc comment): the other two still succeed, and the failing
  // one is reported individually rather than the mutation itself rejecting.
  it("reports a partial result when some cards fail and others succeed, without failing the whole mutation", async () => {
    patchMock.mockImplementation((path: string) => {
      if (path === "/time-cards/tc-2") {
        return Promise.reject(new BackendApiError(403, "boom"));
      }
      return Promise.resolve({ timeCard: beCard("ignored", "approved") });
    });
    const { result } = renderHook(() => useBulkApproveCards(), { wrapper });

    act(() => {
      result.current.mutate(["tc-1", "tc-2", "tc-3"]);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.succeededIds).toEqual(["tc-1", "tc-3"]);
    expect(result.current.data?.failed).toEqual([{ cardId: "tc-2", message: "boom" }]);
  });

  it("falls back to a generic message when the failure isn't a client-facing BackendApiError", async () => {
    patchMock.mockRejectedValue(new Error("ECONNRESET"));
    const { result } = renderHook(() => useBulkApproveCards(), { wrapper });

    act(() => {
      result.current.mutate(["tc-1"]);
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.failed).toEqual([
      { cardId: "tc-1", message: "Could not approve this time card." },
    ]);
  });
});

describe("useRecentApprovers persisted cache", () => {
  const storageKey = "csm-portal:time-cards:recent-approvers:v1:eng-1";

  beforeEach(() => {
    postMock.mockReset();
    window.localStorage.clear();
    usersMeId = "eng-1";
  });

  function cardWithApprovers(id: string, workDate: string, approvers: { id: string; name: string }[]): BeTimeCardView {
    return { ...beCard(id, "submitted"), workDate, approvers };
  }

  it("reads a fresh persisted entry on mount and never calls the search endpoint", async () => {
    window.localStorage.setItem(
      storageKey,
      JSON.stringify({
        approvers: [{ id: "appr-1", name: "Ann Approver" }],
        cachedAt: Date.now(),
      }),
    );

    const { result } = renderHook(() => useRecentApprovers(true), { wrapper });

    await waitFor(() => expect(result.current.data).toEqual([{ id: "appr-1", name: "Ann Approver" }]));
    expect(postMock).not.toHaveBeenCalled();
  });

  it("fetches and writes the cache when there is no persisted entry", async () => {
    postMock.mockResolvedValue(
      bePage(
        [cardWithApprovers("a", "2026-08-20", [{ id: "appr-2", name: "Bob Approver" }])],
        1,
        0,
        20,
      ),
    );

    const { result } = renderHook(() => useRecentApprovers(true), { wrapper });

    await waitFor(() => expect(result.current.data).toEqual([{ id: "appr-2", name: "Bob Approver" }]));
    expect(postMock).toHaveBeenCalledTimes(1);

    const stored = JSON.parse(window.localStorage.getItem(storageKey) ?? "null");
    expect(stored.approvers).toEqual([{ id: "appr-2", name: "Bob Approver" }]);
    expect(typeof stored.cachedAt).toBe("number");
  });

  it("ignores a stale (expired-TTL) persisted entry and refetches", async () => {
    window.localStorage.setItem(
      storageKey,
      JSON.stringify({
        approvers: [{ id: "old-approver", name: "Old Approver" }],
        cachedAt: Date.now() - 25 * 60 * 60 * 1000, // 25h, past the 24h TTL
      }),
    );
    postMock.mockResolvedValue(
      bePage(
        [cardWithApprovers("a", "2026-08-20", [{ id: "appr-3", name: "Carol Approver" }])],
        1,
        0,
        20,
      ),
    );

    const { result } = renderHook(() => useRecentApprovers(true), { wrapper });

    await waitFor(() => expect(result.current.data).toEqual([{ id: "appr-3", name: "Carol Approver" }]));
    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("ignores a persisted entry with a future cachedAt and refetches", async () => {
    window.localStorage.setItem(
      storageKey,
      JSON.stringify({
        approvers: [{ id: "old-approver", name: "Old Approver" }],
        cachedAt: Date.now() + 60 * 60 * 1000, // 1h in the future
      }),
    );
    postMock.mockResolvedValue(
      bePage(
        [cardWithApprovers("a", "2026-08-20", [{ id: "appr-5", name: "Eve Approver" }])],
        1,
        0,
        20,
      ),
    );

    const { result } = renderHook(() => useRecentApprovers(true), { wrapper });

    await waitFor(() => expect(result.current.data).toEqual([{ id: "appr-5", name: "Eve Approver" }]));
    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("ignores a malformed/garbage persisted entry and refetches instead of crashing", async () => {
    window.localStorage.setItem(storageKey, "{not json");
    postMock.mockResolvedValue(
      bePage(
        [cardWithApprovers("a", "2026-08-20", [{ id: "appr-4", name: "Dana Approver" }])],
        1,
        0,
        20,
      ),
    );

    const { result } = renderHook(() => useRecentApprovers(true), { wrapper });

    await waitFor(() => expect(result.current.data).toEqual([{ id: "appr-4", name: "Dana Approver" }]));
    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("falls back to always-fetch when localStorage throws (e.g. private browsing/storage disabled)", async () => {
    const getSpy = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("blocked", "SecurityError");
    });
    const setSpy = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("blocked", "SecurityError");
    });
    postMock.mockResolvedValue(
      bePage(
        [cardWithApprovers("a", "2026-08-20", [{ id: "appr-5", name: "Erin Approver" }])],
        1,
        0,
        20,
      ),
    );

    const { result } = renderHook(() => useRecentApprovers(true), { wrapper });

    await waitFor(() => expect(result.current.data).toEqual([{ id: "appr-5", name: "Erin Approver" }]));
    expect(postMock).toHaveBeenCalledTimes(1);

    getSpy.mockRestore();
    setSpy.mockRestore();
  });

  it("drops the persisted cache when invalidateTimecards runs (e.g. after logging a new card)", () => {
    window.localStorage.setItem(
      storageKey,
      JSON.stringify({ approvers: [{ id: "appr-1", name: "Ann Approver" }], cachedAt: Date.now() }),
    );
    const queryClient = new QueryClient();

    invalidateTimecards(queryClient);

    expect(window.localStorage.getItem(storageKey)).toBeNull();
  });

  it("clearPersistedRecentApprovers removes every engineer's entry, not just one", () => {
    window.localStorage.setItem(
      "csm-portal:time-cards:recent-approvers:v1:eng-1",
      JSON.stringify({ approvers: [], cachedAt: Date.now() }),
    );
    window.localStorage.setItem(
      "csm-portal:time-cards:recent-approvers:v1:eng-2",
      JSON.stringify({ approvers: [], cachedAt: Date.now() }),
    );
    window.localStorage.setItem("unrelated-key", "keep-me");

    clearPersistedRecentApprovers();

    expect(window.localStorage.getItem("csm-portal:time-cards:recent-approvers:v1:eng-1")).toBeNull();
    expect(window.localStorage.getItem("csm-portal:time-cards:recent-approvers:v1:eng-2")).toBeNull();
    expect(window.localStorage.getItem("unrelated-key")).toBe("keep-me");
  });
});
