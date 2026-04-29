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

import { STRING_OVERRIDES } from "@components/support/config";

export function stringAvatar(name: string): string {
  if (!name) return "";
  const parts = name.trim().split(/\s+/);

  if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
  return `${parts[0].charAt(0) + parts[parts.length - 1].charAt(0)}`.toUpperCase();
}

export function capitalize(text: string): string {
  if (!text) return text;

  return text
    .trim()
    .split(/\s+/)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(" ");
}

export function stripHtmlTags(str: string): string {
  if (!str) return "";
  const txt = document.createElement("textarea");
  txt.innerHTML = str.replace(/<br\s*\/?>/gi, " ");
  return txt.value.replace(/<[^>]*>?/gm, "");
}

export function overrideOrDefault(s: string): string {
  return (STRING_OVERRIDES as Record<string, string>)[s] ?? s;
}

export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "0 seconds";

  const totalSeconds = Math.floor(ms / 1000);

  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  const parts: string[] = [];

  if (days) parts.push(`${days} day${days !== 1 ? "s" : ""}`);
  if (hours) parts.push(`${hours} hour${hours !== 1 ? "s" : ""}`);
  if (minutes) parts.push(`${minutes} minute${minutes !== 1 ? "s" : ""}`);
  if (seconds || parts.length === 0) {
    parts.push(`${seconds} second${seconds !== 1 ? "s" : ""}`);
  }

  return parts.join(" ");
}
