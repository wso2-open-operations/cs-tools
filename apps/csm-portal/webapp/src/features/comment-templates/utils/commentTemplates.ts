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

import type {
  BeCommentTemplate,
  BeCommentTemplateAppliesTo,
} from "@api/backend/types";

/** Limits entity-service enforces (migration 0293), checked here first. */
export const COMMENT_TEMPLATE_NAME_MAX = 100;
export const COMMENT_TEMPLATE_BODY_MAX_BYTES = 20000;

export const APPLIES_TO_LABELS: Record<BeCommentTemplateAppliesTo, string> = {
  COMMENT: "Customer comment",
  WORK_NOTE: "Work note",
  BOTH: "Comment or work note",
};

/**
 * The templates the comment box offers in its current mode: a work note gets
 * WORK_NOTE and BOTH, a customer comment gets COMMENT and BOTH, so internal
 * wording is never offered for a public reply.
 */
export function templatesForMode(
  templates: BeCommentTemplate[],
  internal: boolean,
): BeCommentTemplate[] {
  const wanted: BeCommentTemplateAppliesTo = internal ? "WORK_NOTE" : "COMMENT";
  return templates.filter((t) => t.appliesTo === "BOTH" || t.appliesTo === wanted);
}

/** Elements whose end separates words in plain text. */
const BLOCK_BOUNDARY_SELECTOR =
  "p,div,br,li,ul,ol,h1,h2,h3,h4,h5,h6,blockquote,pre,table,tr,td,th,hr";

/**
 * A template's text without markup, whitespace collapsed, for list previews
 * and search. Parsed with DOMParser, whose document is inert (no script runs,
 * no image loads), and read as textContent, so nothing is rendered as HTML.
 */
export function templatePlainText(html: string): string {
  if (!html) return "";
  const doc = new DOMParser().parseFromString(html, "text/html");
  // Blocks and line breaks become a space, so "<p>Next</p><p>step</p>" reads
  // "Next step"; inline elements (<b>, <a>) stay joined to their words.
  doc.body.querySelectorAll(BLOCK_BOUNDARY_SELECTOR).forEach((el) => el.after(" "));
  return (doc.body.textContent ?? "").replace(/\s+/g, " ").trim();
}

/** Case-insensitive match on the name or the text. */
export function filterTemplates(
  templates: BeCommentTemplate[],
  query: string,
): BeCommentTemplate[] {
  const q = query.trim().toLowerCase();
  if (!q) return templates;
  return templates.filter(
    (t) =>
      t.name.toLowerCase().includes(q) ||
      templatePlainText(t.body).toLowerCase().includes(q),
  );
}

/** UTF-8 size of a body, compared with COMMENT_TEMPLATE_BODY_MAX_BYTES. */
export function bodyBytes(html: string): number {
  return new TextEncoder().encode(html).length;
}

/** True when an editor body holds no text (an empty paragraph counts as none). */
export function isBlankBody(html: string): boolean {
  return templatePlainText(html) === "";
}

/**
 * True when the body holds an image. A template cannot keep one: an image in a
 * draft is a local blob: preview until its comment is sent, and that preview
 * is gone by the time anyone inserts the template, so the send would fail.
 */
export function hasImage(html: string): boolean {
  return /<img\b/i.test(html);
}
