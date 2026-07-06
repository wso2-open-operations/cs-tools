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

import { useState, useEffect, type JSX } from "react";
import { type TopBannerItem, topBannersConfig } from "@config/topBannersConfig";
import { useLogger } from "@hooks/useLogger";

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

interface BannerProps {
  banner: TopBannerItem;
}

const FALLBACK_STORAGE_KEY = "top_banner_fallback_v1";

function Banner({ banner }: BannerProps): JSX.Element | null {
  const { html, closeable } = banner;
  const resolvedStorageKey = banner.storageKey || FALLBACK_STORAGE_KEY;
  const logger = useLogger();
  const [closed, setClosed] = useState(() =>
    closeable ? isDismissed(resolvedStorageKey) : false,
  );

  useEffect(() => {
    if (closeable && !banner.storageKey) {
      logger.warn(
        "A top banner has closeable: true but no storageKey set. " +
          "A fallback key is being used — dismiss state may persist incorrectly.",
      );
    }
  }, [closeable, banner.storageKey, logger]);

  if (closed) return null;

  const handleClose = (): void => {
    persistDismissal(resolvedStorageKey);
    setClosed(true);
  };

  return (
    <div style={{ position: "relative", overflow: "hidden" }}>
      {/* biome-ignore lint/security/noDangerouslySetInnerHtml: operator-controlled config HTML */}
      <div dangerouslySetInnerHTML={{ __html: html }} />
      {closeable && (
        <div
          style={{
            position: "absolute",
            inset: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "flex-end",
            paddingRight: "12px",
            pointerEvents: "none",
          }}
        >
          <button
            type="button"
            onClick={handleClose}
            aria-label="Close banner"
            style={{
              pointerEvents: "auto",
              background: "rgba(0,0,0,.55)",
              border: "1px solid rgba(255,255,255,.6)",
              color: "#fff",
              width: "24px",
              height: "24px",
              fontSize: "14px",
              lineHeight: "1",
              cursor: "pointer",
              borderRadius: "4px",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
            }}
          >
            &times;
          </button>
        </div>
      )}
    </div>
  );
}

/**
 * Renders all enabled top banners defined in CUSTOMER_PORTAL_TOP_BANNERS in config.js.
 * Banners are rendered top-to-bottom in array order.
 * Each banner independently tracks its own dismiss state via its storageKey.
 */
export default function TopBanners(): JSX.Element | null {
  const banners = topBannersConfig.filter((b) => b.enabled);

  if (banners.length === 0) return null;

  return (
    <>
      {banners.map((banner, index) => (
        <Banner key={banner.storageKey || index} banner={banner} />
      ))}
    </>
  );
}
