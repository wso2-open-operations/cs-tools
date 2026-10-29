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
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const getMock = vi.fn();
const postMock = vi.fn();
const patchMock = vi.fn();
const postEmptyMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, post: postMock, patch: patchMock, postEmpty: postEmptyMock }),
}));

import {
  STANDARD_CHANGES_KEY,
  useClearStandardChangeTemplateReview,
  useCreateStandardChangeProposal,
  useDecideStandardChangeProposal,
  useGetStandardChangeProposal,
  useGetStandardChangeTemplate,
  useGetStandardChangeTemplateVersion,
  useResumeStandardChangeTemplate,
  useSearchActiveStandardChangeTemplatesForSelect,
  useSearchClosedChangeRequestsForSelect,
  useSearchStandardChangeProposals,
  useSearchStandardChangeTemplates,
  useSubmitStandardChangeProposal,
  useSuspendStandardChangeTemplate,
  useUpdateStandardChangeProposal,
  useWithdrawStandardChangeProposal,
} from "@features/csm-operations/api/useStandardChanges";

function wrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

function newClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
}

beforeEach(() => {
  getMock.mockReset().mockResolvedValue({ id: "x" });
  postMock.mockReset().mockResolvedValue({ templates: [], proposals: [], changeRequests: [], total: 0 });
  patchMock.mockReset().mockResolvedValue({ id: "scp-1" });
  postEmptyMock.mockReset().mockResolvedValue({ id: "x" });
});

describe("standard change reads", () => {
  it("searches the catalogue with the body as given", async () => {
    const client = newClient();
    const body = { states: ["retired" as const], query: "tls", pagination: { offset: 20, limit: 20 } };
    const { result } = renderHook(() => useSearchStandardChangeTemplates(body), { wrapper: wrapper(client) });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledWith("/standard-change-templates/search", body);
  });

  it("offers only active templates to the create form's picker, with the typed query", async () => {
    postMock.mockResolvedValue({ templates: [{ id: "tpl-1" }], total: 1 });
    const client = newClient();
    const { result } = renderHook(() => useSearchActiveStandardChangeTemplatesForSelect(" tls ", true), { wrapper: wrapper(client) });
    await waitFor(() => expect(result.current.data).toEqual([{ id: "tpl-1" }]));
    expect(postMock).toHaveBeenCalledWith("/standard-change-templates/search", {
      states: ["active"],
      query: "tls",
      pagination: { offset: 0, limit: 20 },
    });
  });

  it("does not search for the picker until it is opened", () => {
    const client = newClient();
    renderHook(() => useSearchActiveStandardChangeTemplatesForSelect("", false), { wrapper: wrapper(client) });
    expect(postMock).not.toHaveBeenCalled();
  });

  it("reads a template, a version and a proposal by their paths, and nothing without an id", async () => {
    const client = newClient();
    const { result } = renderHook(
      () => ({
        template: useGetStandardChangeTemplate("tpl-1"),
        version: useGetStandardChangeTemplateVersion("tpl-1", "ver-1"),
        proposal: useGetStandardChangeProposal("scp-1"),
        none: useGetStandardChangeTemplate(undefined),
        noVersion: useGetStandardChangeTemplateVersion("tpl-1", undefined),
      }),
      { wrapper: wrapper(client) },
    );
    await waitFor(() => expect(result.current.proposal.isSuccess).toBe(true));
    await waitFor(() => expect(result.current.version.isSuccess).toBe(true));
    expect(getMock.mock.calls.map((c) => c[0]).sort()).toEqual([
      "/standard-change-proposals/scp-1",
      "/standard-change-templates/tpl-1",
      "/standard-change-templates/tpl-1/versions/ver-1",
    ]);
  });

  it("searches proposals with the body as given", async () => {
    const client = newClient();
    const body = { awaitingMyDecision: true, states: ["cab_review" as const], pagination: { offset: 0, limit: 20 } };
    const { result } = renderHook(() => useSearchStandardChangeProposals(body), { wrapper: wrapper(client) });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledWith("/standard-change-proposals/search", body);
  });

  it("finds evidence among CLOSED change requests only", async () => {
    postMock.mockResolvedValue({ changeRequests: [{ id: "chg-1" }], total: 1 });
    const client = newClient();
    const { result } = renderHook(() => useSearchClosedChangeRequestsForSelect("CHG1", true), { wrapper: wrapper(client) });
    await waitFor(() => expect(result.current.data).toEqual([{ id: "chg-1" }]));
    expect(postMock).toHaveBeenCalledWith("/change-requests/search", {
      filters: { searchQuery: "CHG1", states: ["closed"] },
      pagination: { offset: 0, limit: 20 },
    });
  });
});

