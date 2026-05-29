// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
//
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

/** Decode legacy doubled-hyphen encoding inside a path segment. */
function unescapeDashSegment(raw: string): string {
  const parts = raw.split('--');
  return parts.map((chunk, i) => (i === 0 ? chunk : `-${chunk.replace(/-/g, ' ')}`)).join('');
}

/** Decode one URL path segment: `decodeURIComponent`, plus optional legacy `--` / `-` space rules. */
function decodePathSegment(segment: string): string {
  if (segment.includes('%20')) {
    return decodeURIComponent(segment);
  }
  const raw = decodeURIComponent(segment);
  if (!raw.includes('--')) {
    return raw;
  }
  return unescapeDashSegment(raw);
}

const SLUG_FOLDER_SEGMENT = /^[a-z][a-z0-9]*(-[a-z0-9]+)*$/;

/** Map `security-patches` → `Security Patches`; leaves non-slug segments unchanged. */
function slugFolderSegmentToAzureTitle(segment: string): string {
  if (!SLUG_FOLDER_SEGMENT.test(segment)) {
    return segment;
  }
  return segment
    .split('-')
    .filter((w) => w.length > 0)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase())
    .join(' ');
}

/**
 * Map the browser pathname to the share-relative path sent to **`GET /files/{id}`** (`id` = encoded path).
 * - Strips optional leading `patches` segment.
 * - Requires the last segment to end in `.pdf`.
 * - Rewrites lowercase kebab directory segments to Azure-style titled folder names; does not alter the PDF file name segment.
 *
 * @param pathname - `location.pathname` (no origin or query)
 * @returns Share-relative path, or `null` if the URL is not a valid PDF document path
 */
export function pathnameToPdfStoragePath(pathname: string): string | null {
  const trimmed = pathname.trim().replace(/\/+$/, '');
  const rawSegments = trimmed.split('/').filter((s) => s.length > 0);
  if (rawSegments.length === 0) {
    return null;
  }
  const decoded = rawSegments.map((s) => decodePathSegment(s));
  const last = decoded[decoded.length - 1];
  if (!/\.pdf$/i.test(last)) {
    return null;
  }
  let rest = decoded;
  if (rest[0] === 'patches') {
    rest = rest.slice(1);
  }
  if (rest.length === 0) {
    return null;
  }
  const fileSeg = rest[rest.length - 1];
  const dirSegs = rest.slice(0, -1).map((d) => slugFolderSegmentToAzureTitle(d));
  return [...dirSegs, fileSeg].join('/');
}
