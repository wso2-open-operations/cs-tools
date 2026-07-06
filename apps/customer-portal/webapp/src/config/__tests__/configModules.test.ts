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

import { beforeEach, describe, expect, it, vi } from "vitest";

describe("config modules", () => {
  beforeEach(() => {
    vi.resetModules();
    (
      window as unknown as {
        config?: Record<string, string>;
      }
    ).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
      CUSTOMER_PORTAL_THEME: "acrylicOrange",
      CUSTOMER_PORTAL_AUTH_BASE_URL: "https://auth.test",
      CUSTOMER_PORTAL_AUTH_CLIENT_ID: "client-id",
      CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "https://app.test/sign-in",
      CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "https://app.test/sign-out",
    };
  });

  it("apiConfig exposes backend URL from window.config", async () => {
    const { apiConfig } = await import("@config/apiConfig");
    expect(apiConfig.backendUrl).toBe("https://api.test");
  });

  it("themeConfig module loads", async () => {
    const module = await import("@config/themeConfig");
    expect(module.themeConfig).toBeTruthy();
  }, 30_000);

  it("portal and auth config modules load", async () => {
    const portal = await import("@config/portalConfig");
    const { authConfig } = await import("@config/authConfig");
    expect(portal).toBeDefined();
    expect(authConfig.clientId).toBe("client-id");
  });

  it("banner config modules expose visibility flags", async () => {
    const top = await import("@config/topBannersConfig");
    const notification = await import("@config/notificationBannerConfig");
    const announcement = await import("@config/announcementBannerConfig");
    expect(Array.isArray(top.topBannersConfig)).toBe(true);
    expect(typeof notification.notificationBannerConfig).toBe("object");
    expect(typeof announcement.announcementBannerConfig).toBe("object");
  });

  it("logger and mobile app config modules load", async () => {
    const logger = await import("@config/loggerConfig");
    const { getMobileAppConfig } = await import("@config/mobileAppConfig");
    expect(logger.loggerConfig).toBeDefined();
    expect(getMobileAppConfig()).toBeDefined();
  });
});
