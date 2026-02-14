// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  RICH_TEXT_BLOCK_TAGS,
  type RichTextBlockVariant,
} from "@constants/supportConstants";

/**
 * Returns display label and typography variant for a block tag.
 * @param tag - Block tag name (p, h1–h6).
 * @returns Label and variant.
 */
export function getBlockDisplay(tag: string): {
  label: string;
  variant: RichTextBlockVariant;
} {
  const normalizedTag = tag.toLowerCase();
  const found = RICH_TEXT_BLOCK_TAGS.find((t) => t.value === normalizedTag);
  return found ?? { label: "Body 2", variant: "body2" };
}

/**
 * Converts HTML from the rich editor to Markdown (for Edit as Markdown section).
 * @param html - HTML string from the editor.
 * @returns Markdown string.
 */
export function htmlToMarkdown(html: string): string {
  const parser = new DOMParser();
  const doc = parser.parseFromString(html, "text/html");
  const body = doc.body;
  let out = "";
  const walk = (node: Node): string => {
    if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? "";
    const el = node as HTMLElement;
    const tag = el.tagName?.toLowerCase();
    const children = Array.from(node.childNodes).map(walk).join("");
    switch (tag) {
      case "h1":
        return `# ${children.trim()}\n\n`;
      case "h2":
        return `## ${children.trim()}\n\n`;
      case "h3":
        return `### ${children.trim()}\n\n`;
      case "h4":
        return `#### ${children.trim()}\n\n`;
      case "h5":
        return `##### ${children.trim()}\n\n`;
      case "h6":
        return `###### ${children.trim()}\n\n`;
      case "b":
      case "strong":
        return `**${children}**`;
      case "i":
      case "em":
        return `_${children}_`;
      case "u":
        return children;
      case "a":
        return `[${children}](${el.getAttribute("href") ?? ""})`;
      case "br":
        return "\n";
      case "hr":
        return "\n\n---\n\n";
      case "p":
        return children.trim() ? `${children.trim()}\n\n` : "\n\n";
      case "pre":
        return `\n\`\`\`\n${children}\n\`\`\`\n\n`;
      case "code":
        return el.parentElement?.tagName?.toLowerCase() === "pre"
          ? children
          : `\`${children}\``;
      case "ul":
        return (
          Array.from(el.children)
            .map((li) => `- ${walk(li).trim()}`)
            .join("\n") + "\n\n"
        );
      case "ol":
        return (
          Array.from(el.children)
            .map((li, i) => `${i + 1}. ${walk(li).trim()}`)
            .join("\n") + "\n\n"
        );
      case "li":
        return Array.from(el.childNodes).map(walk).join("").trim();
      case "img":
        return `![image](${el.getAttribute("src") ?? ""})`;
      case "div":
        return children.trim() ? `${children}\n\n` : children;
      case "script":
      case "style":
      case "iframe":
      case "object":
      case "embed":
        return "";
      default:
        return Array.from(node.childNodes).map(walk).join("");
    }
  };
  Array.from(body.childNodes).forEach((n) => {
    out += walk(n);
  });
  return out.replace(/\n{3,}/g, "\n\n").trim() || "";
}

/**
 * Converts Markdown (from Edit as Markdown section) back to HTML for the rich editor.
 * @param md - Markdown string.
 * @returns HTML string.
 */
