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
  }, [pathname, onRouteChange]);

  return scroll;
}
