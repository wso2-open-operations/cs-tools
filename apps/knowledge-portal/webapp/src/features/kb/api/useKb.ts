// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.

import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import {
  getArticle,
  getRelatedArticles,
  listKnowledgeBases,
  recordView,
  searchArticles,
} from "@features/kb/api/kbClient";
import type { SearchKBArticlesRequest } from "@features/kb/types/kb";

const KB_KEY = "kb";

export const useKnowledgeBases = () =>
  useQuery({
    queryKey: [KB_KEY, "knowledge-bases"],
    queryFn: listKnowledgeBases,
    staleTime: 5 * 60 * 1000,
  });

export const useSearchArticles = (req: SearchKBArticlesRequest) =>
  useQuery({
    queryKey: [KB_KEY, "articles", req],
    queryFn: () => searchArticles(req),
  });

export const useArticle = (id: string | undefined) =>
  useQuery({
    queryKey: [KB_KEY, "article", id ?? ""],
    queryFn: () => getArticle(id as string),
    enabled: Boolean(id),
  });

export const useRelatedArticles = (id: string | undefined) =>
  useQuery({
    queryKey: [KB_KEY, "related", id ?? ""],
    queryFn: () => getRelatedArticles(id as string),
    enabled: Boolean(id),
    staleTime: 5 * 60 * 1000,
  });

const VIEWED_KEY_PREFIX = "kb-viewed:";

export const useRecordArticleView = (id: string | undefined): void => {
  useEffect(() => {
    if (!id) return;
    const key = VIEWED_KEY_PREFIX + id;
    try {
      if (localStorage.getItem(key)) return;
      localStorage.setItem(key, "1");
    } catch {
      // localStorage unavailable -- count the view rather than drop it
    }
    void recordView(id);
  }, [id]);
};
