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

import { type ReactElement, type RefObject } from "react";
import { type Theme } from "@wso2/oxygen-ui";
import {
  File as FileIcon,
  FileArchive,
  FileCode,
  FileImage,
  FileText,
} from "@wso2/oxygen-ui-icons-react";
import { createCommand, type LexicalCommand } from "lexical";
import type { InsertImagePayload } from "@features/support/types/supportRichText";

export type { InsertImagePayload } from "@features/support/types/supportRichText";

/**
 * Derives alt text from a URL or filename (e.g. "image.png" -> "image", "/path/to/photo.jpg" -> "photo").
 * Handles absolute URLs, relative paths, and plain filenames.
 */
export function deriveAltFromFilename(src: string): string {
  try {
    const url = new URL(src);
    const path = url.pathname || "";
    const match = path.match(/\/([^/]+)$/);
    const name = (match?.[1] ?? path) || "Image";
    const base = name.replace(/\.[^.]+$/, "");
    return base ? base : "Image";
  } catch {
    const pathOnly = src.split("?")[0].split("#")[0].trim();
    const lastSegment = pathOnly.replace(/\/+$/, "").split("/").pop() || "";
    const base = lastSegment.replace(/\.[^.]+$/, "");
    return base ? base : "Image";
  }
}

/**
 * Converts HTML to plain text by stripping tags and decoding entities.
 */
export function htmlToPlainText(html: string): string {
  if (!html || typeof html !== "string") return "";
  const parsed = new DOMParser().parseFromString(html, "text/html");
  const body = parsed.body ?? parsed.documentElement;
  return (body?.textContent ?? "").trim();
}

/**
 * Escapes HTML entities in a string.
 */
export function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

/**
 * Sanitizes a URL by allowing only safe protocols.
 * Rejects protocol-relative URLs (e.g. //evil.com); allows single leading slash for relative paths.
 * Also allows data:image/* base64 URLs for inline images in the rich text editor.
 */
const SAFE_URL_PATTERN =
  /^(https?:\/\/|mailto:|tel:|\/(?!\/)|#|data:image\/[a-zA-Z0-9.+-]+;base64,)/i;
export function sanitizeUrl(url: string): string {
  const decoded = url.replace(/&amp;/g, "&").trim();
  return SAFE_URL_PATTERN.test(decoded) ? decoded : "";
}

/**
 * Returns the appropriate icon for a file based on its extension or type.
 */
export const getFileIcon = (file: File, theme: Theme): ReactElement => {
  const name = file.name.toLowerCase();
  const type = file.type.toLowerCase();

  if (
    /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i.test(name) ||
    type.startsWith("image/")
  ) {
    return <FileImage size={16} color={theme.palette.primary.main} />;
  }
  if (/\.pdf$/i.test(name) || type.includes("pdf")) {
    return <FileText size={16} color={theme.palette.primary.main} />;
  }
  if (
    /\.(zip|rar|7z|tar|gz)$/i.test(name) ||
    type.includes("zip") ||
    type.includes("archive")
  ) {
    return <FileArchive size={16} color={theme.palette.primary.main} />;
  }
  if (
    /\.(js|ts|tsx|jsx|py|java|cpp|c|h|cs|go|rs|php|rb|html|css|json|md|xml|yaml|yml|sh|sql)$/i.test(
      name,
    ) ||
    type.includes("code")
  ) {
    return <FileCode size={16} color={theme.palette.primary.main} />;
  }
  if (/\.(txt|log|csv)$/i.test(name) || type.startsWith("text/")) {
    return <FileText size={16} color={theme.palette.primary.main} />;
  }
  return <FileIcon size={16} color={theme.palette.primary.main} />;
};

/**
 * Scrolls an element by a given amount. Accepts either element id or ref.
 */
export const scrollElement = (
  elementIdOrRef: string | RefObject<HTMLElement | null>,
  direction: "left" | "right",
  scrollAmount: number = 200,
) => {
  const container =
    typeof elementIdOrRef === "string"
      ? document.getElementById(elementIdOrRef)
      : elementIdOrRef.current;
  if (container) {
    container.scrollBy({
      left: direction === "left" ? -scrollAmount : scrollAmount,
      behavior: "smooth",
    });
  }
};

/**
 * Lexical Command for inserting an image.
 */
export const INSERT_IMAGE_COMMAND: LexicalCommand<
  string | InsertImagePayload
> = createCommand();