describe("standard change writes", () => {
  /** Runs a write and returns the keys it invalidated. */
  async function run<T>(useHook: () => { mutateAsync: (v: T) => Promise<unknown> }, vars: T): Promise<unknown[][]> {
    const client = newClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(useHook, { wrapper: wrapper(client) });
    await act(async () => {
      await result.current.mutateAsync(vars).catch(() => undefined);
    });
    return invalidate.mock.calls.map((c) => (c[0] as { queryKey: unknown[] }).queryKey);
  }

  it.each([
    ["resume", useResumeStandardChangeTemplate, "/standard-change-templates/tpl-1/resume"],
    ["clear the review flag of", useClearStandardChangeTemplateReview, "/standard-change-templates/tpl-1/clear-review"],
  ] as const)("posts to %s a template, with no body, and refreshes every standard change query", async (_n, hook, path) => {
    const keys = await run(hook, "tpl-1");
    expect(postEmptyMock).toHaveBeenCalledWith(path);
    expect(keys).toContainEqual([...STANDARD_CHANGES_KEY]);
  });

  it("suspends a template with the reason, trimmed, and refreshes every standard change query", async () => {
    const keys = await run(useSuspendStandardChangeTemplate, { id: "tpl-1", reason: "  Rolled back twice on prod \n" });
    expect(postMock).toHaveBeenCalledWith("/standard-change-templates/tpl-1/suspend", { reason: "Rolled back twice on prod" });
    expect(postEmptyMock).not.toHaveBeenCalled();
    expect(keys).toContainEqual([...STANDARD_CHANGES_KEY]);
  });

  it.each([
    ["no reason", undefined],
    ["a blank reason", "   "],
  ])("suspends a template with %s as an empty body", async (_n, reason) => {
    await run(useSuspendStandardChangeTemplate, { id: "a/b", reason });
    expect(postMock).toHaveBeenCalledWith("/standard-change-templates/a%2Fb/suspend", {});
  });

  it.each([
    ["submits", useSubmitStandardChangeProposal, "/standard-change-proposals/scp-1/submit"],
    ["withdraws", useWithdrawStandardChangeProposal, "/standard-change-proposals/scp-1/withdraw"],
  ] as const)("%s a proposal", async (_n, hook, path) => {
    const keys = await run(hook, "scp-1");
    expect(postEmptyMock).toHaveBeenCalledWith(path);
    expect(keys).toContainEqual([...STANDARD_CHANGES_KEY]);
  });

  it("creates a proposal with the body as given", async () => {
    const body = { kind: "new" as const, sourceChangeRequestId: "chg-1" };
    const keys = await run(useCreateStandardChangeProposal, body);
    expect(postMock).toHaveBeenCalledWith("/standard-change-proposals", body);
    expect(keys).toContainEqual([...STANDARD_CHANGES_KEY]);
  });

  it("patches a draft with only the patch", async () => {
    const patch = { name: "Rotate", parameters: [], evidenceChangeRequestIds: ["chg-1"] };
    await run(useUpdateStandardChangeProposal, { id: "scp-1", patch });
    expect(patchMock).toHaveBeenCalledWith("/standard-change-proposals/scp-1", patch);
  });

  it("posts a decision without the id in the body", async () => {
    await run(useDecideStandardChangeProposal, { id: "scp-1", decision: "rejected" as const, comment: "Too risky" });
    expect(postMock).toHaveBeenCalledWith("/standard-change-proposals/scp-1/decision", { decision: "rejected", comment: "Too risky" });
  });

  it("refreshes after a refused write too, and hands the error to the caller", async () => {
    postMock.mockRejectedValue(new Error("only the owner or a CAB member can suspend it"));
    const client = newClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const { result } = renderHook(() => useSuspendStandardChangeTemplate(), { wrapper: wrapper(client) });
    let error: unknown;
    await act(async () => {
      await result.current.mutateAsync({ id: "tpl-1", reason: "x" }).catch((e: unknown) => (error = e));
    });
    expect((error as Error).message).toBe("only the owner or a CAB member can suspend it");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: STANDARD_CHANGES_KEY });
  });

  it("escapes ids into the path", async () => {
    await run(useSubmitStandardChangeProposal, "a/b");
    expect(postEmptyMock).toHaveBeenCalledWith("/standard-change-proposals/a%2Fb/submit");
  });
});
