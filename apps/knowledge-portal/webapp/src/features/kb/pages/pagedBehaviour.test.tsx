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
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  BackendApiError,
  countArticles,
  getArticle,
  getRelatedArticles,
  listKnowledgeBases,
  searchArticles,
} from "@api/backend/client";
import type { KBArticle, SearchKBArticlesResponse } from "@api/backend/types";
import { shouldRetryQuery } from "@api/queryRetry";
import LoadingState from "@components/LoadingState";
import KBArticlePage from "@features/kb/pages/KBArticlePage";
import KBBrowsePage from "@features/kb/pages/KBBrowsePage";
import KBHomePage from "@features/kb/pages/KBHomePage";

vi.mock("@api/backend/client", async () => {
  const actual = await vi.importActual<typeof import("@api/backend/client")>("@api/backend/client");
  return {
    ...actual,
    countArticles: vi.fn(),
    getArticle: vi.fn(),
    getRelatedArticles: vi.fn(),
    listKnowledgeBases: vi.fn(),
    searchArticles: vi.fn(),
  };
});

const article = (id: string, extra: Partial<KBArticle> = {}): KBArticle => ({
  id,
  knowledgeBaseId: "kb",
  title: `Title ${id}`,
  body: "<p>Snippet</p>",
  createdOn: "2026-01-01T00:00:00Z",
  updatedOn: "2026-01-01T00:00:00Z",
  ...extra,
});

const page = (
  ids: string[],
  offset: number,
  total: number,
  hasMore: boolean,
  limit = 20,
): SearchKBArticlesResponse => ({ articles: ids.map((id) => article(id)), total, limit, offset, hasMore });

const renderAt = (path: string) => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: shouldRetryQuery, retryDelay: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/" element={<KBHomePage />} />
          <Route path="/knowledge-articles/:id" element={<KBArticlePage />} />
          <Route path="/knowledge-bases" element={<KBBrowsePage />} />
          <Route path="/knowledge-bases/:id" element={<KBBrowsePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
};

const search = vi.mocked(searchArticles);

beforeEach(() => {
  search.mockReset();
  vi.mocked(getArticle).mockReset();
  vi.mocked(listKnowledgeBases).mockResolvedValue({ knowledgeBases: [{ id: "kb", title: "Product KB" }] });
  vi.mocked(getRelatedArticles).mockResolvedValue({ articles: [] });
  vi.mocked(countArticles).mockResolvedValue(3);
});

describe("KBBrowsePage empty pages with hasMore", () => {
  it("skips empty pages instead of showing an empty state or a misleading count", async () => {
    search
      .mockResolvedValueOnce(page([], 0, 100, true))
      .mockResolvedValueOnce(page([], 20, 100, true))
      .mockResolvedValueOnce(page(["a"], 40, 100, false));
    renderAt("/knowledge-bases");

    expect(await screen.findByRole("link", { name: /Title a/ })).toBeInTheDocument();
    expect(search).toHaveBeenCalledTimes(3);
    expect(search.mock.calls.map((c) => c[0].offset)).toEqual([0, 20, 40]);
    expect(screen.queryByText("No articles matched")).toBeNull();
    expect(screen.getByText("1 article")).toBeInTheDocument();
  });

  it("never shows the empty state while skipping", async () => {
    let release!: (v: SearchKBArticlesResponse) => void;
    search
      .mockResolvedValueOnce(page([], 0, 100, true))
      .mockImplementationOnce(() => new Promise((resolve) => { release = resolve; }));
    renderAt("/knowledge-bases");

    await waitFor(() => expect(search).toHaveBeenCalledTimes(2));
    expect(screen.queryByText("No articles matched")).toBeNull();
    expect(screen.queryByText(/0 articles/)).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("Loading articles");
    release(page(["a"], 20, 100, false));
    expect(await screen.findByRole("link", { name: /Title a/ })).toBeInTheDocument();
  });

  it("stops after 5 consecutive empty pages and offers Load more with a neutral message", async () => {
    search.mockImplementation((req) =>
      Promise.resolve(page([], req.offset ?? 0, 900, true)),
    );
    renderAt("/knowledge-bases");

    expect(await screen.findByText("No articles found yet")).toBeInTheDocument();
    expect(search).toHaveBeenCalledTimes(5);
    expect(screen.queryByText("No articles matched")).toBeNull();
    // Unfiltered total is only an upper bound, and nothing is shown yet.
    expect(screen.queryByText(/900/)).toBeNull();

    search.mockResolvedValueOnce(page(["z"], 100, 900, false));
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByRole("link", { name: /Title z/ })).toBeInTheDocument();
    expect(search).toHaveBeenCalledTimes(6);
  });

  it("says 'up to N' when no knowledge base is selected and exact N when one is", async () => {
    search.mockResolvedValue(page(["a"], 0, 57, true));
    const first = renderAt("/knowledge-bases");
    expect(await screen.findByText("Showing 1 of up to 57 articles")).toBeInTheDocument();
    first.unmount();

    search.mockResolvedValue(page(["a"], 0, 57, true));
    renderAt("/knowledge-bases/kb");
    expect(await screen.findByText("57 articles, showing 1")).toBeInTheDocument();
  });

  it("shows the empty state once hasMore is false", async () => {
    search.mockResolvedValue(page([], 0, 0, false));
    renderAt("/knowledge-bases?q=nothing");
    expect(await screen.findByText("No articles matched")).toBeInTheDocument();
  });
});

