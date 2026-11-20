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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactElement } from "react";
import "@testing-library/jest-dom/vitest";

const getMock = vi.fn();

// The real client reads runtime config at module load (same approach as
// LinkedIncidentsListWidget.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

import { ProductIssuesWidget } from "@features/csm-cases/components/ProductIssuesWidget";

function renderWithProviders(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

function row(over: Record<string, unknown> = {}) {
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
      labels: [],
    },
    boardStatus: "WAITING_ON_PRODUCT_TEAM",
    boardStatusRaw: "Waiting on product team",
    lastActivityOn: "2026-01-01T00:00:00Z",
    linkState: "ACTIVE",
    ...over,
  };
}

describe("ProductIssuesWidget", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it("renders nothing at all when the endpoint answers 404 (feature off)", async () => {
    getMock.mockResolvedValue(null);
    const { container } = renderWithProviders(<ProductIssuesWidget caseId="case-1" />);
    await waitFor(() =>
      expect(getMock).toHaveBeenCalledWith("/cases/case-1/product-work-items"),
    );
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });

  it("renders nothing on a 403", async () => {
    getMock.mockRejectedValue(Object.assign(new Error("forbidden"), { status: 403 }));
    const { container } = renderWithProviders(<ProductIssuesWidget caseId="case-1" />);
    await waitFor(() => expect(getMock).toHaveBeenCalled());
    await waitFor(() => expect(container).toBeEmptyDOMElement());
  });

  it("shows a loading state first", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderWithProviders(<ProductIssuesWidget caseId="case-1" />);
    expect(screen.getByText("Product issues")).toBeInTheDocument();
    expect(screen.queryByText("No product issues linked to this case.")).not.toBeInTheDocument();
  });

  it("renders an empty state when nothing is linked", async () => {
    getMock.mockResolvedValue({ items: [] });
    renderWithProviders(<ProductIssuesWidget caseId="case-1" />);
    expect(await screen.findByText("No product issues linked to this case.")).toBeInTheDocument();
  });

  it("shows an error state with retry for a server error", async () => {
    getMock.mockRejectedValue(Object.assign(new Error("boom"), { status: 500 }));
    renderWithProviders(<ProductIssuesWidget caseId="case-1" />);
    expect(await screen.findByText("Could not load product issues.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });

  it("renders rows with status labels, GitHub links and link state", async () => {
    getMock.mockResolvedValue({
      items: [
        row(),
        row({
          id: "wi-2",
          number: "PI0000002",
          title: "Second",
          boardStatus: null,
          boardStatusRaw: null,
          github: { ...row().github, issueNumber: 13, htmlUrl: "https://github.com/wso2/product-is/issues/13", state: "CLOSED" },
          linkState: "INACCESSIBLE",
        }),
        row({ id: "wi-3", number: "PI0000003", title: "Third", boardStatus: "OTHER", boardStatusRaw: "Needs triage" }),
      ],
    });
    renderWithProviders(<ProductIssuesWidget caseId="case-1" />);

    expect(await screen.findByText("Product issues (3)")).toBeInTheDocument();
    expect(screen.getByText("Waiting on product team")).toBeInTheDocument();
    // no board status: falls back to the GitHub state
    expect(screen.getByText("Closed")).toBeInTheDocument();
    expect(screen.getByText("No longer accessible")).toBeInTheDocument();
    // OTHER shows the raw column name
    expect(screen.getByText("Needs triage")).toBeInTheDocument();
    expect(screen.queryByText("Other")).not.toBeInTheDocument();

    const link = screen.getByRole("link", { name: "PI0000001 on GitHub" });
    expect(link).toHaveAttribute("href", "https://github.com/wso2/product-is/issues/12");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("opens the detail with the conversation as plain text and the status history", async () => {
    getMock.mockImplementation((path: string) => {
      if (path === "/product-work-items/wi-1") {
        return Promise.resolve({
          ...row(),
          conversation: [
            {
              id: "c1",
              author: "alice",
              body: "<script>alert(1)</script>\nline two",
              createdOn: "2026-01-01T00:00:00Z",
              origin: "GITHUB",
              githubCommentId: 5,
            },
          ],
          conversationTruncated: false,
          statusHistory: [
            { id: "s1", fromStatus: "OPEN", toStatus: "IN_PROGRESS", occurredOn: "2026-01-02T00:00:00Z", source: "WEBHOOK" },
          ],
        });
      }
      return Promise.resolve({ items: [row()] });
    });
    renderWithProviders(<ProductIssuesWidget caseId="case-1" />);

    fireEvent.click(await screen.findByLabelText("View product issue PI0000001"));

    const body = await screen.findByText(/alert\(1\)/);
    expect(body.textContent).toBe("<script>alert(1)</script>\nline two");
    expect(body.querySelector("script")).toBeNull();
    expect(document.querySelector("script")).toBeNull();
    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(screen.getByText(/Open → In progress/)).toBeInTheDocument();
    expect(screen.getByText(/GitHub webhook/)).toBeInTheDocument();
  });

  it("shows an error state in the detail when it fails to load", async () => {
    getMock.mockImplementation((path: string) =>
      path.startsWith("/product-work-items/")
        ? Promise.reject(Object.assign(new Error("boom"), { status: 500 }))
        : Promise.resolve({ items: [row()] }),
    );
    renderWithProviders(<ProductIssuesWidget caseId="case-1" />);
    fireEvent.click(await screen.findByLabelText("View product issue PI0000001"));
    expect(await screen.findByText("Could not load this product issue.")).toBeInTheDocument();
  });

  it("shows a not-found state in the detail when the issue no longer exists", async () => {
    getMock.mockImplementation((path: string) =>
      path.startsWith("/product-work-items/") ? Promise.resolve(null) : Promise.resolve({ items: [row()] }),
    );
    renderWithProviders(<ProductIssuesWidget caseId="case-1" />);
    fireEvent.click(await screen.findByLabelText("View product issue PI0000001"));
    expect(await screen.findByText("This product issue was not found.")).toBeInTheDocument();
  });
});
