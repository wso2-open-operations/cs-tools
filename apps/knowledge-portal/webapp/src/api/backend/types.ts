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

export interface KnowledgeBase {
  id: string;
  title: string;
  /** Served as true today; inactive knowledge bases are not listed. */
  active?: boolean;
}

export interface ListKnowledgeBasesResponse {
  knowledgeBases: KnowledgeBase[];
}

export interface KBArticle {
  id: string;
  knowledgeBaseId: string;
  title: string;
  body: string;
  number?: string | null;
  createdOn: string;
  updatedOn: string;
  publishedOn?: string | null;
}

/** The only sort order the backend accepts. Omit the field for the default. */
export type KBSortOrder = "recent";

export interface SearchKBArticlesRequest {
  knowledgeBaseId?: string;
  /** At most {@link MAX_SEARCH_QUERY_LENGTH} characters. */
  searchQuery?: string;
  sortBy?: KBSortOrder;
  /** 1-50, default 20. */
  limit?: number;
  /** 0-10000. */
  offset?: number;
}

export interface SearchKBArticlesResponse {
  articles: KBArticle[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

export interface RelatedArticle {
  id: string;
  title: string;
  score: number;
}

export interface RelatedArticlesResponse {
  articles: RelatedArticle[];
}

/** Error envelope returned by every non-2xx response. */
export interface ErrorPayload {
  message?: string;
}

export const MAX_SEARCH_QUERY_LENGTH = 200;
export const MAX_PAGE_SIZE = 50;
export const MAX_OFFSET = 10000;

/** Ids the backend accepts in paths and bodies: `^[A-Za-z0-9_-]{1,64}$`. */
const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;

export const isValidId = (id: string | undefined | null): id is string =>
  typeof id === "string" && ID_PATTERN.test(id);