describe("KBBrowsePage load more", () => {
  it("keeps keyboard focus on the button while loading and ignores repeat clicks", async () => {
    let release!: (v: SearchKBArticlesResponse) => void;
    search
      .mockResolvedValueOnce(page(["a"], 0, 2, true))
      .mockImplementationOnce(() => new Promise((resolve) => { release = resolve; }));
    renderAt("/knowledge-bases");

    const button = await screen.findByRole("button", { name: "Load more" });
    button.focus();
    fireEvent.click(button);
    const busy = await screen.findByRole("button", { name: "Loading..." });
    expect(busy).toHaveAttribute("aria-disabled", "true");
    expect(busy).not.toBeDisabled();
    expect(busy).toHaveFocus();
    fireEvent.click(busy);
    expect(search).toHaveBeenCalledTimes(2);

    release(page(["b"], 20, 2, false));
    expect(await screen.findByRole("link", { name: /Title b/ })).toBeInTheDocument();
  });

  it("keeps earlier results when a load more fails and retries from the same button", async () => {
    search
      .mockResolvedValueOnce(page(["a"], 0, 2, true))
      .mockRejectedValue(new BackendApiError(400, "bad request"));
    renderAt("/knowledge-bases");

    fireEvent.click(await screen.findByRole("button", { name: "Load more" }));
    expect(await screen.findByText("More articles could not be loaded.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Title a/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Load more" })).toBeInTheDocument();

    search.mockReset();
    search.mockResolvedValueOnce(page(["b"], 20, 2, false));
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByRole("link", { name: /Title b/ })).toBeInTheDocument();
    expect(screen.queryByText("More articles could not be loaded.")).toBeNull();
  });

  it("deduplicates articles that shift between pages", async () => {
    search
      .mockResolvedValueOnce(page(["a", "b"], 0, 3, true, 2))
      .mockResolvedValueOnce(page(["b", "c"], 2, 3, false, 2));
    renderAt("/knowledge-bases");
    fireEvent.click(await screen.findByRole("button", { name: "Load more" }));
    await screen.findByRole("link", { name: /Title c/ });
    expect(screen.getAllByRole("link", { name: /Title b/ })).toHaveLength(1);
    expect(within(screen.getByRole("list", { name: "Articles" })).getAllByRole("listitem")).toHaveLength(3);
  });

  it("does not loop on the same offset when the backend reports limit 0 with hasMore", async () => {
    search.mockResolvedValue(page([], 0, 5, true, 0));
    renderAt("/knowledge-bases");
    expect(await screen.findByText("No articles matched")).toBeInTheDocument();
    expect(search).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });

  it("shows a refine-your-search hint when the offset cap ends paging", async () => {
    search.mockResolvedValue(page(["a"], 9990, 50000, true, 20));
    renderAt("/knowledge-bases/kb");
    expect(await screen.findByText(/Refine your search/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });

  it("shows an unknown knowledge base message for a 404", async () => {
    search.mockRejectedValue(new BackendApiError(404, "knowledge base not found"));
    renderAt("/knowledge-bases/gone");
    expect(
      await screen.findByText("This knowledge base does not exist or is no longer available."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Articles could not be loaded.")).toBeNull();
    expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
  });
});

describe("KBHomePage recent articles", () => {
  it("requests a page of 20 and shows the first 5", async () => {
    search.mockResolvedValue(
      page(["1", "2", "3", "4", "5", "6", "7"], 0, 7, false),
    );
    renderAt("/");
    expect(await screen.findByRole("link", { name: /Title 5/ })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Title 6/ })).toBeNull();
    expect(search.mock.calls[0][0]).toEqual({ sortBy: "recent", limit: 20, offset: 0 });
  });

  it("skips empty pages instead of claiming there are no articles", async () => {
    search
      .mockResolvedValueOnce(page([], 0, 40, true))
      .mockResolvedValueOnce(page(["a"], 20, 40, false));
    renderAt("/");
    expect(await screen.findByRole("link", { name: /Title a/ })).toBeInTheDocument();
    expect(screen.queryByText("No articles yet")).toBeNull();
  });

  it("shows the empty state only when there is nothing more to fetch", async () => {
    search.mockResolvedValue(page([], 0, 0, false));
    renderAt("/");
    expect(await screen.findByText("No articles yet")).toBeInTheDocument();
  });

  it("uses a neutral message with Load more when the empty-page bound is hit", async () => {
    search.mockImplementation((req) => Promise.resolve(page([], req.offset ?? 0, 900, true)));
    renderAt("/");
    expect(await screen.findByText("No recent articles to show here")).toBeInTheDocument();
    expect(screen.queryByText("No articles yet")).toBeNull();
    expect(screen.getByRole("button", { name: "Load more" })).toBeInTheDocument();
    expect(search).toHaveBeenCalledTimes(5);
  });
});

