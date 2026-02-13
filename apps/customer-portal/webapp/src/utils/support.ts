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

import {
  CircleAlert,
  CircleCheck,
  CircleQuestionMark,
  Clock,
  MessageCircle,
} from "@wso2/oxygen-ui-icons-react";
import { ChatAction, ChatStatus } from "@constants/supportConstants";
import type { Theme } from "@wso2/oxygen-ui";
import { createElement, type ComponentType, type ReactNode } from "react";

export type ChatActionState =
  | "primary"
  | "info"
  | "success"
  | "warning"
  | "error";

/**
 * Formats a value for display in case details; null/undefined/empty become "--".
 *
 * @param value - Raw value from API or state.
 * @returns {string} Display string.
 */
export function formatValue(value: string | number | null | undefined): string {
  if (value == null || value === "") return "--";
  return String(value);
}

/**
 * Formats byte count for display (e.g. 1024 -> "1 KB", 245760 -> "240 KB").
 *
 * @param bytes - Size in bytes (number or string from API).
 * @returns {string} Formatted string like "1.2 MB" or "18 KB".
 */
export function formatFileSize(bytes: number | string | null | undefined): string {
  const n = typeof bytes === "string" ? parseInt(bytes, 10) : bytes;
  if (n == null || Number.isNaN(n)) return "--";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1).replace(/\.0$/, "")} KB`;
  return `${(n / (1024 * 1024)).toFixed(1).replace(/\.0$/, "")} MB`;
}

/**
 * Returns whether to show View or Resume for a chat status.
 *
 * @param status - Chat status string (e.g. Resolved, Still Open, Abandoned).
 * @returns {ChatAction} "view" or "resume".
 */
export function getChatStatusAction(status: string): ChatAction {
  const normalized = status?.toLowerCase() || "";
  if (normalized.includes("open")) return ChatAction.RESUME;
  return ChatAction.VIEW;
}

/**
 * Returns the color for a chat action button.
 *
 * @param action - The action type ("view" or "resume").
 * @returns {ChatActionState} Palette color path.
 */
export function getChatActionColor(action: ChatAction): ChatActionState {
  if (action === ChatAction.RESUME) {
    return "info";
  }
  return "primary";
}

/**
 * Returns the color path for a chat status.
 *
 * @param status - Chat status string.
 * @returns {string} Palette color path.
 */
export function getChatStatusColor(status: string): string {
  const normalized = status?.toLowerCase() || "";
  if (normalized.includes(ChatStatus.RESOLVED.toLowerCase())) {
    return "success.main";
  }
  if (normalized.includes(ChatStatus.STILL_OPEN.toLowerCase())) {
    return "info.main";
  }
  if (normalized.includes(ChatStatus.ABANDONED.toLowerCase())) {
    return "error.main";
  }
  return "secondary.main";
}

/**
 * Resolves a color from the theme palette for the alpha() utility.
 *
 * @param path - Color path.
 * @param theme - Oxygen UI theme.
 * @returns {string} The resolved color string.
 */
export function resolveColorFromTheme(path: string, theme: Theme): string {
  return (
    (path
      .split(".")
      .reduce(
        (acc: any, part: string) => acc?.[part],
        theme.palette,
      ) as string) || path
  );
}

/**
 * Formats a date string or Date object into a relative time (e.g., "2 days ago", "1 hour ago").
 *
 * @param date - Date string or Date object.
 * @returns {string} Human readable relative time.
 */
export function formatRelativeTime(date: string | Date | undefined): string {
  if (!date) return "--";

  const d = typeof date === "string" ? new Date(date) : date;
  if (isNaN(d.getTime())) return "--";

  const now = new Date();
  const diffInSeconds = Math.floor((now.getTime() - d.getTime()) / 1000);

  if (diffInSeconds < 60) return "just now";

  const diffInMinutes = Math.floor(diffInSeconds / 60);
  if (diffInMinutes < 60) {
    return `${diffInMinutes} minute${diffInMinutes === 1 ? "" : "s"} ago`;
  }

  const diffInHours = Math.floor(diffInMinutes / 60);
  if (diffInHours < 24) {
    return `${diffInHours} hour${diffInHours === 1 ? "" : "s"} ago`;
  }

  const diffInDays = Math.floor(diffInHours / 24);
  if (diffInDays < 30) {
    return `${diffInDays} day${diffInDays === 1 ? "" : "s"} ago`;
  }

  const diffInMonths = Math.floor(diffInDays / 30);
  if (diffInMonths < 12) {
    return `${diffInMonths} month${diffInMonths === 1 ? "" : "s"} ago`;
  }

  const diffInYears = Math.floor(diffInMonths / 12);
  return `${diffInYears} year${diffInYears === 1 ? "" : "s"} ago`;
}

/**
 * Derives the label and pluralized "all" label from a filter ID.
 *
 * @param id - The filter ID (e.g., "status").
 * @returns { label: string; allLabel: string } The derived labels.
 */
export function deriveFilterLabels(id: string): {
  label: string;
  allLabel: string;
} {
  const label = id.charAt(0).toUpperCase() + id.slice(1);
  const allLabel = `All ${
    label.endsWith("s")
      ? `${label}es`
      : label.endsWith("y")
        ? `${label.slice(0, -1)}ies`
        : `${label}s`
  }`;

  return { allLabel, label };
}

/** Attachment file category for icon selection. */
export type AttachmentFileCategory =
  | "image"
  | "pdf"
  | "archive"
  | "text"
  | "file";

/**
 * Returns the file category for attachment icon/display (image, pdf, archive, text, file).
 *
 * @param fileName - File name.
 * @param type - MIME type.
 * @returns {AttachmentFileCategory} The category.
 */
export function getAttachmentFileCategory(
  fileName: string,
  type: string,
): AttachmentFileCategory {
  const n = fileName.toLowerCase();
  const t = type.toLowerCase();
  if (
    /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i.test(n) ||
    t.startsWith("image/")
  ) {
    return "image";
  }
  if (/\.pdf$/i.test(n) || t.includes("pdf")) return "pdf";
  if (
    /\.(zip|rar|7z|tar|gz)$/i.test(n) ||
    t.includes("zip") ||
    t.includes("archive")
  ) {
    return "archive";
  }
  if (/\.(log|txt)$/i.test(n) || t.startsWith("text/")) return "text";
  return "file";
}

/**
 * Returns the icon component for a given case status label.
 *
 * @param statusLabel - The case status label (e.g., "Open", "Working in Progress").
 * @returns {ComponentType<{ size?: number }>} The icon component.
 */
export function getStatusIcon(
  statusLabel?: string,
): ComponentType<{ size?: number }> {
  const normalized = statusLabel?.toLowerCase() || "";
  if (normalized.includes("open")) return CircleAlert;
  if (normalized.includes("progress")) return Clock;
  if (normalized.includes("awaiting")) return MessageCircle;
  if (normalized.includes("waiting")) return CircleQuestionMark;
  if (normalized.includes("resolved") || normalized.includes("closed"))
    return CircleCheck;
  return CircleAlert;
}

/**
 * Returns the icon element for a given case status label (avoids creating a component during render).
 *
 * @param statusLabel - The case status label.
 * @param size - Icon size in pixels.
 * @returns {ReactNode} The icon element.
 */
export function getStatusIconElement(
  statusLabel: string | null | undefined,
  size = 12,
): ReactNode {
  const Icon = getStatusIcon(statusLabel ?? undefined);
  return createElement(Icon, { size });
}

/**
 * Formats a comment/activity date for display (e.g. "Jan 1, 2026, 2:30 PM").
 *
 * @param date - ISO date string or parseable date.
 * @returns {string} Formatted date string.
 */
export function formatCommentDate(date: string | undefined): string {
  if (!date) return "--";
  const d = new Date(date);
  if (Number.isNaN(d.getTime())) return "--";
  return d.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/**
 * Strips a code-block wrapper from HTML content if present (e.g. outer div from rich editor).
 *
 * @param html - Raw HTML string.
 * @returns {string} HTML without the wrapper.
 */
export function stripCodeWrapper(html: string): string {
  if (!html?.trim()) return "";
  const trimmed = html.trim();
  const match = trimmed.match(
    /^<div[^>]*class="[^"]*code-wrapper[^"]*"[^>]*>([\s\S]*)<\/div>$/i,
  );
  if (match) return match[1].trim();
  return trimmed;
}

/** Shape used by replaceInlineImageSources (id -> url mapping). */
export interface InlineAttachmentRef {
  id: string;
  url: string;
}

/**
 * Replaces inline image placeholders (e.g. cid:xxx or data-attachment-id) with URLs from attachments.
 *
 * @param html - HTML string that may contain img src placeholders.
 * @param attachments - Optional list of inline attachments (id, url).
 * @returns {string} HTML with image sources replaced.
 */
export function replaceInlineImageSources(
  html: string,
  attachments: InlineAttachmentRef[] | undefined,
): string {
  if (!html?.trim()) return "";
  if (!attachments?.length) return html;
  const byId = new Map(attachments.map((a) => [a.id, a.url]));
  return html.replace(
    /(<img[^>]*\ssrc=)(["'])(?:cid:)?([^"']+)\2/gi,
    (_match, prefix, quote, id) => {
      const url = byId.get(id.trim()) ?? "";
      return `${prefix}${quote}${url}${quote}`;
    },
  );
}
