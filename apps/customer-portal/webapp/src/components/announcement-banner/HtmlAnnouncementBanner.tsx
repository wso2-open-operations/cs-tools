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

import { useState, type JSX, type MouseEvent } from "react";
import { announcementBannerConfig } from "@config/announcementBannerConfig";

function isDismissed(storageKey: string): boolean {
  try {
    return localStorage.getItem(storageKey) === "dismissed";
  } catch {
    return false;
  }
}

function persistDismissal(storageKey: string): void {
  try {
    localStorage.setItem(storageKey, "dismissed");
  } catch {
    // ignore storage errors
  }
}

/**
 * Renders the announcement banner from fully self-contained HTML configured in config.js.
 * All styling (background, padding, colors) lives in the HTML string.
 * Add `data-close-banner` attribute to any element in the HTML to make it dismiss the banner.
 * Dismissed state is persisted in localStorage per storageKey.
 */
export default function HtmlAnnouncementBanner(): JSX.Element | null {
  const { visible, storageKey, html } = announcementBannerConfig;
  const [closed, setClosed] = useState(() => isDismissed(storageKey));

  if (!visible || !html || closed) return null;

  const handleClick = (e: MouseEvent<HTMLDivElement>) => {
    const target = e.target as HTMLElement;
    if (target.closest("[data-close-banner]")) {
      persistDismissal(storageKey);
      setClosed(true);
    }
  };

  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: close is handled by the button inside the HTML
    <div
      onClick={handleClick}
      // biome-ignore lint/security/noDangerouslySetInnerHtml: operator-controlled config HTML
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