describe("document title and headings on the article page", () => {
  it("titles the not-found state itself", async () => {
    vi.mocked(getArticle).mockRejectedValue(new BackendApiError(404, "missing"));
    renderAt("/knowledge-articles/abc");
    await screen.findByRole("heading", { level: 1, name: "Article not found" });
    await waitFor(() => expect(document.title).toBe("Article not found - WSO2 Knowledge Base"));
  });

  it("titles an invalid id as not found without calling the backend", async () => {
    renderAt("/knowledge-articles/bad%20id");
    await screen.findByRole("heading", { level: 1, name: "Article not found" });
    expect(document.title).toBe("Article not found - WSO2 Knowledge Base");
  });

  it("titles the load-failure state itself", async () => {
    vi.mocked(getArticle).mockRejectedValue(new BackendApiError(400, "bad"));
    renderAt("/knowledge-articles/abc");
    await screen.findByText("The article could not be loaded.");
    await waitFor(() => expect(document.title).toBe("Article could not be loaded - WSO2 Knowledge Base"));
  });

  it("keeps a single h1 when the body contains its own h1", async () => {
    vi.mocked(getArticle).mockResolvedValue(
      article("abc", { title: "Real title", body: "<h1>Body h1</h1><h2>Sub</h2><p>x</p>" }),
    );
    renderAt("/knowledge-articles/abc");
    await screen.findByRole("heading", { level: 1, name: "Real title" });
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 2, name: "Body h1" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 3, name: "Sub" })).toBeInTheDocument();
  });
});

describe("LoadingState", () => {
  it("is a status region carrying the label as text", () => {
    render(<LoadingState label="Loading articles" />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading articles");
  });
});
