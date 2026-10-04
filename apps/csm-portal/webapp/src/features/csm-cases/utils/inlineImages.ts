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
 * Extracts the backing attachment id from an inline `<img>` `src` value. The
 * backing data source embeds inline images as `.iix`-suffixed references
 * (e.g. `.../<attachmentId>.iix`); this pulls the id out regardless of
 * whether it appears as a full path or a bare token.
 */
export function extractInlineImageRefId(src: string): string {
  const s = src.trim();
  const fromPath = s.match(/\/([a-f0-9]{32})\.iix(?:\?|#|$)/i);
  if (fromPath) return fromPath[1];
  const tail =
    s
      .replace(/\.iix$/i, "")
      .split("/")
      .pop()
      ?.trim() ?? "";
  if (/^[a-f0-9]{32}$/i.test(tail)) return tail;
  return s
    .replace(/^\//, "")
    .replace(/\.iix$/i, "")
    .trim();
}

/**
 * Formats a 32-character backing-system sysid (no hyphens) as a canonical UUID
 * (`8-4-4-4-12`) — the shape the backend's `/attachments/{id}/content`
 * endpoint requires. Ids already in another shape are returned unchanged.
 */
export function sysidToUuid(id: string): string {
  if (!/^[a-f0-9]{32}$/i.test(id)) return id;
  return `${id.slice(0, 8)}-${id.slice(8, 12)}-${id.slice(12, 16)}-${id.slice(16, 20)}-${id.slice(20, 32)}`;
}

/**
 * Whether `src` is a raw base64-embedded image (`data:image/...;base64,...`)
 * rather than a real, separately-stored attachment. Content authored before
 * `SFTPGO_ATTACHMENT_STORAGE_ENABLED` was on (or created while it's off) never
 * gets extracted into a `.iix`-referenced attachment at all — the pixels stay
 * embedded directly in the comment/description HTML the backend already sends
 * to anyone holding `PermView`. The backend now redacts the real bytes out of
 * this before it ever reaches here (`apps/csm-portal/backend`'s own
 * `redactRawBase64Images` — see that repo's `CLAUDE.md`); this check just
 * decides whether the (now-inert, possibly-already-redacted) src still needs
 * hiding behind a placeholder for display consistency.
 */
export function isRawBase64ImageSrc(src: string): boolean {
  return /^data:image\//i.test(src.trim());
}

/**
 * A styled placeholder, in the same visual space the real image would have
 * occupied, for a `.iix` reference (or denied raw base64 image) that
 * couldn't be resolved into an image — explaining why in place of a
 * blank/broken `<img>`. Renders as its own block (`display:flex`, not
 * `inline-flex`) rather than sitting mid-line with surrounding text, and
 * styled as a warning alert (amber tint, warning icon) rather than a plain
 * dashed chip — reported live as reading poorly squeezed inline between two
 * sentences, unlike a real inline image, which naturally takes its own line.
 *
 * Built via DOM APIs (`textContent`, `setAttribute`) rather than an
 * HTML-string template: `reason`/its derived label are always one of two
 * fixed literals this module controls, never attacker-influenced, but
 * constructing the replacement as a real element — never parsed from a
 * string — means that stays true structurally, not just by the values
 * happening to be safe today. Plain inline styles (no CSS class) since this
 * is spliced into HTML with no app stylesheet guaranteed to apply to it;
 * the amber tones are semi-transparent (not a solid fill) so they read as a
 * warning tint over either a light or dark surrounding background without
 * hardcoding either, and `color:inherit` keeps the label itself the
 * surrounding text's own (already theme-correct) color.
 */
function createUnresolvedImagePlaceholder(
  doc: Document,
  reason: "permission" | "error",
): HTMLSpanElement {
  const label =
    reason === "permission"
      ? "You don't have permission to view this image"
      : "Image unavailable";
  const span = doc.createElement("span");
  span.setAttribute("data-unresolved-reason", reason);
  span.title = label;
  span.textContent = `⚠️ ${label}`;
  span.setAttribute(
    "style",
    "display:flex;align-items:center;gap:8px;width:fit-content;max-width:100%;" +
      "margin:6px 0;padding:6px 12px;box-sizing:border-box;" +
      "border:1px solid rgba(237,108,2,0.5);border-radius:6px;" +
      "background:rgba(237,108,2,0.12);" +
      "font-size:0.85em;color:inherit;",
  );
  return span;
}

/**
 * Parses `html` as a document fragment and returns every real `<img>`
 * element in it, via `DOMParser` rather than a regex over the raw markup.
 *
 * A regex-based "before-src attrs, src, after-src attrs" grammar (this
 * file's previous approach) cannot tell an attribute's real value apart from
 * attribute-boundary-crossing text elsewhere in the same tag — a crafted
 * `alt` containing the literal text `src="..."` could be matched as if it
 * were the image's actual `src`, leaving the real `src` (now shifted into
 * the regex's own "after" capture) completely unexamined and unredacted on
 * the client. `DOMParser` parses attributes the same way the browser that's
 * about to render this HTML does, so there is no equivalent confusion:
 * `img.getAttribute("src")` can only ever return the real `src` attribute's
 * value, never text from a different attribute or from the element's own
 * content.
 */
function parseImgElements(html: string): {
  doc: Document;
  imgs: HTMLImageElement[];
} {
  const doc = new DOMParser().parseFromString(html, "text/html");
  return { doc, imgs: Array.from(doc.images) };
}

/** Extracts every attachment id referenced by a `.iix` `<img>` src within an HTML string. */
export function extractIixAttachmentIds(html: string): string[] {
  const { imgs } = parseImgElements(html);
  const ids: string[] = [];
  for (const img of imgs) {
    const src = img.getAttribute("src") ?? "";
    if (src.includes(".iix")) {
      const id = extractInlineImageRefId(src);
      if (id && !ids.includes(id)) ids.push(id);
    }
  }
  return ids;
}

/**
 * Replaces every `.iix` `<img>` src in `html` with its resolved data URL from
 * `dataUrls`. A `.iix` reference in `deniedIds` is replaced with a
 * "no permission" placeholder; any other unresolved reference (not yet
 * loaded, unsupported type, a non-permission failure) gets a generic
 * "unavailable" placeholder — never left pointing at an auth-gated URL the
 * browser cannot fetch, and never silently blank.
 *
 * `denyRawBase64`, when true, additionally replaces every raw base64-embedded
 * image (see {@link isRawBase64ImageSrc}) with the same "no permission"
 * placeholder — pass `!canDownloadAttachment` here. The backend now redacts
 * the real image bytes out of the response before it ever reaches this
 * frontend for a caller who lacks that permission (`redactRawBase64Images` in
 * `apps/csm-portal/backend`, see that repo's own `CLAUDE.md`), so this is
 * display consistency on top of an already-closed confidentiality gap, not
 * the only thing standing between a denied caller and the real bytes.
 *
 * Parses `html` via `DOMParser` (see {@link parseImgElements}) rather than a
 * regex over the raw markup, so a crafted attribute (e.g. an `alt` containing
 * `src="..."`-shaped text) cannot be mistaken for the element's real `src`.
 */
export function replaceInlineImageSrcs(
  html: string,
  dataUrls: Map<string, string>,
  deniedIds?: Set<string>,
  denyRawBase64?: boolean,
): string {
  const { doc, imgs } = parseImgElements(html);
  for (const img of imgs) {
    const src = img.getAttribute("src") ?? "";
    if (src.includes(".iix")) {
      const refId = extractInlineImageRefId(src);
      const dataUrl = dataUrls.get(refId);
      if (dataUrl) {
        img.setAttribute("src", dataUrl);
      } else {
        img.replaceWith(
          createUnresolvedImagePlaceholder(
            doc,
            deniedIds?.has(refId) ? "permission" : "error",
          ),
        );
      }
      continue;
    }
    if (denyRawBase64 && isRawBase64ImageSrc(src)) {
      img.replaceWith(createUnresolvedImagePlaceholder(doc, "permission"));
    }
  }
  return doc.body.innerHTML;
}
