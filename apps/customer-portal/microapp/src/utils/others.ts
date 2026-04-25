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

import { useEffect } from "react";
import { STRING_OVERRIDES } from "../components/features/support/config";
import { matchPath, useLocation } from "react-router-dom";
import { SCROLL_OVERRIDES } from "../components/layout/config";
import { LOCAL_STORAGE_LAST_VISITED_PROJECT_KEY } from "../config/constants";

export const stringAvatar = (name: string) => {
  if (!name) return "";
  const parts = name.trim().split(/\s+/);

  if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
  return `${parts[0].charAt(0) + parts[parts.length - 1].charAt(0)}`.toUpperCase();
};

export function capitalize(text: string): string {
  if (!text) return text;

  return text
    .trim()
    .split(/\s+/)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(" ");
}

export const stripHtmlTags = (str: string): string => {
  if (!str) return "";
  const txt = document.createElement("textarea");
  txt.innerHTML = str.replace(/<br\s*\/?>/gi, " ");
  return txt.value.replace(/<[^>]*>?/gm, "");
};

export const overrideOrDefault = (s: string) => {
  return STRING_OVERRIDES[s] ?? s;
};

export function useScrollControl(position: "top" | "bottom" = "top", onRouteChange = true) {
  const { pathname } = useLocation();

  const scroll = () => {
    const matched = SCROLL_OVERRIDES.find((o) => matchPath(o.path, pathname));
    const pos = matched?.position ?? position;

    if (pos === "top") window.scrollTo({ top: 0, behavior: "smooth" });
    else if (pos === "bottom") window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
    else window.scrollTo({ top: pos, behavior: "smooth" });
  };

  useEffect(() => {
    if (onRouteChange) scroll();

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname, onRouteChange]);

  return scroll;
}

export const setLastVisitedProjectId = (projectId: string | null) => {
  if (projectId === null) {
    document.cookie = `${LOCAL_STORAGE_LAST_VISITED_PROJECT_KEY}=; path=/`;
    return;
  }

  document.cookie = `${LOCAL_STORAGE_LAST_VISITED_PROJECT_KEY}=${projectId}; path=/`;
};

export function getLastVisitedProjectId(): string | undefined {
  const name = LOCAL_STORAGE_LAST_VISITED_PROJECT_KEY + "=";
  const decodedCookie = decodeURIComponent(document.cookie);
  const ca = decodedCookie.split(";");

  for (let i = 0; i < ca.length; i++) {
    let c = ca[i];
    while (c.charAt(0) == " ") {
      c = c.substring(1);
    }
    if (c.indexOf(name) == 0) {
      const value = c.substring(name.length, c.length);

      if (!value || value === "null") return undefined;

      return value;
    }
  }

  return undefined;
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
