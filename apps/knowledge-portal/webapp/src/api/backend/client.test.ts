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

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  BackendApiError,
  getArticle,
  isClientError,
  isNotFoundError,
  listKnowledgeBases,
  searchArticles,
} from "@api/backend/client";
import { shouldRetryQuery } from "@api/queryRetry";

const json = (status: number, body: unknown, statusText = ""): Response =>
  new Response(JSON.stringify(body), { status, statusText });

const fetchMock = vi.fn();

beforeEach(() => {
  window.config = { KNOWLEDGE_PORTAL_BACKEND_BASE_URL: "https://api.example.com/" };
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  fetchMock.mockReset();
  vi.unstubAllGlobals();
  window.config = undefined;
});

describe("client", () => {
  it("calls the configured base URL and returns the body", async () => {
    fetchMock.mockResolvedValue(json(200, { knowledgeBases: [] }));
    await expect(listKnowledgeBases()).resolves.toEqual({ knowledgeBases: [] });
    expect(fetchMock.mock.calls[0][0]).toBe("https://api.example.com/public/knowledge-bases");
  });

  it("posts the search body as JSON", async () => {
    fetchMock.mockResolvedValue(json(200, { articles: [], total: 0, limit: 20, offset: 0, hasMore: false }));
    await searchArticles({ searchQuery: "x", limit: 20, offset: 0 });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("https://api.example.com/public/kb-articles/search");
    expect(init.method).toBe("POST");
    expect(init.headers["Content-Type"]).toBe("application/json");
    expect(JSON.parse(init.body)).toEqual({ searchQuery: "x", limit: 20, offset: 0 });
  });

  it("encodes ids in the path", async () => {
    fetchMock.mockResolvedValue(json(200, {}));
    await getArticle("a/b");
    expect(fetchMock.mock.calls[0][0]).toBe("https://api.example.com/public/kb-articles/a%2Fb");
  });

  it("maps a 404 envelope to a BackendApiError carrying status and message", async () => {
    fetchMock.mockResolvedValue(json(404, { message: "Article not found." }));
    const error = await getArticle("x").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(BackendApiError);
    expect((error as BackendApiError).status).toBe(404);
    expect((error as BackendApiError).message).toBe("Article not found.");
    expect(isNotFoundError(error)).toBe(true);
    expect(isClientError(error)).toBe(true);
  });

  it("maps a 502 with a non-JSON body to a server error", async () => {
    fetchMock.mockResolvedValue(new Response("<html>bad gateway</html>", { status: 502, statusText: "Bad Gateway" }));
    const error = (await getArticle("x").catch((e: unknown) => e)) as BackendApiError;
    expect(error.status).toBe(502);
    expect(error.message).toBe("Bad Gateway");
    expect(isNotFoundError(error)).toBe(false);
    expect(isClientError(error)).toBe(false);
  });

  it("maps a network failure to status 0", async () => {
    fetchMock.mockRejectedValue(new TypeError("Failed to fetch"));
    const error = (await getArticle("x").catch((e: unknown) => e)) as BackendApiError;
    expect(error).toBeInstanceOf(BackendApiError);
    expect(error.status).toBe(0);
  });

  it("rethrows an abort untouched", async () => {
    fetchMock.mockRejectedValue(new DOMException("aborted", "AbortError"));
    const error = await getArticle("x").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DOMException);
  });

  it("throws a ConfigError instead of falling back when config is missing", async () => {
    window.config = undefined;
    await expect(getArticle("x")).rejects.toThrow(/KNOWLEDGE_PORTAL_BACKEND_BASE_URL/);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("shouldRetryQuery", () => {
  it("never retries a 4xx", () => {
    expect(shouldRetryQuery(0, new BackendApiError(404, "nf"))).toBe(false);
    expect(shouldRetryQuery(0, new BackendApiError(400, "bad"))).toBe(false);
  });

  it("retries 5xx and network errors a limited number of times", () => {
    expect(shouldRetryQuery(0, new BackendApiError(502, "x"))).toBe(true);
    expect(shouldRetryQuery(1, new BackendApiError(0, "x"))).toBe(true);
    expect(shouldRetryQuery(2, new BackendApiError(504, "x"))).toBe(false);
  });
});
