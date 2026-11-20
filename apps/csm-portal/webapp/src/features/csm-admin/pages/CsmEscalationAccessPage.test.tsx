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


import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const getMock = vi.fn();
const putMock = vi.fn();
const deleteMock = vi.fn();
let mockIsAdmin = true;

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ get: getMock, put: putMock, delete: deleteMock, post: vi.fn() }),
}));
vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => ({ isAdmin: mockIsAdmin }),
}));
// The directory search is its own, already-tested picker; here it only needs
// to hand back a chosen user id.
vi.mock("@features/csm-cases/components/AsyncUserIdSingleSelect", () => ({
  default: ({ value, onChange }: { value: string; onChange: (id: string) => void }) => (
    <button type="button" onClick={() => onChange("11111111-1111-4111-8111-111111111111")}>
      pick-user:{value || "none"}
    </button>
  ),
}));

import CsmEscalationAccessPage from "@features/csm-admin/pages/CsmEscalationAccessPage";

function renderPage(): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
  return render(<CsmEscalationAccessPage />, { wrapper });
}

const named = {
  userId: "22222222-2222-4222-8222-222222222222",
  name: "Named Person",
  email: "named@example.com",
  grantedBy: "admin@example.com",
  grantedOn: "2026-11-01T00:00:00Z",
  active: true,
};

describe("CsmEscalationAccessPage", () => {
  beforeEach(() => {
    getMock.mockReset();
    putMock.mockReset();
    deleteMock.mockReset();
    mockIsAdmin = true;
  });

  it("lists who may de-escalate", async () => {
    getMock.mockResolvedValue({ deescalators: [named] });
    renderPage();
    expect(await screen.findByText("Named Person")).toBeInTheDocument();
    expect(getMock).toHaveBeenCalledWith("/case-escalation-access/deescalators");
  });

  it("shows nothing and asks for nothing to a non-admin", () => {
    mockIsAdmin = false;
    renderPage();
    expect(getMock).not.toHaveBeenCalled();
    expect(screen.queryByText("Escalation access")).not.toBeInTheDocument();
  });

  it("adds the chosen person", async () => {
    getMock.mockResolvedValue({ deescalators: [] });
    putMock.mockResolvedValue(undefined);
    renderPage();
    await screen.findByText(/Nobody is named yet/);

    fireEvent.click(screen.getByRole("button", { name: /pick-user/ }));
    fireEvent.click(screen.getByRole("button", { name: "Let them de-escalate" }));

    await waitFor(() =>
      expect(putMock).toHaveBeenCalledWith(
        "/case-escalation-access/deescalators/11111111-1111-4111-8111-111111111111",
        undefined,
      ),
    );
  });

  it("removes a person after confirming", async () => {
    getMock.mockResolvedValue({ deescalators: [named] });
    deleteMock.mockResolvedValue(undefined);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Remove" }));
    expect(screen.getByText(/Remove Named Person\?/)).toBeInTheDocument();
    expect(deleteMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getAllByRole("button", { name: "Remove" }).at(-1) as HTMLElement);
    await waitFor(() =>
      expect(deleteMock).toHaveBeenCalledWith(
        "/case-escalation-access/deescalators/22222222-2222-4222-8222-222222222222",
      ),
    );
  });

  it("flags a grant whose holder is no longer active", async () => {
    getMock.mockResolvedValue({ deescalators: [{ ...named, active: false }] });
    renderPage();
    expect(await screen.findByText("Inactive")).toBeInTheDocument();
  });
});
