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
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
import { shouldRetryQuery } from "@api/queryRetry";
import KBArticlePage from "@features/kb/pages/KBArticlePage";
import KBBrowsePage from "@features/kb/pages/KBBrowsePage";

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

const renderAt = (path: string) => {
  // Same retry policy as production, with no delay so a retried 5xx settles fast.
  const client = new QueryClient({
    defaultOptions: { queries: { retry: shouldRetryQuery, retryDelay: 0 } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/knowledge-articles/:id" element={<KBArticlePage />} />
          <Route path="/knowledge-bases" element={<KBBrowsePage />} />
          <Route path="/knowledge-bases/:id" element={<KBBrowsePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
};

beforeEach(() => {
  vi.mocked(getArticle).mockReset();
  vi.mocked(searchArticles).mockReset();
  vi.mocked(listKnowledgeBases).mockResolvedValue({ knowledgeBases: [] });
  vi.mocked(getRelatedArticles).mockResolvedValue({ articles: [] });
  vi.mocked(countArticles).mockResolvedValue(3);
});

describe("KBArticlePage errors", () => {
  it("shows not found for a 404 without retrying or offering retry", async () => {
    vi.mocked(getArticle).mockRejectedValue(new BackendApiError(404, "Article not found."));
    renderAt("/knowledge-articles/abc");
    expect(await screen.findByRole("heading", { level: 1, name: "Article not found" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
    expect(getArticle).toHaveBeenCalledTimes(1);
  });

  it("does not call the backend for an id it would reject", async () => {
    renderAt("/knowledge-articles/bad%20id");
    expect(await screen.findByRole("heading", { level: 1, name: "Article not found" })).toBeInTheDocument();
    expect(getArticle).not.toHaveBeenCalled();
  });

  it("offers a retry button for a 5xx and recovers on retry", async () => {
    const article = {
      id: "abc",
      knowledgeBaseId: "kb",
      title: "Recovered article",
      body: "<p>Body</p>",
      createdOn: "2026-01-01T00:00:00Z",
      updatedOn: "2026-01-01T00:00:00Z",
    };
    vi.mocked(getArticle)
      .mockRejectedValueOnce(new BackendApiError(502, "bad gateway"))
      .mockRejectedValueOnce(new BackendApiError(502, "bad gateway"))
      .mockRejectedValueOnce(new BackendApiError(502, "bad gateway"))
      .mockResolvedValue(article);
    renderAt("/knowledge-articles/abc");
    const retry = await screen.findByRole("button", { name: "Try again" });
    expect(screen.queryByText("Article not found")).toBeNull();
    fireEvent.click(retry);
    expect(await screen.findByRole("heading", { level: 1, name: "Recovered article" })).toBeInTheDocument();
    await waitFor(() => expect(document.title).toBe("Recovered article - WSO2 Knowledge Base"));
  });
});

describe("KBBrowsePage", () => {
  it("renders results as links and loads more", async () => {
    const mk = (id: string) => ({
      id,
      knowledgeBaseId: "kb",
      title: `Title ${id}`,
      body: "<style>x{}</style><p>Snippet</p>",
      createdOn: "2026-01-01T00:00:00Z",
      updatedOn: "2026-01-01T00:00:00Z",
    });
    vi.mocked(searchArticles)
      .mockResolvedValueOnce({ articles: [mk("a")], total: 2, limit: 20, offset: 0, hasMore: true })
      .mockResolvedValueOnce({ articles: [mk("b")], total: 2, limit: 20, offset: 20, hasMore: false });
    renderAt("/knowledge-bases?q=snip");

    const link = await screen.findByRole("link", { name: /Title a/ });
    expect(link).toHaveAttribute("href", "/knowledge-articles/a");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent('Search results for "snip"');
    expect(screen.getByRole("searchbox", { name: "Search articles" })).toBeInTheDocument();
    expect(screen.queryByText(/x\{\}/)).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(await screen.findByRole("link", { name: /Title b/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Title a/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });
});
