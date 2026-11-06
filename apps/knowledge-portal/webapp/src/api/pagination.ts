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

import { MAX_OFFSET } from "@api/backend/types";
import type { SearchKBArticlesResponse } from "@api/backend/types";

export const PAGE_SIZE = 20;

/**
 * Offset of the page after `last`, or undefined when there is none. The
 * backend reports `hasMore`, but also refuses any offset above MAX_OFFSET, so
 * a very large result set stops there instead of requesting a page that would
 * be rejected.
 */
export function getNextOffset(last: SearchKBArticlesResponse): number | undefined {
  if (!last.hasMore) return undefined;
  // A page that reports hasMore but a non-positive limit would repeat the
  // same offset forever.
  if (!(last.limit > 0)) return undefined;
  const next = last.offset + last.limit;
  return next <= MAX_OFFSET ? next : undefined;
}

/**
 * Without a knowledge base filter the backend filters each upstream page
 * after fetching it, so a page can come back short or empty while hasMore is
 * still true. This many consecutive empty pages are fetched automatically
 * before the user is handed a manual "Load more".
 */
export const MAX_AUTO_EMPTY_PAGES = 5;

/** Number of empty pages at the end of `pages`. */
export function trailingEmptyPages(pages: SearchKBArticlesResponse[]): number {
  let count = 0;
  for (let i = pages.length - 1; i >= 0 && pages[i].articles.length === 0; i--) count++;
  return count;
}
