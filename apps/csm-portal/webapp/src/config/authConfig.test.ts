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

type TestConfig = Record<string, string | undefined>;

function setConfig(overrides: TestConfig = {}): void {
  (window as unknown as { config: TestConfig }).config = {
    CSM_PORTAL_AUTH_BASE_URL: "https://auth.test",
    CSM_PORTAL_AUTH_CLIENT_ID: "client-id",
    CSM_PORTAL_BACKEND_BASE_URL: "https://api.test",
    ...overrides,
  };
}

describe("authConfig", () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it("leaves the redirect URLs undefined when they are not set, so the SDK uses the page origin", async () => {
    setConfig();
    const { authConfig } = await import("./authConfig");
    expect(authConfig.signInRedirectURL).toBeUndefined();
    expect(authConfig.signOutRedirectURL).toBeUndefined();
  });

  it("treats blank redirect URLs as unset", async () => {
    setConfig({
      CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "",
      CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "   ",
    });
    const { authConfig } = await import("./authConfig");
    expect(authConfig.signInRedirectURL).toBeUndefined();
    expect(authConfig.signOutRedirectURL).toBeUndefined();
  });

  it("passes configured redirect URLs through", async () => {
    setConfig({
      CSM_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "https://app.test/in",
      CSM_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "https://app.test/out",
    });
    const { authConfig } = await import("./authConfig");
    expect(authConfig.signInRedirectURL).toBe("https://app.test/in");
    expect(authConfig.signOutRedirectURL).toBe("https://app.test/out");
  });

  it("still requires the identity provider base URL and client id", async () => {
    setConfig({ CSM_PORTAL_AUTH_BASE_URL: undefined, CSM_PORTAL_AUTH_CLIENT_ID: undefined });
    await expect(import("./authConfig")).rejects.toThrow(
      /CSM_PORTAL_AUTH_BASE_URL, CSM_PORTAL_AUTH_CLIENT_ID/,
    );
  });
});
