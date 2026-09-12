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

import { sysidToUuid } from "@features/csm-cases/utils/inlineImages";

/**
 * A generic "the backing data source embedded one of its own record links in
 * this HTML" mechanism — the same idea `callRequestLinks.ts` pioneered for
 * call requests, generalized so a new record type (alert, smart alert, and
 * whatever comes after) is a config entry here rather than a third bespoke
 * regex-and-marker module. Alert references are the dominant real-world case:
 * they show up almost exclusively in incident work notes.
 *
 * Each entry matches a distinct backing-table URL pattern (bare text and,
 * where the source system is known to also emit it pre-anchored, an already
 * wrapped `<a href="...">`), extracts the 32-hex sysid, and converts it to a
 * uuid via the same {@link sysidToUuid} helper the inline-image and
 * call-request code already uses. The match is replaced with a `<span
 * data-sn-link-type="…" data-sn-link-id="…">` marker — a `<span>`, not an
 * `<a>`, so it can never be picked up by `CsmCaseCommentBubble`'s "open every
 * safe-href anchor in a new tab" pass and can never be re-linkified by
 * `linkifyBareUrls`, which must run after {@link replaceSnLinks}. Clicking it
 * opens an in-app modal instead of navigating away or reloading the app.
 */
export type SnLinkType = "alert" | "smartAlert";

interface SnLinkDefinition {
  type: SnLinkType;
  /** Accessible label for the clickable marker, e.g. "View alert". */
  label: string;
  /** Bare (non-anchor) occurrence of the record's URL as plain text. */
  bareUrlRegex: RegExp;
  /**
   * Already-wrapped `<a href="...">` occurrence of the record's URL, when the
   * source system is known to sometimes emit it that way. Omitted for a
   * pattern that only ever appears as a bare URL.
   */
  anchorRegex?: RegExp;
}

const HEX32 = "[a-f0-9]{32}";

function bareUrlPattern(table: string): RegExp {
  return new RegExp(
    `https?:\\/\\/[^\\s<>"']*${table}\\.do\\?sys_id=(${HEX32})[^\\s<>"']*`,
    "gi",
  );
}

function anchorPattern(table: string): RegExp {
  return new RegExp(
    `<a\\b[^>]*href\\s*=\\s*(?:"[^"]*${table}\\.do\\?sys_id=(${HEX32})[^"]*"|'[^']*${table}\\.do\\?sys_id=(${HEX32})[^']*')[^>]*>[\\s\\S]*?<\\/a>`,
    "gi",
  );
}

// The backing alert table. Appears both as a bare URL and, per the sampled
// work-note HTML, already wrapped in an `<a href="...">` — same dual shape as
// the call-request link.
const ALERT_TABLE = "u_custom_alert";

// The backing smart-alert table. Sampled work notes only ever carry this as a
// bare URL on its own line, never pre-anchored — but `bareUrlRegex` alone
// would still match the same URL text sitting inside an `<a href="...">` if
// one ever showed up, mangling the href with an inserted `<span>`. Carrying
// `anchorRegex` too, same as the `alert` entry, means that case is handled
// correctly instead of producing malformed HTML if it ever occurs.
const SMART_ALERT_TABLE = "u_smart_alert_buffer";

const SN_LINK_DEFINITIONS: SnLinkDefinition[] = [
  {
    type: "alert",
    label: "View alert",
    bareUrlRegex: bareUrlPattern(ALERT_TABLE),
    anchorRegex: anchorPattern(ALERT_TABLE),
  },
  {
    type: "smartAlert",
    label: "View smart alert",
    bareUrlRegex: bareUrlPattern(SMART_ALERT_TABLE),
    anchorRegex: anchorPattern(SMART_ALERT_TABLE),
  },
];

// The definitions' regexes carry the `g` flag (needed for `.replace()` to
// swap every occurrence, not just the first). `RegExp.prototype.test` on a
// global regex is stateful across calls (it advances `lastIndex`), which
// would make repeated `containsSnLink` calls on the same shared regex
// instance unreliable. Build a fresh non-global copy per check instead.
function toDetectionRegex(pattern: RegExp): RegExp {
  return new RegExp(pattern.source, pattern.flags.replace("g", ""));
}

/** True when `html` contains at least one recognized alert/smart-alert link (bare or anchored). */
export function containsSnLink(html: string): boolean {
  return SN_LINK_DEFINITIONS.some(
    (def) =>
      toDetectionRegex(def.bareUrlRegex).test(html) ||
      (def.anchorRegex ? toDetectionRegex(def.anchorRegex).test(html) : false),
  );
}

function markup(type: SnLinkType, label: string, uuid: string): string {
  return (
    `<span data-sn-link-type="${type}" data-sn-link-id="${uuid}" role="button" tabindex="0" ` +
    `style="color:inherit;text-decoration:underline;cursor:pointer;font-weight:600;">` +
    `${label}</span>`
  );
}

/**
 * Replaces every recognized alert/smart-alert URL reference in `html` — bare
 * text or already wrapped in an `<a href="...">` — with a clickable in-app
 * marker (`<span data-sn-link-type="…" data-sn-link-id="…">`).
 *
 * Must run before {@link import("./commentContent").linkifyBareUrls}, and
 * pairs with {@link import("./callRequestLinks").replaceCallRequestLinks} —
 * either order between the two is fine since they match disjoint URL
 * patterns, but both must run before the bare-URL linkifier.
 */
export function replaceSnLinks(html: string): string {
  if (!html || typeof html !== "string") return html;
  return SN_LINK_DEFINITIONS.reduce((acc, def) => {
    let next = acc;
    if (def.anchorRegex) {
      next = next.replace(def.anchorRegex, (_full, sysidDouble, sysidSingle) => {
        const sysid = sysidDouble ?? sysidSingle;
        return markup(def.type, def.label, sysidToUuid(sysid));
      });
    }
    next = next.replace(def.bareUrlRegex, (_full, sysid) =>
      markup(def.type, def.label, sysidToUuid(sysid)),
    );
    return next;
  }, html);
}
