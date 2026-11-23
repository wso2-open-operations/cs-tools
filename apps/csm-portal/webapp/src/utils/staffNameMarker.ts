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

/**
 * Some internal (WSO2 staff) users carry a trailing circled "W" in their
 * stored name (e.g. "Jane Doe ⓦ") — upstream data, not something this
 * codebase writes (see `apps/customer-portal/webapp/src/utils/pdfText.ts`'s
 * identical marker for the PDF-export equivalent of this same fix). Seen as
 * either the lowercase (U+24E6, "ⓦ") or uppercase (U+24CC, "Ⓦ") circled
 * letter, optionally surrounded by whitespace, at the end of the name — and,
 * in a rarer case, as the entire string when there was no real name to
 * attach it to. It can also occur before a surname or a display suffix
 * such as "(you)", and may carry invisible presentation characters.
 */
const STAFF_NAME_MARKER = /[Ⓦⓦ][\uFE0E\uFE0F\u200B\u200C\u200D]*/gu;

/**
 * Strips circled-W markers (see {@link STAFF_NAME_MARKER}) from a
 * display name, so avatars/initials and rendered text only ever show the
 * person's actual first and last name. Safe to call on any string — a name
 * without the marker passes through unchanged (aside from trimming).
 */
export function stripStaffNameMarker(name: string | null | undefined): string {
  if (!name) return "";
  return name.replace(STAFF_NAME_MARKER, "").replace(/\s+/gu, " ").trim();
}

/**
 * Cleans each name part before joining the non-empty parts with a space.
 */
export function joinStaffName(
  firstName: string | null | undefined,
  lastName: string | null | undefined,
): string {
  return [stripStaffNameMarker(firstName), stripStaffNameMarker(lastName)]
    .filter(Boolean)
    .join(" ");
}

/**
 * Whether a display name carries the circled-W staff marker (see
 * {@link STAFF_NAME_MARKER}). Lets a place that only has a name (a dashboard
 * group label, a list cell) show the staff badge in its place. A separate,
 * non-global pattern: `test` on the global one keeps `lastIndex` between calls.
 */
export function hasStaffNameMarker(name: string | null | undefined): boolean {
  return !!name && /[Ⓦⓦ]/u.test(name);
}

/**
 * A name for display without the staff marker, changing nothing else: a name
 * without the marker comes back exactly as given (its spacing included), and
 * only a marked name goes through {@link stripStaffNameMarker}'s clean-up.
 * Empty for no name.
 */
export function withoutStaffNameMarker(name: string | null | undefined): string {
  if (!name) return "";
  return hasStaffNameMarker(name) ? stripStaffNameMarker(name) : name;
}
