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

import { MAX_SEARCH_QUERY_LENGTH } from "@api/backend/types";

// The backend rejects control characters and counts characters (code points),
// not UTF-16 units, so the same rules are applied before a query is sent.
// eslint-disable-next-line no-control-regex
const CONTROL_CHARS = /[\u0000-\u001f\u007f-\u009f]/g;

/** Trim, drop control characters and cap at the backend's 200-character limit. */
export function normalizeSearchQuery(raw: string | null | undefined): string {
  if (!raw) return "";
  const cleaned = raw.replace(CONTROL_CHARS, " ").trim();
  return Array.from(cleaned).slice(0, MAX_SEARCH_QUERY_LENGTH).join("").trim();
}
