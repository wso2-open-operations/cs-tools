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

import { render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { MyWocGitIssuesPanel } from "@features/csm-cases/components/MyWocGitIssuesPanel";

function renderPanel() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <MemoryRouter>
      <QueryClientProvider client={queryClient}>
        <MyWocGitIssuesPanel />
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

function item(over: Record<string, unknown> = {}) {
  return {
    id: "wi-1",
    number: "PI0000001",
    title: "Login fails",
    github: {
      owner: "wso2",
      repository: "product-is",
      issueNumber: 12,
      htmlUrl: "https://github.com/wso2/product-is/issues/12",
      state: "OPEN",
    },
    boardStatus: "WAITING_ON_CS",
    case: { id: "case-uuid-1", number: "CS0001234" },
    lastActivityOn: "2026-01-01T00:00:00Z",
    linkState: "ACTIVE",
    sla: {
      state: "PAUSED",
      budgetSeconds: 86400,
      consumedSeconds: 36000,
      remainingSeconds: 50400,
      percentage: 41.67,
      running: false,
      pausedSince: "2026-11-18T08:00:00Z",
    },
    ...over,
  };
}

describe("MyWocGitIssuesPanel", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("sends exactly the waiting-on-CS / mine / pausedSince search", async () => {
    postMock.mockResolvedValue({ items: [], total: 0 });
    renderPanel();
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(1));
    expect(postMock).toHaveBeenCalledWith("/product-work-items/search", {
      filters: { boardStatuses: ["WAITING_ON_CS"], mine: true, sort: "pausedSince" },
      pagination: { limit: 50, offset: 0 },
    });
  });

  it("renders rows with GitHub link, case link, waiting since and SLA chip", async () => {
    postMock.mockResolvedValue({ items: [item()], total: 1 });
    renderPanel();
    expect(await screen.findByText("My WOC Git issues (1)")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "PI0000001 on GitHub" })).toHaveAttribute(
      "href",
      "https://github.com/wso2/product-is/issues/12",
    );
    expect(screen.getByRole("link", { name: "Case CS0001234" })).toHaveAttribute(
      "href",
      "/cases/case-uuid-1",
    );
    expect(screen.getByText("Login fails")).toBeInTheDocument();
    expect(screen.getByText("Paused (waiting on CS)")).toBeInTheDocument();
    const rowEl = screen.getByText("Login fails").closest("tr") as HTMLElement;
    expect(within(rowEl).queryByText("—")).not.toBeInTheDocument();
  });

  it("shows the empty state", async () => {
    postMock.mockResolvedValue({ items: [], total: 0 });
    renderPanel();
    expect(await screen.findByText("No Git issues are waiting on you.")).toBeInTheDocument();
  });

  it("renders nothing when the route answers 404 (feature off)", async () => {
    postMock.mockRejectedValue(Object.assign(new Error("nf"), { status: 404 }));
    const { container } = renderPanel();
    await waitFor(() => expect(postMock).toHaveBeenCalled());
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });

  it("renders nothing on a 403", async () => {
    postMock.mockRejectedValue(Object.assign(new Error("no"), { status: 403 }));
    const { container } = renderPanel();
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });

  it("shows an error state with retry for a server error", async () => {
    postMock.mockRejectedValue(Object.assign(new Error("boom"), { status: 500 }));
    renderPanel();
    expect(await screen.findByText("Could not load your Git issues.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });
});
