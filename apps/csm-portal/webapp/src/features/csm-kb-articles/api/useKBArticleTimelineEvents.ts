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

import { useMemo } from "react";
import { useUsersByIds } from "@features/csm-kb-articles/api/useUsersByIds";
import { useGetKBArticleHistory } from "@features/csm-kb-articles/api/useGetKBArticleHistory";
import type { KBArticle, KBArticleHistoryEntry } from "@features/csm-kb-articles/types/csmKbArticles";

export interface KBArticleTimelineEvent {
  id: string;
  label: string;
  color: "default" | "success" | "error" | "warning";
  actorId: string;
  timestamp: string;
  title: string;
  body: string;
  previousTitle: string | null;
  previousBody: string | null;
}

/**
 * Labels a transition from `fromState` to `toState` in plain language for a
 * reader, rather than showing raw enum values. `pending_review -> draft`
 * is ambiguous on its own (it's the shape of both a manager's rejection and
 * an author reopening a published article for a revision) -- the two are
 * told apart by which prior state preceded them.
 */
function transitionLabel(fromState: string, toState: string): string {
  if (toState === "pending_review") return "Submitted for review";
  if (toState === "published") return "Approved & published";
  if (toState === "retired") return "Retired";
  if (toState === "draft") {
    if (fromState === "pending_review") return "Rejected";
    if (fromState === "published") return "Revised (moved back to draft)";
  }
  return "Updated";
}

/** Colors the timeline chip by what actually happened, matching the same
 * scheme used everywhere else in the KB feature. Keyed off the *label*
 * rather than the raw state, since "pending_review -> draft" alone can't
 * distinguish a rejection from an author reopening a published article. */
function transitionColor(label: string): "default" | "success" | "error" | "warning" {
  if (label === "Approved & published") return "success";
  if (label === "Rejected") return "error";
  if (label === "Submitted for review") return "warning";
  return "default";
}

/**
 * Shared timeline-building logic for an article: creation (from the
 * article record itself -- kb_article_history has no row for creation,
 * only for later transitions) followed by every real transition since,
 * newest first, each carrying its predecessor's title/body for diffing.
 * Used by both the quick-preview history drawer and the full detail page,
 * so the two never disagree about what happened.
 */
export function useKBArticleTimelineEvents(
  article: KBArticle | null,
): { events: KBArticleTimelineEvent[]; nameById: Map<string, string> } {
  const { data: history } = useGetKBArticleHistory(article?.id);

  const events = useMemo<KBArticleTimelineEvent[]>(() => {
    if (!article) return [];
    const rows: KBArticleHistoryEntry[] = [...(history?.history ?? [])].sort(
      (a, b) => new Date(a.createdOn).getTime() - new Date(b.createdOn).getTime(),
    );

    const built: KBArticleTimelineEvent[] = [];
    let prevTitle: string | null = null;
    let prevBody: string | null = null;
    let prevState = "draft";

    for (const row of rows) {
      const label = transitionLabel(prevState, row.state);
      built.push({
        id: row.id,
        label,
        color: transitionColor(label),
        actorId: row.changedBy,
        timestamp: row.createdOn,
        title: row.title,
        body: row.body,
        previousTitle: prevTitle,
        previousBody: prevBody,
      });
      prevTitle = row.title;
      prevBody = row.body;
      prevState = row.state;
    }

    const withCreation: KBArticleTimelineEvent[] = [
      {
        id: "created",
        label: "Created",
        color: "default",
        actorId: article.authorId,
        timestamp: article.createdOn,
        title: article.title,
        body: article.body,
        previousTitle: null,
        previousBody: null,
      },
      ...built,
    ];

    return withCreation.reverse();
  }, [article, history]);

  const actorIds = useMemo(() => Array.from(new Set(events.map((e) => e.actorId))), [events]);
  const { data: nameById = new Map<string, string>() } = useUsersByIds(actorIds);

  return { events, nameById };
}
