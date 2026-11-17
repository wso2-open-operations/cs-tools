import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { useAcknowledge, useSetOrganizationOwner } from "./hooks";

const api = vi.hoisted(() => ({ post: vi.fn(), patch: vi.fn() }));
vi.mock("@features/plg/api/client", () => ({ usePlgApi: () => api }));

function setup() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  // The organisation page caches its detail under the Cloud id it was opened by.
  client.setQueryData(["organization", "cloud-1"], { id: "o1", moesifCompanyId: "cloud-1", owner: null });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { client, wrapper };
}

describe("organisation detail cached under its Cloud id", () => {
  it("is refreshed when a registration is acknowledged", async () => {
    const { client, wrapper } = setup();
    api.post.mockResolvedValue({ organizationId: "o1", product: { code: "IAM" } });
    const { result } = renderHook(() => useAcknowledge(), { wrapper });
    await act(() => result.current.mutateAsync({ orgPlatformId: "p1" }));
    expect(client.getQueryState(["organization", "cloud-1"])?.isInvalidated).toBe(true);
  });

  it("shows a new owner at once", async () => {
    const { client, wrapper } = setup();
    const updated = { id: "o1", moesifCompanyId: "cloud-1", owner: { id: "u1", name: "Ana", email: "ana@example.test" } };
    api.patch.mockResolvedValue(updated);
    const { result } = renderHook(() => useSetOrganizationOwner("o1"), { wrapper });
    await act(() => result.current.mutateAsync("u1"));
    expect(client.getQueryData(["organization", "cloud-1"])).toEqual(updated);
  });
});
