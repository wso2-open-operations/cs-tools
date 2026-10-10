// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

import type {
  KBArticle,
  ListKnowledgeBasesResponse,
  RelatedArticlesResponse,
  SearchKBArticlesRequest,
  SearchKBArticlesResponse,
} from "@features/kb/types/kb";

const baseUrl = (): string =>
  (import.meta.env.VITE_KB_API_BASE_URL ?? "http://localhost:8085").replace(/\/$/, "");

const request = async <T>(path: string, init?: RequestInit): Promise<T> => {
  const response = await fetch(baseUrl() + path, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });
  if (!response.ok) {
    throw new Error(`Request failed (${response.status})`);
  }
  return (await response.json()) as T;
};

export const listKnowledgeBases = (): Promise<ListKnowledgeBasesResponse> =>
  request<ListKnowledgeBasesResponse>("/public/knowledge-bases");

export const searchArticles = (
  body: SearchKBArticlesRequest,
): Promise<SearchKBArticlesResponse> =>
  request<SearchKBArticlesResponse>("/public/kb-articles/search", {
    method: "POST",
    body: JSON.stringify(body),
  });

export const recordView = async (id: string): Promise<void> => {
  try {
    await fetch(`${baseUrl()}/public/kb-articles/${encodeURIComponent(id)}/view`, {
      method: "POST",
    });
  } catch {
    // ignored on purpose -- a missed view count is not worth an error
  }
};

export const getArticle = (id: string): Promise<KBArticle> =>
  request<KBArticle>(`/public/kb-articles/${encodeURIComponent(id)}`);

export const getRelatedArticles = (id: string): Promise<RelatedArticlesResponse> =>
  request<RelatedArticlesResponse>(`/public/kb-articles/${encodeURIComponent(id)}/related`);
