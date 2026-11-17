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

import { escapeHtml } from "@utils/sanitizeHtml";

/** What the ETA being shared is about; only changes the template's opening line. */
export type FixEtaCommentKind = "fix" | "report_analysis";

export const FIX_ETA_COMMENT_KIND_LABEL: Record<FixEtaCommentKind, string> = {
  fix: "Fix",
  report_analysis: "Report analysis",
};

const OPENING_LINE: Record<FixEtaCommentKind, string> = {
  fix: "ETA information for the fix will be as follows.",
  report_analysis:
    "ETA information for the report analysis will be as follows.",
};

export interface FixEtaCommentFields {
  kind: FixEtaCommentKind;
  /** Product display name; omitted from the text while blank. */
  product?: string;
  /** Public issue URL; omitted from the text while blank. */
  publicTicket?: string;
  /** Date-only "YYYY-MM-DD" strings; each line is omitted while blank. */
  bestCaseFixEta?: string;
  mostLikelyFixEta?: string;
  worstCaseFixEta?: string;
}

/**
 * Plain-text customer-visible comment pre-filled in the Set fix ETA dialog:
 * an opening line, a blank line, then one "Label: value" line per field that
 * has a value. Same wording the case's share-ETA comment has always used (only
 * the opening line varies by `kind`), so the default `fix` output is
 * unchanged. Newlines are real `\n`; see `fixEtaCommentTextToHtml` for the
 * posted form.
 */
export function buildFixEtaCommentTemplate(fields: FixEtaCommentFields): string {
  const lines: string[] = [];
  if (fields.product) lines.push(`Product: ${fields.product}`);
  if (fields.publicTicket) lines.push(`Public git issue: ${fields.publicTicket}`);
  if (fields.bestCaseFixEta) {
    lines.push(`Best Case Estimate: ${fields.bestCaseFixEta}`);
  }
  if (fields.mostLikelyFixEta) {
    lines.push(`Most Likely Estimate: ${fields.mostLikelyFixEta}`);
  }
  if (fields.worstCaseFixEta) {
    lines.push(`Worst Case Estimate: ${fields.worstCaseFixEta}`);
  }
  return [OPENING_LINE[fields.kind], ...(lines.length ? [lines.join("\n")] : [])].join(
    "\n\n",
  );
}

/**
 * Converts the (possibly engineer-edited) plain text into the HTML body the
 * comment endpoint stores: HTML-escaped (the text is free-form input that ends
 * up customer-visible), blank-line-separated blocks as `<p>`, single newlines
 * as `<br>`. For an unedited template this yields exactly
 * `<p>opening line</p><p>line<br>line…</p>`.
 */
export function fixEtaCommentTextToHtml(text: string): string {
  return text
    .replace(/\r\n?/g, "\n")
    .trim()
    .split(/\n{2,}/)
    .map((block) => block.trim())
    .filter((block) => block.length > 0)
    .map((block) => `<p>${escapeHtml(block).replace(/\n/g, "<br>")}</p>`)
    .join("");
}
