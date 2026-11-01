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

import DOMPurify from "dompurify";

/**
 * Article bodies are authored rich text served by a public, unauthenticated
 * API and rendered with dangerouslySetInnerHTML, so the policy is stricter
 * than DOMPurify's defaults: no inline styles (CSS can overlay or exfiltrate),
 * and nothing interactive (forms, inputs, frames, embedded objects) that could
 * stage a phishing form or load third-party content inside the page.
 */
const SANITIZE_CONFIG = {
  FORBID_TAGS: [
    "style",
    "form",
    "input",
    "button",
    "textarea",
    "select",
    "iframe",
    "object",
    "embed",
  ],
  FORBID_ATTR: ["style"],
};

/** True when `href` points at a different origin than the page. */
export function isExternalHref(href: string | null, pageOrigin: string): boolean {
  if (!href) return false;
  // Protocol-relative (//host/path) and absolute http(s) URLs only. Relative
  // paths, fragments, mailto: and tel: stay in the same tab.
  if (!/^(https?:)?\/\//i.test(href.trim())) return false;
  try {
    return new URL(href.trim(), pageOrigin).origin !== pageOrigin;
  } catch {
    return false;
  }
}

// External links open in a new tab and must not hand the new page a reference
// to this one (reverse tabnabbing). DOMPurify strips `target` itself, so it is
// set here, after sanitization, once the href is known to have survived.
// Registered once at module load.
if (typeof window !== "undefined") {
  DOMPurify.addHook("afterSanitizeAttributes", (node) => {
    if (node.tagName === "A" && isExternalHref(node.getAttribute("href"), window.location.origin)) {
      node.setAttribute("target", "_blank");
      node.setAttribute("rel", "noopener noreferrer");
    }
  });
}

/**
 * The page title is the only h1. When an article body brings its own h1, every
 * body heading moves down one level (h1 to h2, h2 to h3, ... capped at h6) so
 * the outline stays nested. Bodies without an h1 are left alone.
 */
function demoteHeadings(root: ParentNode): void {
  if (!root.querySelector("h1")) return;
  const headings = Array.from(root.querySelectorAll("h1, h2, h3, h4, h5, h6")).reverse();
  for (const heading of headings) {
    const level = Math.min(Number(heading.tagName[1]) + 1, 6);
    if (`h${level}` === heading.tagName.toLowerCase()) continue;
    const replacement = heading.ownerDocument.createElement(`h${level}`);
    for (const attr of Array.from(heading.attributes)) {
      replacement.setAttribute(attr.name, attr.value);
    }
    replacement.append(...Array.from(heading.childNodes));
    heading.replaceWith(replacement);
  }
}

/** Sanitize an article body for rendering via dangerouslySetInnerHTML. */
export function sanitizeArticleHtml(html: string): string {
  const fragment = DOMPurify.sanitize(html, { ...SANITIZE_CONFIG, RETURN_DOM_FRAGMENT: true });
  demoteHeadings(fragment);
  const container = document.createElement("div");
  container.appendChild(fragment);
  return container.innerHTML;
}
