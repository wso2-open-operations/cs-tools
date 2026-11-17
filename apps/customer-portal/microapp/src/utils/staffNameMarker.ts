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

// Mirrors the webapp's apps/customer-portal/webapp/src/utils/staffNameMarker.ts: keep the two in step.

/**
 * Some internal (WSO2 staff) users carry a trailing circled "W" in their stored name (e.g.
 * "Jane Doe ⓦ"). That is upstream data, not something this app writes. It shows as either the
 * lowercase (U+24E6) or uppercase (U+24CC) circled letter at the end of the name, optionally with
 * whitespace around it, and rarely as the whole string when there was no real name to attach it to.
 */
const STAFF_NAME_MARKER = /\s*[Ⓦⓦ]\s*$/u;

/**
 * Strips the trailing circled-W marker from a display name, so text and avatar initials only ever
 * show the person's actual name. A name without the marker passes through unchanged, trimmed.
 */
export function stripStaffNameMarker(name: string | null | undefined): string {
  if (!name) return "";
  return name.replace(STAFF_NAME_MARKER, "").trim();
}

/**
 * Avatar initials: the first letters of the first two words of the name, with any staff marker
 * removed first. A third word, or any other trailing marker, never reaches the avatar.
 */
export function nameInitials(name: string | null | undefined): string {
  return (
    stripStaffNameMarker(name)
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      // The first code point, not the first UTF-16 unit: charAt(0) would split a character outside the
      // basic plane (for example 𠮷) into a broken half.
      .map((word) => Array.from(word)[0])
      .join("")
      .toUpperCase()
  );
}
