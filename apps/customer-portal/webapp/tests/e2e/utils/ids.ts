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

//
// Record ids appear in two interchangeable spellings, and a URL assertion has to
// accept either.
//
// ServiceNow sys_ids — which is what projects, cases and deployments are keyed
// by — are 32 hex characters with no separators, and that is the form the
// fixtures in config/testData.ts store and the API returns. The portal, however,
// canonicalises the id in the address bar to the UUID-hyphenated spelling of the
// same 32 characters: navigating to
//
//   /projects/641058e63b5a87103e1e088aa4e45a13/dashboard
//
// lands on
//
//   /projects/641058e6-3b5a-8710-3e1e-088aa4e45a13/dashboard
//
// (verified live). Both address the same record and both are accepted on the way
// in, so this is a display convention rather than a redirect to somewhere else —
// but a `toHaveURL` built by interpolating a fixture id fails against it, and
// fails in a way that reads like a routing bug rather than a formatting one.
//

/**
 * Matches ANY record id, in either spelling, for a URL whose id is not known
 * ahead of time.
 *
 * {@link idPattern} is for an id you already hold; this is for asserting the
 * SHAPE of a route — "…/engagements/<some record>" — where the id only exists
 * after the navigation being asserted.
 *
 * Both spellings are accepted for the reason described above: the portal renders
 * ids UUID-hyphenated while the API returns them as plain 32-hex, and a pattern
 * fixed to one of those quietly stops matching if the other is rendered. It is a
 * regex fragment rather than a RegExp so it can be interpolated into a larger
 * path pattern.
 */
export const RECORD_ID_PATTERN =
  "(?:[0-9a-f]{32}|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})";

/**
 * Escapes a string for literal use inside a regular expression.
 *
 * @param value - Raw string.
 * @returns The string with regex metacharacters escaped.
 */
function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Builds a regex fragment matching an id in either spelling.
 *
 * Ids that are not 32-hex are returned escaped and unchanged — service request
 * numbers and the like have no second form, and silently widening them would
 * make an assertion looser than it looks.
 *
 * @param id - Record id, in either spelling.
 * @returns A non-capturing regex fragment, for interpolation into a URL pattern.
 */
export function idPattern(id: string): string {
  const plain = id.replace(/-/g, "");
  if (!/^[0-9a-f]{32}$/i.test(plain)) return escapeRegExp(id);

  const uuid = [
    plain.slice(0, 8),
    plain.slice(8, 12),
    plain.slice(12, 16),
    plain.slice(16, 20),
    plain.slice(20),
  ].join("-");

  return `(?:${plain}|${uuid})`;
}

/**
 * Drops a terminal `$` from a suffix, when it is a regex anchor rather than a
 * literal.
 *
 * Several callers anchor their own suffix — `"support$"` — which predates this
 * helper enforcing the path end itself. Left in place, that `$` demands the
 * string end THERE, so `/projects/<id>/support?tab=cases` stops matching while
 * the unanchored `"support"` accepts it: the same intent, two different
 * behaviours depending on how the caller happened to write it. Removing the
 * anchor lets the boundary assertion below do the job uniformly.
 *
 * A `$` preceded by an odd number of backslashes is escaped — a literal dollar
 * in the path — and is kept.
 *
 * @param suffix - Path fragment, possibly self-anchored.
 * @returns The suffix without its trailing anchor.
 */
function stripTerminalAnchor(suffix: string): string {
  if (!suffix.endsWith("$")) return suffix;

  let backslashes = 0;
  for (let i = suffix.length - 2; i >= 0 && suffix[i] === "\\"; i -= 1) {
    backslashes += 1;
  }

  return backslashes % 2 === 0 ? suffix.slice(0, -1) : suffix;
}

/**
 * Builds a pattern for a project-scoped path, tolerant of both id spellings.
 *
 * The path must END after `suffix` — a query string or fragment may follow, but
 * no further path segments. Without that, the pattern is a prefix match:
 * `projectPathPattern(id, "settings")` would also accept
 * `/projects/<id>/settings/users/42`, so an assertion that a nav landed on
 * Settings would pass on a deeper route it never meant to allow.
 *
 * Enforced with a lookahead rather than `$` so it composes with suffixes that
 * match a query of their own. A suffix that anchors itself with a trailing `$`
 * has that anchor removed first — see {@link stripTerminalAnchor} — so anchored
 * and unanchored suffixes behave identically.
 *
 * @param projectId - Project id, in either spelling.
 * @param suffix - Path after the project id, e.g. `dashboard` or
 *   `support/cases/abc`. Interpolate ids in it via {@link idPattern}.
 * @returns A RegExp matching that path under either spelling.
 */
export function projectPathPattern(
  projectId: string,
  suffix: string,
): RegExp {
  // (?![^?#]) — the next character, if any, must be `?` or `#`. At the end of
  // the string the lookahead trivially succeeds. A caller's own trailing anchor
  // is dropped first, or it would forbid the query string this is meant to
  // allow.
  const path = stripTerminalAnchor(suffix);
  return new RegExp(`/projects/${idPattern(projectId)}/${path}(?![^?#])`);
}
