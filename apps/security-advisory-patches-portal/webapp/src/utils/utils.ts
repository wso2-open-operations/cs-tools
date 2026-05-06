// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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
 * Format bytes to human-readable size
 * @param bytes - Size in bytes
 * @returns Formatted string (e.g., "1.5 MB")
 */
export const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes';

  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
};

/**
 * Encode a path segment for shareable `/patches/...` URLs: spaces become "-", literal "-" become "--",
 * then encodeURIComponent (customer-portal uses opaque ids in routes instead of this file-name scheme).
 *
 * Example: "September-Special" → "September--Special"; "Aug Sep" → "Aug-Sep"
 */
export const toUrlFriendly = (segment: string): string => {
  const escapedHyphens = segment.replace(/-/g, '--');
  const spaced = escapedHyphens.replace(/\s+/g, '-');
  return encodeURIComponent(spaced);
};

/**
 * Private-use char — cannot appear in decoded Azure names; used only while decoding.
 * "--" splits into literal "-" joins; remaining "-" in each chunk were spaces.
 */
const DASH_LITERAL_HOLD = '\uE000';

/** Reverse {@link toUrlFriendly}: split on "--", turn single "-" into spaces, rejoin literal hyphens. */
function unescapeDashSegment(raw: string): string {
  return raw
    .split('--')
    .map((chunk) => chunk.replace(/-/g, ' '))
    .join(DASH_LITERAL_HOLD)
    .replace(new RegExp(DASH_LITERAL_HOLD, 'g'), '-');
}

/**
 * Decode a URL path segment to the Azure/storage name.
 * Legacy: if the segment still contains %20, only decodeURIComponent (old bookmarks).
 * Otherwise: decodeURIComponent then unescape dash encoding (see toUrlFriendly).
 */
export const fromUrlFriendly = (segment: string): string => {
  if (segment.includes('%20')) {
    return decodeURIComponent(segment);
  }
  const raw = decodeURIComponent(segment);
  if (!raw.includes('-')) {
    return raw;
  }
  return unescapeDashSegment(raw);
};

/**
 * Parse path segments from URL path
 * @param path - URL path (e.g., "patches/August-Special/file.pdf")
 * @returns Array of path segments with original names restored, excluding the 'patches' prefix
 */
export const parsePathSegments = (path: string): string[] => {
  if (!path) return [];
  const segments = path.split('/').filter((segment) => segment.length > 0);

  // Remove 'patches' prefix if it exists (for backward compatibility with old portal URLs)
  const filteredSegments = segments[0] === 'patches' ? segments.slice(1) : segments;

  return filteredSegments.map((segment) => fromUrlFriendly(segment));
};

/**
 * Build API path from path segments
 * @param segments - Array of path segments with original names
 * @returns Path string with trailing slash for API calls
 */
export const buildPath = (segments: string[]): string => {
  if (segments.length === 0) return '';
  return segments.join('/') + '/';
};

/**
 * Build URL path from path segments with /patches prefix for backward compatibility
 * @param segments - Array of path segments
 * @param fileName - Optional file name to append
 * @returns URL-friendly path string with /patches prefix
 */
export const buildUrlPath = (segments: string[], fileName?: string): string => {
  if (segments.length === 0 && !fileName) return '/patches';

  const pathSegments = segments.map((segment) => toUrlFriendly(segment));
  let urlPath = '/patches/' + pathSegments.join('/');

  if (fileName) {
    urlPath += '/' + toUrlFriendly(fileName);
  }

  return urlPath;
};
