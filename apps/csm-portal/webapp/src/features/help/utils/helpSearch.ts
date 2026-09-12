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

import { HELP_TOPIC_CONTENT } from "@features/help/utils/helpContent";

/**
 * Strips common Markdown syntax down to plain, readable text — good enough
 * for search matching and a snippet preview, not a full Markdown parse (no
 * need to reach for `markdown-it` here: this never gets rendered as HTML,
 * just matched against and sliced for display).
 */
export function stripMarkdownToPlainText(source: string): string {
  return source
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/`([^`]+)`/g, "$1")
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .replace(/^#{1,6}\s+/gm, "")
    .replace(/[*_]{1,3}([^*_]+)[*_]{1,3}/g, "$1")
    .replace(/^\s*[-*+]\s+/gm, "")
    .replace(/^\s*\d+\.\s+/gm, "")
    .replace(/\|/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

/** Every Help topic's Markdown, pre-stripped to plain text once at module
 * load (13 short files — cheap) rather than on every keystroke. */
export const HELP_TOPIC_PLAIN_TEXT: Record<string, string> = Object.fromEntries(
  Object.entries(HELP_TOPIC_CONTENT).map(([id, source]) => [id, stripMarkdownToPlainText(source)]),
);

/** The portion of a topic's plain-text content immediately around a search
 * match, split into the parts before/at/after the match so a caller can
 * render the middle part emphasized without its own re-matching. */
export interface TopicSnippet {
  before: string;
  match: string;
  after: string;
}

export interface TopicSearchMatch {
  /** True when `query` matched the topic's own label — the common case,
   * needing no further explanation to the searcher. */
  matchedInTitle: boolean;
  /** Only set when the match came from the topic's content rather than its
   * label, so the UI can show *why* an otherwise-unrelated-looking topic
   * matched (e.g. searching "incidents" surfacing "Operations"). */
  snippet?: TopicSnippet;
}

/** How much plain-text context to keep on each side of a content match. */
const SNIPPET_RADIUS = 60;

/**
 * Whether/how `query` matches one topic, checking its label first and its
 * full content second. Returns `null` for no match at all (topic dropped
 * from the filtered list), `{ matchedInTitle: true }` for a label hit (blank
 * `query` always counts as one, so every topic matches when the search box
 * is empty), or `{ matchedInTitle: false, snippet }` for a content-only hit.
 */
export function findTopicMatch(
  label: string,
  plainContent: string,
  query: string,
): TopicSearchMatch | null {
  const trimmed = query.trim();
  if (!trimmed) return { matchedInTitle: true };

  const lowerQuery = trimmed.toLowerCase();
  if (label.toLowerCase().includes(lowerQuery)) return { matchedInTitle: true };

  const lowerContent = plainContent.toLowerCase();
  const index = lowerContent.indexOf(lowerQuery);
  if (index === -1) return null;

  const start = Math.max(0, index - SNIPPET_RADIUS);
  const end = Math.min(plainContent.length, index + trimmed.length + SNIPPET_RADIUS);
  return {
    matchedInTitle: false,
    snippet: {
      before: (start > 0 ? "…" : "") + plainContent.slice(start, index),
      match: plainContent.slice(index, index + trimmed.length),
      after: plainContent.slice(index + trimmed.length, end) + (end < plainContent.length ? "…" : ""),
    },
  };
}
