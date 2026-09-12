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

// The backing data source embeds call-request references as a plain-text URL
// to its own `sn_customerservice_customer_call.do?sys_id=<32-hex>` page —
// sometimes bare, sometimes already wrapped in an `<a href="...">` by the
// source system. Either form is matched by the same URL grammar; the
// surrounding `<a ...>...</a>` (if present) is replaced wholesale by our own
// marker so we never leave a dangling anchor around it.
const CALL_REQUEST_URL = /sn_customerservice_customer_call\.do\?sys_id=([a-f0-9]{32})/i;

// Matches an already-wrapped anchor whose href contains the call-request URL,
// e.g. `<a href="https://host/sn_customerservice_customer_call.do?sys_id=...">
// CTASK0012345</a>`. Captures both the sysid and the anchor's inner (visible)
// content so the whole anchor can be swapped for our marker while preserving
// whatever text the source system actually put there — usually the task
// number, which is real information the sentence around it depends on (see
// `callRequestMarkup`'s doc comment for why we keep it rather than replacing
// it with a generic label).
const CALL_REQUEST_ANCHOR = new RegExp(
  `<a\\b[^>]*href\\s*=\\s*(?:"[^"]*sn_customerservice_customer_call\\.do\\?sys_id=([a-f0-9]{32})[^"]*"|'[^']*sn_customerservice_customer_call\\.do\\?sys_id=([a-f0-9]{32})[^']*')[^>]*>([\\s\\S]*?)<\\/a>`,
  "gi",
);

// Matches a bare (non-anchor) occurrence of the call-request URL as plain
// text — the common case, since ServiceNow usually embeds these as raw URL
// text rather than a real link. Consumes the full URL run (scheme + host +
// path + query) so nothing but our marker is left behind.
const BARE_CALL_REQUEST_URL = /https?:\/\/[^\s<>"']*sn_customerservice_customer_call\.do\?sys_id=([a-f0-9]{32})[^\s<>"']*/gi;

/** True when `html` contains at least one call-request URL reference (bare or anchored). */
export function containsCallRequestLink(html: string): boolean {
  return CALL_REQUEST_URL.test(html);
}

// Fallback label when there's no original visible text to reuse — the bare
// (non-anchor) URL case, which never carried a label in the first place.
const DEFAULT_LABEL = "View call request";

/**
 * Renders the clickable in-app marker for a converted call-request id. Uses a
 * `<span>` (not `<a>`) deliberately — an anchor would need a real `href` and
 * would otherwise be picked up by `CsmCaseCommentBubble`'s generic
 * "make every safe-href anchor open in a new tab" pass, which is the wrong
 * behavior here (this should open the in-app popup, never navigate/new-tab).
 * `role="button"` + `tabindex="0"` mirror the existing inline-image click
 * affordance so it's keyboard operable without a dedicated a11y pass.
 *
 * `label` defaults to {@link DEFAULT_LABEL} for a bare URL, but the anchor
 * case passes through the anchor's own inner content instead (typically the
 * call-request task number, e.g. `CTASK0012345`) — substituting a generic
 * label there discarded real information from the surrounding sentence
 * (backend text like "Case Task CTASK0012345 has been created" rendered as
 * "Case Task View call request has been created").
 */
function callRequestMarkup(uuid: string, label: string = DEFAULT_LABEL): string {
  return (
    `<span data-call-request-sysid="${uuid}" role="button" tabindex="0" ` +
    `style="color:inherit;text-decoration:underline;cursor:pointer;font-weight:600;">` +
    `${label}</span>`
  );
}

/**
 * Replaces every ServiceNow call-request URL reference in `html` — bare text
 * or already wrapped in an `<a href="...">` — with a clickable in-app marker
 * (`<span data-call-request-sysid="<uuid>">`) that `CsmCaseCommentBubble`
 * turns into an open-the-call-request-popup action, instead of linking out to
 * ServiceNow directly (which requires separate SN auth).
 *
 * Must run before {@link import("./commentContent").linkifyBareUrls}, which
 * would otherwise turn the bare URL form into a plain external `<a>` first.
 */
export function replaceCallRequestLinks(html: string): string {
  if (!html || typeof html !== "string") return html;
  return html
    .replace(
      CALL_REQUEST_ANCHOR,
      (_full, sysidDouble, sysidSingle, innerContent: string) => {
        const sysid = sysidDouble ?? sysidSingle;
        // An empty/whitespace-only anchor body (rare, but not impossible)
        // has no real text to preserve — fall back to the generic label
        // rather than rendering a blank clickable span.
        const label = innerContent.trim() || DEFAULT_LABEL;
        return callRequestMarkup(sysidToUuid(sysid), label);
      },
    )
    .replace(BARE_CALL_REQUEST_URL, (_full, sysid) =>
      callRequestMarkup(sysidToUuid(sysid)),
    );
}