export function markdownToHtml(md: string): string {
  // 1. Extract and protect fenced code blocks before general escaping
  // Use {{}} placeholders so the emphasis regex (^|\W)_(.+?)_(?=\W|$) does not match them
  const fencedCodeBlocks: string[] = [];
  let html = md.replace(/```(\w*)\n?([\s\S]*?)```/g, (_m, _lang, code) => {
    const id = `{{BT_FENCED_${fencedCodeBlocks.length}}}`;
    fencedCodeBlocks.push(`<pre><code>${escapeHtml(code)}</code></pre>`);
    return id;
  });

  // 2. Escape everything else for XSS protection
  html = escapeHtml(html);

  // 3. Protect inline code spans from formatting rules (now safely escaped)
  const inlinePlaceholders: string[] = [];
  html = html.replace(/`([^`]+)`/g, (_m, code) => {
    const id = `{{BT_INLINE_${inlinePlaceholders.length}}}`;
    inlinePlaceholders.push(`<code>${code}</code>`);
    return id;
  });

  html = html.replace(/^(-{3,}|\*{3,}|_{3,})\s*$/gm, "<hr/>");

  html = html.replace(/^######\s+(.+)$/gm, "<h6>$1</h6>");
  html = html.replace(/^#####\s+(.+)$/gm, "<h5>$1</h5>");
  html = html.replace(/^####\s+(.+)$/gm, "<h4>$1</h4>");
  html = html.replace(/^###\s+(.+)$/gm, "<h3>$1</h3>");
  html = html.replace(/^##\s+(.+)$/gm, "<h2>$1</h2>");
  html = html.replace(/^#\s+(.+)$/gm, "<h1>$1</h1>");
  html = html.replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>");
  html = html.replace(/\*(.+?)\*/g, "<em>$1</em>");
  html = html.replace(/(^|\W)_(.+?)_(?=\W|$)/g, "$1<em>$2</em>");
  html = html.replace(
    /\[([^\]]+)\]\(([^)]+)\)/g,
    (_m, text, url) =>
      `<a href="${sanitizeUrl(url)}" target="_blank" rel="noopener">${text}</a>`,
  );

  // 4. Restore code spans
  inlinePlaceholders.forEach((content, i) => {
    html = html.replace(`{{BT_INLINE_${i}}}`, content);
  });

  fencedCodeBlocks.forEach((content, i) => {
    html = html.replace(`{{BT_FENCED_${i}}}`, content);
  });

  // Protect code blocks before splitting by blank lines
  const placeholders: string[] = [];
  html = html.replace(/<pre>[\s\S]*?<\/pre>/g, (match) => {
    const id = `{{BT_CODE_BLOCK_${placeholders.length}}}`;
    placeholders.push(match);
    return id;
  });

  // Handle unordered lists
  html = html.replace(/^[\t ]*[*+-] (.+)(\n[\t ]*[*+-] (.+))*/gm, (match) => {
    const items = match
      .split("\n")
      .map((line) => `<li>${line.replace(/^[\t ]*[*+-] /, "").trim()}</li>`)
      .join("");
    return `<ul>${items}</ul>`;
  });

  // Handle ordered lists
  html = html.replace(/^[\t ]*\d+\. (.+)(\n[\t ]*\d+\. (.+))*/gm, (match) => {
    const items = match
      .split("\n")
      .map((line) => `<li>${line.replace(/^[\t ]*\d+\. /, "").trim()}</li>`)
      .join("");
    return `<ol>${items}</ol>`;
  });

  const blocks = html.split(/(\n\s*\n|<hr\/>)/).filter((b) => b.trim() !== "");
  let result = blocks
    .map((block) => {
      const t = block.trim();
      if (t === "") return "";
      if (t === "<hr/>") return "<hr/>";
      // Don't wrap code block placeholders in <p> labels
      if (
        t.startsWith("<h") ||
        t.startsWith("<ul") ||
        t.startsWith("<ol") ||
        t.startsWith("{{BT_CODE_BLOCK_")
      ) {
        return t;
      }
      return `<p>${t.replace(/\n/g, "<br/>")}</p>`;
    })
    .join("");

  // Restore code blocks
  placeholders.forEach((content, i) => {
    result = result.replace(`{{BT_CODE_BLOCK_${i}}}`, content);
  });

  return result || "";
}

/**
 * Escapes HTML entities in a string to prevent XSS and display code correctly.
 * @param text - The text to escape.
 * @returns Escaped HTML string.
 */
const SAFE_URL_PATTERN = /^(https?:\/\/|mailto:|tel:|\/|#)/i;

/**
 * Sanitizes a URL by allowing only safe protocols.
 * @param url - The URL to sanitize.
 * @returns Sanitized URL or an empty string if unsafe.
 */
function sanitizeUrl(url: string): string {
  const decoded = url.replace(/&amp;/g, "&"); // un-escape for check
  return SAFE_URL_PATTERN.test(decoded.trim()) ? url : "";
}

export function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

/**
 * Creates HTML for a code block with proper escaping.
 * @param code - The code content to insert.
 * @returns HTML string for the code block.
 */
export function createCodeBlockHtml(code: string): string {
  const escaped = escapeHtml(code);
  return `<pre><code>${escaped}</code></pre><p><br></p>`;
}

/**
 * Attempts to insert HTML at the current cursor position.
 * Falls back to appending if execCommand fails.
 * @param editorElement - The contentEditable element.
 * @param html - The HTML string to insert.
 * @param logger - Optional logger for error reporting.
 * @returns True if insertion was successful.
 */
export function insertHtmlAtCursor(
  editorElement: HTMLElement,
  html: string,
  rangeToRestore?: Range | null,
  logger?: { warn: (message: string, ...args: unknown[]) => void },
): boolean {
  editorElement.focus();

  const sel = window.getSelection();
  if (!sel) return false;

  let range: Range;
  if (
    rangeToRestore &&
    editorElement.contains(rangeToRestore.commonAncestorContainer)
  ) {
    try {
      sel.removeAllRanges();
      sel.addRange(rangeToRestore);
      range = rangeToRestore;
    } catch (error) {
      logger?.warn("Failed to restore saved range:", error);
      range = sel.rangeCount > 0 ? sel.getRangeAt(0) : document.createRange();
    }
  } else if (
    sel.rangeCount > 0 &&
    editorElement.contains(sel.getRangeAt(0).commonAncestorContainer)
  ) {
    range = sel.getRangeAt(0);
  } else {
    range = document.createRange();
    range.selectNodeContents(editorElement);
    range.collapse(false);
    sel.removeAllRanges();
    sel.addRange(range);
  }

  let htmlToInsert = html;
  const isAtEnd =
    range.startContainer === editorElement ||
    (range.startContainer.nodeType === Node.TEXT_NODE &&
      range.startContainer.parentElement === editorElement &&
      range.startOffset === range.startContainer.textContent?.length);

  if (isAtEnd) {
    const lastChild = editorElement.lastElementChild;
    const isLastBlock =
      lastChild && /^(P|H[1-6]|PRE|DIV|UL|OL|LI)$/.test(lastChild.tagName);

    if (editorElement.innerHTML.trim() !== "" && !isLastBlock) {
      htmlToInsert = `<br>${html}`;
    }
  }

  try {
    range.deleteContents();
    const fragment = range.createContextualFragment(htmlToInsert);
    const lastNode = fragment.lastChild;
    range.insertNode(fragment);

    if (lastNode) {
      const newRange = document.createRange();
      newRange.setStartAfter(lastNode);
      newRange.collapse(true);
      sel.removeAllRanges();
      sel.addRange(newRange);
    }
    return true;
  } catch (error) {
    logger?.warn("Failed to insert HTML with Range API:", error);
    editorElement.innerHTML += htmlToInsert;
    return true;
  }
}

/**
 * Toggles an inline format (bold, italic, underline) using modern Range API.
 */
export function toggleInlineFormat(tag: string): void {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0 || sel.isCollapsed) return;

  const range = sel.getRangeAt(0);
  const tagUpper = tag.toUpperCase();

  let parent = range.commonAncestorContainer as Node | null;
  while (parent && parent !== document.body) {
    if (parent.nodeName === tagUpper) {
      const parentEl = parent as HTMLElement;
      const fragment = document.createDocumentFragment();
      while (parentEl.firstChild) {
        fragment.appendChild(parentEl.firstChild);
      }
      parentEl.parentNode?.replaceChild(fragment, parentEl);
      return;
    }
    parent = parent.parentNode;
  }

  const el = document.createElement(tag);
  try {
    el.appendChild(range.extractContents());
    range.insertNode(el);
    sel.selectAllChildren(el);
  } catch (e) {
    // Selection might be complex or invalid
  }
}

/**
 * Sets the block format (h1, h2, etc.) for the current selection.
 */
export function setBlockFormat(tag: string): void {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0) return;

  const range = sel.getRangeAt(0);
  let node = range.startContainer;

  // Find the block parent
  while (
    node &&
    node.parentNode &&
    !/^(P|H[1-6]|DIV|LI)$/.test(node.nodeName)
  ) {
    node = node.parentNode;
  }

  if (node && /^(P|H[1-6]|DIV|LI)$/.test(node.nodeName)) {
    const newEl = document.createElement(tag);
    newEl.innerHTML = (node as HTMLElement).innerHTML;
    node.parentNode?.replaceChild(newEl, node);
  }
}

/**
 * Sets text alignment for the current block.
 */
export function setTextAlignment(alignment: string): void {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0) return;

  const range = sel.getRangeAt(0);
  let node = range.startContainer;

  while (
    node &&
    node.parentNode &&
    !/^(P|H[1-6]|DIV|LI)$/.test(node.nodeName)
  ) {
    node = node.parentNode;
  }

  if (node && node.nodeType === Node.ELEMENT_NODE) {
    (node as HTMLElement).style.textAlign = alignment;
  }
}

/**
 * Handles indentation.
 */
export function setIndentation(type: "indent" | "outdent"): void {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0) return;

  const range = sel.getRangeAt(0);
  let node = range.startContainer;

  while (
    node &&
    node.parentNode &&
    !/^(P|H[1-6]|DIV|LI)$/.test(node.nodeName)
  ) {
    node = node.parentNode;
  }

  if (node && node.nodeType === Node.ELEMENT_NODE) {
    const el = node as HTMLElement;
    const currentPadding = parseInt(el.style.paddingLeft || "0", 10);
    const newPadding =
      type === "indent"
        ? currentPadding + 40
        : Math.max(0, currentPadding - 40);
    el.style.paddingLeft = newPadding ? `${newPadding}px` : "";
  }
}

/**
 * Toggles lists using modern Range API.
 */
export function toggleList(type: "ul" | "ol"): void {
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0) return;

  const range = sel.getRangeAt(0);
  let node = range.startContainer;

  // Find the block parent
  while (
    node &&
    node.parentNode &&
    !/^(P|H[1-6]|DIV|LI)$/.test(node.nodeName)
  ) {
    node = node.parentNode;
  }

  if (node && node.nodeType === Node.ELEMENT_NODE) {
    const el = node as HTMLElement;
    if (el.tagName === "LI") {
      const parentList = el.parentNode as HTMLElement;
      if (!parentList) return;

      const outerParent = parentList.parentNode;
      if (!outerParent) return;

      // Create the new paragraph for the current LI
      const p = document.createElement("p");
      p.innerHTML = el.innerHTML;

      // Extract all siblings after the current LI
      const nextSiblings: Node[] = [];
      let sibling = el.nextSibling;
      while (sibling) {
        nextSiblings.push(sibling);
        sibling = sibling.nextSibling;
      }

      // If there are next siblings, we move them to a new list of the same type
      if (nextSiblings.length > 0) {
        const newList = document.createElement(parentList.tagName);
        nextSiblings.forEach((s) => {
          newList.appendChild(s);
        });
        outerParent.insertBefore(newList, parentList.nextSibling);
      }

      // Insert the paragraph after the original (possibly truncated) list
      outerParent.insertBefore(p, parentList.nextSibling);

      // Remove the targeted LI
      el.remove();

      // If the original list is now empty, remove it
      if (parentList.childNodes.length === 0) {
        parentList.remove();
      }
    } else {
      // Wrap current block in a list
      const list = document.createElement(type);
      const li = document.createElement("li");
      const clone = el.cloneNode(true) as HTMLElement;

      // If the block is a paragraph, we take its contents to avoid <p> inside <li>
      if (clone.tagName === "P") {
        li.innerHTML = clone.innerHTML;
      } else {
        li.appendChild(clone);
      }

      list.appendChild(li);
      el.parentNode?.replaceChild(list, el);
    }
  }
}
/**
 * Detects active formatting at the current selection by traversing parents.
 */
export function queryActiveFormats(): {
  bold: boolean;
  italic: boolean;
  underline: boolean;
  block: string;
} {
  const sel = window.getSelection();
  const formats = { bold: false, italic: false, underline: false, block: "p" };

  if (!sel || sel.rangeCount === 0) return formats;

  let node: Node | null = sel.anchorNode;
  while (node && node.nodeName !== "BODY" && node.nodeName !== "HTML") {
    const tag = node.nodeName.toLowerCase();
    if (tag === "strong" || tag === "b") formats.bold = true;
    if (tag === "em" || tag === "i") formats.italic = true;
    if (tag === "u") formats.underline = true;
    if (/^(p|h[1-6]|pre)$/i.test(tag)) {
      formats.block = tag;
    }
    node = node.parentNode;
  }
  return formats;
}
