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

const escapeRegExp = (s: string): string => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/** Distinct whitespace-separated terms of a query, longest first. */
export function searchTerms(query: string | undefined | null): string[] {
  if (!query) return [];
  const terms = Array.from(new Set(query.trim().split(/\s+/).filter((t) => t.length > 0)));
  return terms.sort((a, b) => b.length - a.length);
}

export interface TextPart {
  text: string;
  match: boolean;
}

/**
 * Splits `text` into runs that do and do not match any term of `query`,
 * case-insensitively, so a result title can emphasise what was searched for.
 * Terms are matched literally: regex characters in the query are escaped.
 */
export function splitByTerms(text: string, query: string | undefined | null): TextPart[] {
  const terms = searchTerms(query);
  if (!text || terms.length === 0) return [{ text, match: false }];
  const pattern = new RegExp(`(${terms.map(escapeRegExp).join("|")})`, "giu");
  // With a capturing group, odd indexes of split() are the matches.
  return text
    .split(pattern)
    .map((part, index) => ({ text: part, match: index % 2 === 1 }))
    .filter((part) => part.text !== "");
}
