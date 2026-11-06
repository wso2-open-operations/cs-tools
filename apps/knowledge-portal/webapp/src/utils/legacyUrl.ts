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

import { isValidId } from "@api/backend/types";
import { browseUrl, articleUrl } from "@utils/browseUrl";
import { normalizeSearchQuery } from "@utils/searchQuery";

const LEGACY_ID_PATTERN = /^[0-9a-f]{32}$/i;

/** Legacy id (32 hex chars) as a lowercase 8-4-4-4-12 uuid, or null if malformed. */
export function legacyIdToUuid(raw: string | null): string | null {
  if (raw === null || !LEGACY_ID_PATTERN.test(raw)) return null;
  const h = raw.toLowerCase();
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

/**
 * Target for a legacy knowledge site URL (`/kb?id=...`), or null when it
 * cannot be mapped. Only internal paths built from validated ids are returned.
 */
export function legacyKbTarget(search: string): string | null {
  const params = new URLSearchParams(search);
  const id = params.get("id");
  if (id === "kb_search") {
    const rawKb = params.get("kb_knowledge_base");
    const q = normalizeSearchQuery(params.get("q") ?? params.get("keyword"));
    if (rawKb === null || rawKb === "") return browseUrl({ q });
    const knowledgeBaseId = legacyIdToUuid(rawKb);
    return knowledgeBaseId ? browseUrl({ knowledgeBaseId, q }) : null;
  }
  if (id === "kb_article_view") {
    const articleId = legacyIdToUuid(params.get("sys_kb_id"));
    return articleId ? articleUrl(articleId) : null;
  }
  return null;
}

/** Target for the previous `/browse?knowledgeBaseId=&q=` format. Never null. */
export function previousBrowseTarget(search: string): string {
  const params = new URLSearchParams(search);
  const rawKb = params.get("knowledgeBaseId");
  return browseUrl({
    knowledgeBaseId: isValidId(rawKb) ? rawKb : undefined,
    q: normalizeSearchQuery(params.get("q")),
  });
}

/** Target for the previous `/articles/:id` format, or null for an invalid id. */
export const previousArticleTarget = (id: string | undefined): string | null =>
  isValidId(id) ? articleUrl(id) : null;
