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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const api = { post: vi.fn(), get: vi.fn() };
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => api,
}));

const showSuccess = vi.fn();
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess }),
}));

import { BackendApiError } from "@api/backend/client";
import CsmAllocationDetailPage from "./CsmAllocationDetailPage";
import { ALLOCATION, CANCELLED_UPDATE, UPDATE } from "../test/fixtures";
import { NOT_VISIBLE_NOTE } from "../utils/allocations";

const ID = ALLOCATION.id;
const UPDATES_PATH = `/engagement-allocations/${ID}/updates`;

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/allocations/${ID}`]}>
        <Routes>
          <Route path="/allocations" element={<div>list page</div>} />
          <Route path="/allocations/:allocationId" element={<CsmAllocationDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const updatesPage = (updates: object[], over: object = {}) => ({
  updates,
  total: updates.length,
  limit: 20,
  offset: 0,
  hasMore: false,
  ...over,
});

/** Serves the allocation and a mutable list of updates. */
function serve(initial: object[], over: object = {}) {
  let list = initial;
  api.get.mockImplementation(async (path: string) => {
    if (path === `/engagement-allocations/${ID}`) return ALLOCATION;
    if (path.startsWith(UPDATES_PATH)) return updatesPage(list, over);
    throw new Error(`unexpected GET ${path}`);
  });
  return { set: (next: object[]) => (list = next) };
}

beforeEach(() => {
  api.get.mockReset();
  api.post.mockReset();
  showSuccess.mockReset();
});

describe("CsmAllocationDetailPage: details", () => {
  it("shows the allocation and engagement fields", async () => {
    serve([]);
    renderPage();
    expect(await screen.findByRole("heading", { level: 1, name: /ALLOC0001001/ })).toBeInTheDocument();
    const alloc = within(screen.getByRole("region", { name: "Allocation" }));
    expect(alloc.getByText("jane.doe@example.com")).toBeInTheDocument();
    expect(alloc.getByRole("link", { name: "Example Corp" })).toHaveAttribute(
      "href",
      "/customers/accounts/00000000-0000-0000-0000-0000000000c1",
    );
    expect(alloc.getByText("50%")).toBeInTheDocument();
    expect(alloc.getByText("09:00:00 - 17:00:00")).toBeInTheDocument();
    expect(alloc.getByText("UTC")).toBeInTheDocument();
    const eng = within(screen.getByRole("region", { name: "Engagement" }));
    expect(eng.getByText("Example Migration")).toBeInTheDocument();
    expect(eng.getByText("ENG0001001")).toBeInTheDocument();
    expect(eng.getByText("Migration")).toBeInTheDocument();
    expect(eng.getByText("Remote")).toBeInTheDocument();
    expect(eng.getByText("Integration")).toBeInTheDocument();
    expect(eng.getByText("Yes")).toBeInTheDocument();
    expect(eng.getByText("Dec 1, 2026")).toBeInTheDocument();
  });

  it("says so when the allocation does not exist", async () => {
    api.get.mockResolvedValue(null);
    renderPage();
    expect(await screen.findByText("Allocation not found.")).toBeInTheDocument();
  });

  it("goes back to the list", async () => {
    serve([]);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: /back to allocations/i }));
    expect(await screen.findByText("list page")).toBeInTheDocument();
  });
});

describe("CsmAllocationDetailPage: updates", () => {
  it("lists updates in the order served (newest first) with subject, author, date, cycle week and state", async () => {
    serve([UPDATE, { ...UPDATE, id: "u2", subject: "Older status", state: "PUBLISHED" }]);
    renderPage();
    const list = await screen.findByRole("list", { name: /updates, newest first/i });
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText("Week 1 status")).toBeInTheDocument();
    expect(within(items[0]).getByText(/Jane Doe/)).toBeInTheDocument();
    expect(within(items[0]).getByText(/Sep 22, 2026/)).toBeInTheDocument();
    expect(within(items[0]).getByText(/Cycle week of Sep 21, 2026/)).toBeInTheDocument();
    expect(within(items[0]).getByText("Published")).toBeInTheDocument();
    expect(within(items[1]).getByText("Older status")).toBeInTheDocument();
    expect(screen.getByText("Showing 2 of 2")).toBeInTheDocument();
  });

  it("keeps plain-text line breaks as text", async () => {
    serve([UPDATE]);
    renderPage();
    const body = await screen.findByText(/Line one/);
    expect(body.textContent).toBe("Line one\nLine two");
    expect(body).toHaveStyle({ whiteSpace: "pre-wrap" });
  });

  it("marks a cancelled update clearly instead of hiding it", async () => {
    serve([CANCELLED_UPDATE]);
    renderPage();
    const item = (await screen.findAllByRole("listitem"))[0];
    expect(within(item).getByText("Cancelled")).toBeInTheDocument();
    expect(within(item).getByText("Withdrawn status")).toHaveStyle({ textDecoration: "line-through" });
  });

  it("sanitizes markup in an update's content", async () => {
    serve([{ ...UPDATE, content: '<p>Safe <b>bold</b></p><img src="x" onerror="alert(1)"><script>alert(2)</script>' }]);
    const { container } = renderPage();
    const html = await screen.findByTestId("update-content-html");
    expect(within(html).getByText("bold").tagName).toBe("B");
    expect(container.querySelector("script")).toBeNull();
    expect(container.innerHTML).not.toMatch(/onerror/i);
  });

  it("shows the empty state", async () => {
    serve([]);
    renderPage();
    expect(await screen.findByText("No updates have been posted yet.")).toBeInTheDocument();
  });

  it("loads more updates at the next offset", async () => {
    api.get.mockImplementation(async (path: string) => {
      if (path === `/engagement-allocations/${ID}`) return ALLOCATION;
      if (path.includes("offset=0")) return updatesPage([UPDATE], { total: 2, hasMore: true });
      if (path.includes("offset=1")) return updatesPage([{ ...UPDATE, id: "u2", subject: "Second page" }], { total: 2, offset: 1 });
      throw new Error(`unexpected GET ${path}`);
    });
    renderPage();
    await screen.findByText("Week 1 status");
    expect(api.get).toHaveBeenCalledWith(`${UPDATES_PATH}?limit=20&offset=0`);
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByText("Second page")).toBeInTheDocument();
    expect(api.get).toHaveBeenCalledWith(`${UPDATES_PATH}?limit=20&offset=1`);
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
    expect(screen.getByText("Showing 2 of 2")).toBeInTheDocument();
  });

  it("shows an error state when the updates cannot be loaded", async () => {
    api.get.mockImplementation(async (path: string) => {
      if (path === `/engagement-allocations/${ID}`) return ALLOCATION;
      throw new Error("updates are down");
    });
    renderPage();
    expect(await screen.findByText("updates are down")).toBeInTheDocument();
  });
});

describe("CsmAllocationDetailPage: post update", () => {
  it("shows the not-visible-to-customer note, to everyone, without any role check", async () => {
    serve([]);
    renderPage();
    const note = await screen.findByText(NOT_VISIBLE_NOTE);
    expect(note).toBeInTheDocument();
    expect(NOT_VISIBLE_NOTE).toBe(
      "These updates are currently not visible to the customer and do not trigger any emails internally.",
    );
    expect(screen.getByRole("button", { name: "Post update" })).toBeInTheDocument();
  });

  it("keeps the button off until subject and content are filled", async () => {
    serve([]);
    renderPage();
    const button = await screen.findByRole("button", { name: "Post update" });
    expect(button).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/Subject/), { target: { value: "Hello" } });
    expect(button).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/Content/), { target: { value: "World" } });
    expect(button).toBeEnabled();
  });

  it("posts, clears the form, confirms and shows the new update", async () => {
    const server = serve([]);
    api.post.mockImplementation(async () => {
      server.set([{ ...UPDATE, id: "new", subject: "Fresh status" }]);
      return { ...UPDATE, id: "new", subject: "Fresh status" };
    });
    renderPage();
    await screen.findByText("No updates have been posted yet.");
    fireEvent.change(screen.getByLabelText(/Subject/), { target: { value: "  Fresh status  " } });
    fireEvent.change(screen.getByLabelText(/Content/), { target: { value: "All good" } });
    fireEvent.click(screen.getByRole("button", { name: "Post update" }));

    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith(UPDATES_PATH, { subject: "Fresh status", content: "All good" }),
    );
    await waitFor(() => expect(showSuccess).toHaveBeenCalledWith("Update posted."));
    expect(screen.getByLabelText(/Subject/)).toHaveValue("");
    expect(screen.getByLabelText(/Content/)).toHaveValue("");
    const list = await screen.findByRole("list", { name: /updates, newest first/i });
    expect(within(list).getByText("Fresh status")).toBeInTheDocument();
  });

  it("surfaces the server's message on a 403 and keeps what was typed", async () => {
    serve([]);
    api.post.mockRejectedValue(new BackendApiError(403, "only the allocated engineer can post updates", undefined));
    renderPage();
    await screen.findByText("No updates have been posted yet.");
    fireEvent.change(screen.getByLabelText(/Subject/), { target: { value: "Status" } });
    fireEvent.change(screen.getByLabelText(/Content/), { target: { value: "Body" } });
    fireEvent.click(screen.getByRole("button", { name: "Post update" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("only the allocated engineer can post updates");
    expect(screen.getByLabelText(/Subject/)).toHaveValue("Status");
    expect(screen.getByLabelText(/Content/)).toHaveValue("Body");
    expect(showSuccess).not.toHaveBeenCalled();
  });

  it("surfaces the server's message on a 400", async () => {
    serve([]);
    api.post.mockRejectedValue(new BackendApiError(400, "content is too long", undefined));
    renderPage();
    await screen.findByText("No updates have been posted yet.");
    fireEvent.change(screen.getByLabelText(/Subject/), { target: { value: "Status" } });
    fireEvent.change(screen.getByLabelText(/Content/), { target: { value: "Body" } });
    fireEvent.click(screen.getByRole("button", { name: "Post update" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("content is too long");
  });
});
