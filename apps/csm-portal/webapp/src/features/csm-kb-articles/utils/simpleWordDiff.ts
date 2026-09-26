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

export interface DiffToken {
  text: string;
  type: "same" | "added" | "removed";
}

/** Strips HTML tags to plain text for diffing -- article bodies are rich
 * text HTML from Lexical, and a byte-level diff of the markup itself would
 * be unreadable noise (e.g. every edit re-numbers Lexical's internal node
 * keys). Diffing the rendered text is what a reader actually cares about. */
export function stripHtml(html: string): string {
  const div = document.createElement("div");
  div.innerHTML = html;
  return (div.textContent ?? "").replace(/\s+/g, " ").trim();
}

/**
 * Word-level diff via a classic LCS (longest common subsequence) over the
 * two token arrays, then walking the LCS to emit same/added/removed runs.
 * O(n*m) -- fine for article-length text (hundreds/low-thousands of words),
 * not meant for huge documents. No external dependency: this project has no
 * existing diff library, and pulling one in for a single, bounded-size use
 * case wasn't worth the added dependency risk.
 */
export function wordDiff(oldText: string, newText: string): DiffToken[] {
  const oldWords = oldText.split(/(\s+)/).filter(Boolean);
  const newWords = newText.split(/(\s+)/).filter(Boolean);

  const m = oldWords.length;
  const n = newWords.length;
  const lcs: number[][] = Array.from({ length: m + 1 }, () => new Array<number>(n + 1).fill(0));

  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      lcs[i][j] = oldWords[i] === newWords[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
    }
  }

  const tokens: DiffToken[] = [];
  let i = 0;
  let j = 0;
  while (i < m && j < n) {
    if (oldWords[i] === newWords[j]) {
      tokens.push({ text: oldWords[i], type: "same" });
      i++;
      j++;
    } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
      tokens.push({ text: oldWords[i], type: "removed" });
      i++;
    } else {
      tokens.push({ text: newWords[j], type: "added" });
      j++;
    }
  }
  while (i < m) {
    tokens.push({ text: oldWords[i], type: "removed" });
    i++;
  }
  while (j < n) {
    tokens.push({ text: newWords[j], type: "added" });
    j++;
  }
  return tokens;
}
