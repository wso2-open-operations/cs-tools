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
import {
  replaceMatchingAnchors,
  replaceTextNodes,
  splitTextByPattern,
  transformHtml,
  type HtmlDomTransform,
} from "@utils/renderTrustedHtml";

// The backing data source embeds call-request references as a plain-text URL
// to its own call-request page (`...customer_call.do?sys_id=<32-hex>`) —
// sometimes bare, sometimes already wrapped in an `<a href="...">` by the
// source system. Either form is matched by the same URL grammar; the
// surrounding `<a ...>...</a>` (if present) is replaced wholesale by our own
// marker so we never leave a dangling anchor around it. The anchor's inner
// (visible) content is carried over into the marker so whatever text the
// source system actually put there — usually the task number, which is real
// information the sentence around it depends on (see `buildMarker`) — is
// preserved.
const CALL_REQUEST_URL = /sn_customerservice_customer_call\.do\?sys_id=([a-f0-9]{32})/i;

// Matches a bare (non-anchor) occurrence of the call-request URL as plain
// text — the common case, since the source usually embeds these as raw URL
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
 * Builds the clickable in-app marker for a converted call-request id. Uses a
 * `<span>` (not `<a>`) deliberately — an anchor would need a real `href` and
 * would otherwise be picked up by `CsmCaseCommentBubble`'s generic
 * "make every safe-href anchor open in a new tab" pass, which is the wrong
 * behavior here (this should open the in-app popup, never navigate/new-tab).
 * `role="button"` + `tabindex="0"` mirror the existing inline-image click
 * affordance so it's keyboard operable without a dedicated a11y pass.
 *
 * `labelNodes` is empty for a bare URL (the {@link DEFAULT_LABEL} is used),
 * but the anchor case passes through the anchor's own children instead
 * (typically the call-request task number, e.g. `CTASK0012345`) — substituting
 * a generic label there discarded real information from the surrounding
 * sentence (backend text like "Case Task CTASK0012345 has been created"
 * rendered as "Case Task View call request has been created").
 */
function buildMarker(doc: Document, uuid: string, labelNodes: Node[] = []): HTMLSpanElement {
  const span = doc.createElement("span");
  span.setAttribute("data-call-request-sysid", uuid);
  span.setAttribute("role", "button");
  span.setAttribute("tabindex", "0");
  span.setAttribute(
    "style",
    "color:inherit;text-decoration:underline;cursor:pointer;font-weight:600;",
  );
  // An empty/whitespace-only anchor body (rare, but not impossible) has no
  // real text to preserve — fall back to the generic label rather than
  // rendering a blank clickable span.
  const hasLabel = labelNodes.some(
    (node) => node.nodeType === 1 || (node.textContent ?? "").trim() !== "",
  );
  if (hasLabel) span.append(...labelNodes);
  else span.textContent = DEFAULT_LABEL;
  return span;
}

/**
 * DOM transform: replaces every call-request URL reference — bare text or an
 * `<a href="...">` — with a clickable in-app marker
 * (`<span data-call-request-sysid="<uuid>">`) that `CsmCaseCommentBubble`
 * turns into an open-the-call-request-popup action, instead of linking out to
 * the backing system directly (which requires separate auth).
 *
 * Must run before `linkifyBareUrlsInDom`, which would otherwise turn the bare
 * URL form into a plain external `<a>` first.
 */
export const replaceCallRequestLinksInDom: HtmlDomTransform = (body) => {
  replaceMatchingAnchors(body, CALL_REQUEST_URL, (anchor, match) =>
    buildMarker(body.ownerDocument, sysidToUuid(match[1]), [...anchor.childNodes]),
  );
  replaceTextNodes(body, "a", (text, doc) =>
    splitTextByPattern(text, BARE_CALL_REQUEST_URL, doc, (match) =>
      buildMarker(doc, sysidToUuid(match[1])),
    ),
  );
};

/**
 * String form of {@link replaceCallRequestLinksInDom}. The result is NOT
 * sanitised: pass it through `renderTrustedHtml` before it reaches the DOM.
 */
export function replaceCallRequestLinks(html: string): string {
  return transformHtml(html, [replaceCallRequestLinksInDom]);
}
