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
import { act, fireEvent, render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { listKnowledgeBases, searchArticles } from "@api/backend/client";
import type { KBArticle } from "@api/backend/types";
import SearchBox from "@features/kb/components/SearchBox";

vi.mock("@api/backend/client", async () => {
  const actual = await vi.importActual<typeof import("@api/backend/client")>("@api/backend/client");
  return { ...actual, listKnowledgeBases: vi.fn(), searchArticles: vi.fn() };
});

const article = (id: string, title: string): KBArticle => ({
  id,
  knowledgeBaseId: "kb",
  title,
  body: "<p>b</p>",
  createdOn: "2026-01-01T00:00:00Z",
  updatedOn: "2026-01-01T00:00:00Z",
});

function Location() {
  const { pathname, search } = useLocation();
  return <output data-testid="location">{pathname + search}</output>;
}

const renderBox = (onSearch = vi.fn()) => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route
            path="*"
            element={
              <>
                <SearchBox label="Search articles" placeholder="Search" onSearch={onSearch} />
                <Location />
              </>
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return onSearch;
};

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.mocked(listKnowledgeBases).mockResolvedValue({ knowledgeBases: [{ id: "kb", title: "Product KB" }] });
  vi.mocked(searchArticles).mockReset();
});

describe("SearchBox suggestions", () => {
  it("suggests matching articles after a pause and opens one with the keyboard", async () => {
    vi.mocked(searchArticles).mockResolvedValue({
      articles: [article("a1", "Gateway timeout"), article("a2", "Gateway certificate")],
      total: 2,
      limit: 5,
      offset: 0,
      hasMore: false,
    });
    const onSearch = renderBox();
    const input = screen.getByRole("searchbox", { name: "Search articles" });

    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "gate" } });
    expect(searchArticles).not.toHaveBeenCalled();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(300);
    });

    expect(searchArticles).toHaveBeenCalledWith({ searchQuery: "gate", limit: 5 }, expect.anything());
    const options = await screen.findAllByRole("option");
    expect(options).toHaveLength(3);
    expect(options[0]).toHaveTextContent("Gateway timeout");
    expect(options[0]).toHaveTextContent("Product KB");
    expect(options[2]).toHaveTextContent('See all results for “gate”');

    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(input).toHaveAttribute("aria-activedescendant", options[1].id);
    fireEvent.submit(input.closest("form")!);
    expect(screen.getByTestId("location")).toHaveTextContent("/knowledge-articles/a2");
    expect(onSearch).not.toHaveBeenCalled();
  });

  it("never offers the previous query's articles while a newer draft is still loading", async () => {
    vi.mocked(searchArticles).mockResolvedValue({
      articles: [article("a1", "Gateway timeout")],
      total: 1,
      limit: 5,
      offset: 0,
      hasMore: false,
    });
    const onSearch = renderBox();
    const input = screen.getByRole("searchbox", { name: "Search articles" });

    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "gate" } });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(300);
    });
    expect(await screen.findAllByRole("option")).toHaveLength(2);

    // Keep typing: the old options must disappear before the new request settles.
    fireEvent.change(input, { target: { value: "gatez" } });
    expect(screen.queryAllByRole("option")).toHaveLength(0);
    expect(screen.getByText("Searching…")).toBeInTheDocument();

    // Enter during that window submits the draft rather than opening the stale article.
    fireEvent.submit(input.closest("form")!);
    expect(onSearch).toHaveBeenCalledWith("gatez");
    expect(screen.getByTestId("location")).toHaveTextContent("/");
  });

  it("submits a plain search when nothing is highlighted and does not query for one character", async () => {
    vi.mocked(searchArticles).mockResolvedValue({ articles: [], total: 0, limit: 5, offset: 0, hasMore: false });
    const onSearch = renderBox();
    const input = screen.getByRole("searchbox", { name: "Search articles" });

    fireEvent.focus(input);
    fireEvent.change(input, { target: { value: "g" } });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(300);
    });
    expect(searchArticles).not.toHaveBeenCalled();

    fireEvent.change(input, { target: { value: "gate" } });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(300);
    });
    expect(await screen.findByText("No article titles match. Press Enter to search anyway.")).toBeInTheDocument();

    fireEvent.submit(input.closest("form")!);
    expect(onSearch).toHaveBeenCalledWith("gate");
  });

  it("clears the field from the clear button", () => {
    renderBox();
    const input = screen.getByRole("searchbox", { name: "Search articles" });
    fireEvent.change(input, { target: { value: "abc" } });
    fireEvent.click(screen.getByRole("button", { name: "Clear search" }));
    expect(input).toHaveValue("");
  });
});
