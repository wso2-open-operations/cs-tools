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

/**
 * A KB article's lifecycle stage. Mirrors the Go backend's
 * `kb_article_state_enum`; the DB trigger (`trg_kb_article_valid_transition`)
 * and the backend's role check are the real enforcement — this type only
 * constrains what the frontend can *request*.
 */
export type KBArticleState =
  | "draft"
  | "pending_review"
  | "published"
  | "retired";

export interface KBArticle {
  id: string;
  knowledgeBaseId: string;
  title: string;
  body: string;
  state: KBArticleState;
  authorId: string;
  // Set once a manager has acted on this article. Null while still in draft.
  reviewerId: string | null;
  // Set only when this article was auto-drafted from a closed case (future
  // Kafka-driven entry point). Null for manually created articles.
  sourceCaseId: string | null;
  // The manager's reason for the most recent rejection. Null unless the
  // article was rejected and has not since been resubmitted.
  rejectionComment: string | null;
  updatedBy: string | null;
  teamKey: string | null;
  createdOn: string;
  updatedOn: string;
  submittedOn: string | null;
  publishedOn: string | null;
  retiredOn: string | null;
}

export interface CreateKBArticleRequest {
  knowledgeBaseId: string;
  title: string;
  body: string;
  authorId: string;
  teamKey?: string;
}

export interface CreateKBArticleResponse {
  article: KBArticle;
}

export interface SearchKBArticlesRequest {
  knowledgeBaseId?: string;
  states?: KBArticleState[];
  authorId?: string;
  teamKeys?: string[];
  searchQuery?: string;
  pagination: {
    limit: number;
    offset: number;
  };
}

export interface SearchKBArticlesResponse {
  articles: KBArticle[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

export interface UpdateKBArticleStateRequest {
  state: KBArticleState;
  // Required by convention when state is "draft" as the result of a
  // rejection (pending_review -> draft); optional/ignored for every other
  // transition. The backend does not enforce this — it is a UX convention
  // for the review-queue form, not a validated contract field.
  rejectionComment?: string;
}

export interface UpdateKBArticleStateResponse {
  article: KBArticle;
}

export interface UpdateKBArticleContentRequest {
  title: string;
  body: string;
}

export interface KnowledgeBase {
  id: string;
  productId: string | null;
  name: string;
  isActive: boolean;
  createdOn: string;
  updatedOn: string;
}

export interface CreateKnowledgeBaseRequest {
  productId?: string;
  name: string;
}

export interface UpdateKnowledgeBaseRequest {
  name: string;
}

export interface UpdateKnowledgeBaseActiveRequest {
  isActive: boolean;
}

export interface KBManager {
  id: string;
  knowledgeBaseId: string;
  userId: string;
  createdOn: string;
}

export interface SearchKBManagersRequest {
  knowledgeBaseId?: string;
  userId?: string;
}

export interface SearchKBManagersResponse {
  managers: KBManager[];
}

export interface CreateKBManagerRequest {
  knowledgeBaseId: string;
  userId: string;
}

export interface ListKnowledgeBasesResponse {
  knowledgeBases: KnowledgeBase[];
}

export interface KBArticleHistoryEntry {
  id: string;
  kbArticleId: string;
  title: string;
  body: string;
  state: KBArticleState;
  changedBy: string;
  createdOn: string;
}
