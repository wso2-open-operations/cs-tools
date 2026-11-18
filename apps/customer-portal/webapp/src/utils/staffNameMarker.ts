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
 * codebase writes (see `pdfText.ts`'s identical marker for the PDF-export
 * equivalent of this same fix). Seen as either the lowercase (U+24E6, "ⓦ")
 * or uppercase (U+24CC, "Ⓦ") circled letter, optionally surrounded by
 * whitespace, at the end of the name — and, in a rarer case, as the entire
 * string when there was no real name to attach it to.
 */
const STAFF_NAME_MARKER = /\s*[Ⓦⓦ]\s*$/u;

/**
 * Strips the trailing circled-W marker (see {@link STAFF_NAME_MARKER}) from a
 * display name, so avatars/initials and rendered text only ever show the
 * person's actual first and last name. Safe to call on any string — a name
 * without the marker passes through unchanged (aside from trimming).
 */
export function stripStaffNameMarker(name: string | null | undefined): string {
  if (!name) return "";
  return name.replace(STAFF_NAME_MARKER, "").trim();
}

/**
 * Strips each of `firstName`/`lastName` individually, then joins the
 * non-empty ones with a space. Deliberately NOT `stripStaffNameMarker(
 * \`${firstName} ${lastName}\`)` — stripping only once, after joining,
 * misses the marker whenever it's actually a trailing word of `firstName`
 * (e.g. firstName "Jane Ⓦ", lastName "Doe" joins to "Jane Ⓦ Doe", with the
 * marker now in the middle of the combined string, past where the
 * trailing-anchored regex looks).
 */
export function joinStaffName(
  firstName: string | null | undefined,
  lastName: string | null | undefined,
): string {
  return [stripStaffNameMarker(firstName), stripStaffNameMarker(lastName)]
    .filter(Boolean)
    .join(" ");
}
