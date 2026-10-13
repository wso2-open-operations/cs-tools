// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

export interface KnowledgeBase {
  id: string;
  title: string;
  active: boolean;
}

export interface ListKnowledgeBasesResponse {
  knowledgeBases: KnowledgeBase[];
}

export interface KBArticle {
  id: string;
  knowledgeBaseId: string;
  title: string;
  body: string;
  state: string;
  number?: string | null;
  createdOn: string;
  updatedOn: string;
  publishedOn?: string | null;
  viewCount?: number | null;
  rating?: number | null;
}

export type KBSortOrder = "recent" | "mostViewed" | "topRated";

export interface SearchKBArticlesRequest {
  knowledgeBaseId?: string;
  searchQuery?: string;
  sortBy?: KBSortOrder;
  limit?: number;
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
