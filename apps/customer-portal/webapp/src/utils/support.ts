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
export function formatValue(
  value: string | number | null | undefined,
): string {
  if (value == null || value === "") return "--";
  return String(value);
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

/**
 * Returns the icon component for a given case status label.
 *
 * @param statusLabel - The case status label (e.g., "Open", "Working in Progress").
 * @returns {ComponentType<{ size?: number }>} The icon component.
 */
export function getStatusIcon(statusLabel?: string): ComponentType<{ size?: number }> {
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
  size = 12
): ReactNode {
  const Icon = getStatusIcon(statusLabel ?? undefined);
  return createElement(Icon, { size });
}
