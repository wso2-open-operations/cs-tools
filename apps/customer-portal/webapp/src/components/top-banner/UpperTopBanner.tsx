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
import { upperTopBannerConfig } from "@config/upperTopBannerConfig";
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

/**
 * Renders the upper top banner sourced from CUSTOMER_PORTAL_UPPER_TOP_BANNER_HTML in config.js.
 * Close button visibility is controlled by CUSTOMER_PORTAL_UPPER_TOP_BANNER_CLOSEABLE.
 */
export default function UpperTopBanner(): JSX.Element | null {
  const { enabled, html, closeable, storageKey } = upperTopBannerConfig;
  const logger = useLogger();
  const [closed, setClosed] = useState(() =>
    closeable ? isDismissed(storageKey) : false,
  );

  useEffect(() => {
    if (closeable && !window.config?.CUSTOMER_PORTAL_UPPER_TOP_BANNER_STORAGE_KEY) {
      logger.warn(
        "CUSTOMER_PORTAL_UPPER_TOP_BANNER_STORAGE_KEY is missing from config.js. " +
          "A fallback key is being used — dismiss state may persist incorrectly across deployments.",
      );
    }
  }, [closeable, logger]);

  if (!enabled || !html || closed) return null;

  const handleClose = (): void => {
    persistDismissal(storageKey);
    setClosed(true);
  };

  return (
    <div style={{ position: "relative", lineHeight: 0, fontSize: 0, overflow: "hidden" }}>
      {/* biome-ignore lint/security/noDangerouslySetInnerHtml: operator-controlled config HTML */}
      <div dangerouslySetInnerHTML={{ __html: html }} />
      {closeable && (
        <button
          type="button"
          onClick={handleClose}
          aria-label="Close banner"
          style={{
            position: "absolute",
            top: "50%",
            right: "12px",
            transform: "translateY(-50%)",
            background: "rgba(0,0,0,.55)",
            border: "1px solid rgba(255,255,255,.35)",
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
          }}
        >
          &times;
        </button>
      )}
    </div>
  );
}
