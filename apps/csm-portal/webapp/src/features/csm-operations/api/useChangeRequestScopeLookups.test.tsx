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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest; stub it (same approach as usePatchChangeRequest.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: postMock }),
}));

import { useChangeRequestScopeLookups } from "@features/csm-operations/api/useChangeRequestScopeLookups";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

const OPTIONS = {
  deployments: [
    { id: "dep-prod", name: "Acme Production", type: "primary_production" },
    { id: "dep-stg", name: "Acme Staging", type: "staging" },
    { id: "dep-odd", name: "Odd", type: "other" },
  ],
  customerContacts: [
    { id: "pc-1", name: "Alice Aaron", email: "alice@acme.example" },
    { id: "pc-2", name: "Bob Bell" },
  ],
  deploymentProducts: [
    { id: "dp-apim", name: "API Manager 4.3.0", deployment: { id: "dep-prod", name: "Acme Production" } },
    { id: "dp-is", name: "Identity Server 7.0.0", deployment: { id: "dep-prod", name: "Acme Production" } },
  ],
};

describe("useChangeRequestScopeLookups", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue(OPTIONS);
  });

  it("does not call the backend until a project is chosen", () => {
    const { result } = renderHook(() => useChangeRequestScopeLookups(undefined, []), { wrapper });
    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.deployments).toEqual([]);
    expect(result.current.isLoading).toBe(false);
  });

  it("asks POST /change-requests/link-options for the project alone, with no deploymentIds key", async () => {
    const { result } = renderHook(() => useChangeRequestScopeLookups("proj-a", []), { wrapper });
    await waitFor(() => expect(result.current.deployments).toHaveLength(3));
    expect(postMock).toHaveBeenCalledWith("/change-requests/link-options", { projectId: "proj-a" });
  });

  it("maps each deployment to its name, and reports no products for unchosen deployments", async () => {
    const { result } = renderHook(() => useChangeRequestScopeLookups("proj-a", []), { wrapper });
    await waitFor(() => expect(result.current.deployments).toHaveLength(3));
    const [prod] = result.current.deployments;
    expect(prod).toEqual({ id: "dep-prod", label: "Acme Production", type: "primary_production", products: undefined });
    expect(Object.keys(prod!)).not.toContain("environments");
  });

  it("exposes the project's registered contacts as the read-only customer group, once the lookup has settled", async () => {
    const { result } = renderHook(() => useChangeRequestScopeLookups("proj-a", []), { wrapper });
    // Not ready (and empty) before the lookup resolves.
    expect(result.current.contactsReady).toBe(false);
    expect(result.current.customerContacts).toEqual([]);
    await waitFor(() => expect(result.current.contactsReady).toBe(true));
    expect(result.current.customerContacts.map((c) => c.name)).toEqual(["Alice Aaron", "Bob Bell"]);
  });

  it("has no contacts, and is not ready, without a project", () => {
    const { result } = renderHook(() => useChangeRequestScopeLookups(undefined, []), { wrapper });
    expect(result.current.customerContacts).toEqual([]);
    expect(result.current.contactsReady).toBe(false);
  });

  it("treats a response without customerContacts as an empty (but known) group", async () => {
    postMock.mockResolvedValue({ deployments: [], deploymentProducts: [] });
    const { result } = renderHook(() => useChangeRequestScopeLookups("proj-a", []), { wrapper });
    await waitFor(() => expect(result.current.contactsReady).toBe(true));
    expect(result.current.customerContacts).toEqual([]);
  });

  it("sends the chosen deployments and attributes each product to the deployment it is deployed in", async () => {
    const { result } = renderHook(() => useChangeRequestScopeLookups("proj-a", ["dep-prod", "dep-stg"]), {
      wrapper,
    });
    await waitFor(() => expect(result.current.deployments[0]?.products).toBeDefined());
    expect(postMock).toHaveBeenCalledWith("/change-requests/link-options", {
      projectId: "proj-a",
      deploymentIds: ["dep-prod", "dep-stg"],
    });
    const [prod, stg, odd] = result.current.deployments;
    expect(prod!.products).toEqual([
      { id: "dp-apim", label: "API Manager 4.3.0" },
      { id: "dp-is", label: "Identity Server 7.0.0" },
    ]);
    // Chosen but carrying no products: known-empty.
    expect(stg!.products).toEqual([]);
    // Not chosen: unknown.
    expect(odd!.products).toBeUndefined();
  });

  it("reports products as not yet known (not stale) while a changed selection is loading, keeping the deployment list", async () => {
    let resolveSecond: (v: unknown) => void = () => undefined;
    postMock.mockResolvedValueOnce(OPTIONS);
    postMock.mockImplementationOnce(() => new Promise((r) => (resolveSecond = r)));
    const { result, rerender } = renderHook(
      ({ ids }: { ids: string[] }) => useChangeRequestScopeLookups("proj-a", ids),
      { wrapper, initialProps: { ids: [] as string[] } },
    );
    await waitFor(() => expect(result.current.deployments).toHaveLength(3));
    rerender({ ids: ["dep-prod"] });
    // Previous result stays on screen, but its products must not be reported for the new selection.
    expect(result.current.deployments).toHaveLength(3);
    expect(result.current.deployments[0]!.products).toBeUndefined();
    resolveSecond(OPTIONS);
    await waitFor(() => expect(result.current.deployments[0]!.products).toHaveLength(2));
  });

  it("never shows one project's deployments under another project while the new ones load", async () => {
    let resolveSecond: (v: unknown) => void = () => undefined;
    postMock.mockResolvedValueOnce(OPTIONS);
    postMock.mockImplementationOnce(() => new Promise((r) => (resolveSecond = r)));
    const { result, rerender } = renderHook(
      ({ project }: { project: string }) => useChangeRequestScopeLookups(project, []),
      { wrapper, initialProps: { project: "proj-a" } },
    );
    await waitFor(() => expect(result.current.deployments).toHaveLength(3));
    rerender({ project: "proj-b" });
    expect(result.current.deployments).toEqual([]);
    // Nor does one customer's contact list: the new project's is empty until it loads.
    expect(result.current.customerContacts).toEqual([]);
    expect(result.current.contactsReady).toBe(false);
    resolveSecond({
      deployments: [{ id: "dep-b", name: "Beta", type: "development" }],
      deploymentProducts: [],
      customerContacts: [{ id: "pc-9", name: "Carol Cook" }],
    });
    await waitFor(() => expect(result.current.deployments.map((d) => d.id)).toEqual(["dep-b"]));
    expect(result.current.customerContacts.map((c) => c.name)).toEqual(["Carol Cook"]);
  });

  it("surfaces a failed lookup", async () => {
    postMock.mockReset();
    postMock.mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() => useChangeRequestScopeLookups("proj-a", []), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.deployments).toEqual([]);
  });
});
