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

export interface OutlineHeading {
  id: string;
  text: string;
  /** 2 or 3: the heading levels that make up the "On this page" outline. */
  level: 2 | 3;
}

export interface ArticleOutline {
  /** The body HTML with heading ids and code-block copy buttons added. */
  html: string;
  headings: OutlineHeading[];
}

/** Prefix for generated heading ids so they cannot collide with the page's own ids. */
const ID_PREFIX = "kb-";
const MAX_SLUG_LENGTH = 64;

/** Marker attribute of the copy button added to every code block. */
export const CODE_COPY_ATTR = "data-kb-copy";
export const CODE_BLOCK_CLASS = "kb-code";
export const CODE_COPY_CLASS = "kb-code-copy";

export function slugify(text: string): string {
  const slug = text
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, MAX_SLUG_LENGTH)
    .replace(/-+$/g, "");
  return slug || "section";
}

/**
 * Post-processes an already sanitised article body for display:
 * - every h2/h3 gets a stable id (existing ids are kept) and is collected
 *   into an outline for the table of contents;
 * - every `<pre>` is wrapped in a container with a "Copy" button, so code
 *   samples can be copied with one click.
 *
 * Call this on the output of sanitizeArticleHtml, never before it: the copy
 * button is a `<button>`, which the sanitiser would strip.
 */
export function buildArticleOutline(sanitizedHtml: string): ArticleOutline {
  const doc = new DOMParser().parseFromString(sanitizedHtml, "text/html");
  const used = new Set<string>();
  const headings: OutlineHeading[] = [];

  doc.body.querySelectorAll("h2, h3").forEach((heading) => {
    const text = (heading.textContent ?? "").replace(/\s+/g, " ").trim();
    if (!text) return;
    let id = heading.id;
    if (!id || used.has(id)) {
      const base = ID_PREFIX + slugify(text);
      id = base;
      for (let n = 2; used.has(id); n++) id = `${base}-${n}`;
      heading.id = id;
    }
    used.add(id);
    headings.push({ id, text, level: heading.tagName === "H2" ? 2 : 3 });
  });

  doc.body.querySelectorAll("pre").forEach((pre) => {
    if (pre.parentElement?.classList.contains(CODE_BLOCK_CLASS)) return;
    const wrapper = doc.createElement("div");
    wrapper.className = CODE_BLOCK_CLASS;
    const button = doc.createElement("button");
    button.type = "button";
    button.className = CODE_COPY_CLASS;
    button.setAttribute(CODE_COPY_ATTR, "");
    button.setAttribute("aria-label", "Copy code to clipboard");
    button.textContent = "Copy";
    pre.replaceWith(wrapper);
    wrapper.append(button, pre);
  });

  return { html: doc.body.innerHTML, headings };
}
