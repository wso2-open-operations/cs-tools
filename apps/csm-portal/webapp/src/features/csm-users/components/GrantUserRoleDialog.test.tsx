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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

const postMock = vi.fn();
const getMock = vi.fn();

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, get: getMock }),
}));

import GrantUserRoleDialog from "@features/csm-users/components/GrantUserRoleDialog";

function renderDialog(currentRoles: string[] = []) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <GrantUserRoleDialog open onClose={vi.fn()} userId="user-1" currentRoles={currentRoles} />
    </QueryClientProvider>,
  );
}

describe("GrantUserRoleDialog", () => {
  beforeEach(() => {
    postMock.mockReset();
    getMock.mockReset();
    getMock.mockResolvedValue({ roles: [{ key: "cs_engineer" }, { key: "escalator" }] });
  });

  it("renders a checkbox per grantable role with its friendly label", async () => {
    renderDialog();
    await waitFor(() => expect(getMock).toHaveBeenCalledWith("/roles/grantable"));

    expect(await screen.findByRole("checkbox", { name: "CS Engineer" })).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Escalator" })).toBeInTheDocument();
  });

  it("shows an already-granted role as checked and disabled, and never sends it", async () => {
    postMock.mockResolvedValue({ granted: ["escalator"] });
    renderDialog(["cs_engineer"]);

    const alreadyGranted = await screen.findByRole("checkbox", {
      name: "CS Engineer (already granted)",
    });
    expect(alreadyGranted).toBeChecked();
    expect(alreadyGranted).toBeDisabled();

    fireEvent.click(screen.getByRole("checkbox", { name: "Escalator" }));
    fireEvent.click(screen.getByRole("button", { name: /^grant$/i }));

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith("/users/user-1/roles", { roles: ["escalator"] }),
    );
  });

  it("disables Grant until at least one role is selected", async () => {
    renderDialog();
    await screen.findByRole("checkbox", { name: "CS Engineer" });
    expect(screen.getByRole("button", { name: /^grant$/i })).toBeDisabled();

    fireEvent.click(screen.getByRole("checkbox", { name: "CS Engineer" }));
    expect(screen.getByRole("button", { name: /^grant$/i })).toBeEnabled();
  });

  it("shows a success message naming the granted role(s) on full success", async () => {
    postMock.mockResolvedValue({ granted: ["cs_engineer"] });
    renderDialog();

    fireEvent.click(await screen.findByRole("checkbox", { name: "CS Engineer" }));
    fireEvent.click(screen.getByRole("button", { name: /^grant$/i }));

    expect(await screen.findByText(/granted: cs engineer/i)).toBeInTheDocument();
  });

  it("shows a partial-failure message naming both the granted and the failed role(s), and leaves the failed one checked", async () => {
    postMock.mockResolvedValue({ granted: ["cs_engineer"], failed: ["escalator"] });
    renderDialog();

    fireEvent.click(await screen.findByRole("checkbox", { name: "CS Engineer" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Escalator" }));
    fireEvent.click(screen.getByRole("button", { name: /^grant$/i }));

    expect(await screen.findByText(/granted cs engineer; failed: escalator/i)).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Escalator" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "CS Engineer" })).not.toBeChecked();
  });

  it("shows an error message when the request fails outright", async () => {
    postMock.mockRejectedValue(new Error("network error"));
    renderDialog();

    fireEvent.click(await screen.findByRole("checkbox", { name: "CS Engineer" }));
    fireEvent.click(screen.getByRole("button", { name: /^grant$/i }));

    expect(await screen.findByText("network error")).toBeInTheDocument();
  });

  it("shows an error with a retry action when the portal-roles fetch fails", async () => {
    getMock.mockReset();
    getMock.mockRejectedValueOnce(new Error("network error"));
    getMock.mockResolvedValueOnce({ roles: [{ key: "cs_engineer" }] });
    renderDialog();

    await screen.findByText(/failed to load portal roles/i);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await screen.findByRole("checkbox", { name: "CS Engineer" });
  });
});
