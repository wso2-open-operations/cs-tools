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

import { getBackendBaseUrl } from "@config/apiConfig";
import type {
  ErrorPayload,
  KBArticle,
  ListKnowledgeBasesResponse,
  RelatedArticlesResponse,
  SearchKBArticlesRequest,
  SearchKBArticlesResponse,
} from "@api/backend/types";

/**
 * Error thrown for any failed backend call. `status` is the HTTP status, or 0
 * when no response arrived at all (network failure, CORS rejection, abort).
 * Callers branch on it: 404 is "not found", other 4xx are not retryable, 5xx
 * and 0 are transient.
 */
export class BackendApiError extends Error {
  status: number;
  payload?: ErrorPayload;
  constructor(status: number, message: string, payload?: ErrorPayload) {
    super(message);
    this.name = "BackendApiError";
    this.status = status;
    this.payload = payload;
  }
}

export const isNotFoundError = (error: unknown): boolean =>
  error instanceof BackendApiError && error.status === 404;

/** True for 4xx responses: the same request will fail the same way. */
export const isClientError = (error: unknown): boolean =>
  error instanceof BackendApiError &&
  error.status >= 400 &&
  error.status < 500;

async function readError(response: Response): Promise<BackendApiError> {
  let payload: ErrorPayload | undefined;
  try {
    payload = (await response.json()) as ErrorPayload;
  } catch {
    /* body is not JSON */
  }
  const message =
    (typeof payload?.message === "string" && payload.message) ||
    response.statusText ||
    `Request failed with status ${response.status}`;
  return new BackendApiError(response.status, message, payload);
}

interface RequestOptions {
  method?: "GET" | "POST";
  body?: unknown;
  signal?: AbortSignal;
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, signal } = options;
  // Resolved outside the try: a missing or malformed config must surface as a
  // ConfigError naming the key, not be reported as a network failure.
  const url = getBackendBaseUrl() + path;
  let response: Response;
  try {
    response = await fetch(url, {
      method,
      signal,
      headers: {
        Accept: "application/json",
        ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch (cause) {
    // An abort is the caller's own doing (query cancelled): rethrow as-is.
    if (cause instanceof DOMException && cause.name === "AbortError") {
      throw cause;
    }
    throw new BackendApiError(0, "Could not reach the knowledge base service.");
  }
  if (!response.ok) throw await readError(response);
  try {
    return (await response.json()) as T;
  } catch {
    throw new BackendApiError(
      response.status,
      "The knowledge base service returned an unreadable response.",
    );
  }
}

export const listKnowledgeBases = (
  signal?: AbortSignal,
): Promise<ListKnowledgeBasesResponse> =>
  request<ListKnowledgeBasesResponse>("/public/knowledge-bases", { signal });

export const searchArticles = (
  body: SearchKBArticlesRequest,
  signal?: AbortSignal,
): Promise<SearchKBArticlesResponse> =>
  request<SearchKBArticlesResponse>("/public/kb-articles/search", {
    method: "POST",
    body,
    signal,
  });

/**
 * Number of published articles in one knowledge base. With a knowledge base
 * filter the backend's `total` is exact, so a page of one is enough.
 */
export const countArticles = (knowledgeBaseId: string, signal?: AbortSignal): Promise<number> =>
  searchArticles({ knowledgeBaseId, limit: 1 }, signal).then((response) => response.total);

export const getArticle = (id: string, signal?: AbortSignal): Promise<KBArticle> =>
  request<KBArticle>(`/public/kb-articles/${encodeURIComponent(id)}`, { signal });

export const getRelatedArticles = (
  id: string,
  signal?: AbortSignal,
): Promise<RelatedArticlesResponse> =>
  request<RelatedArticlesResponse>(
    `/public/kb-articles/${encodeURIComponent(id)}/related`,
    { signal },
  );
