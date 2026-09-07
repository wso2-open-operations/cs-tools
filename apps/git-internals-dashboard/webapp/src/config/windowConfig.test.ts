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

import { afterEach, describe, expect, it } from "vitest";
import { windowConfig } from "./windowConfig";

describe("windowConfig", () => {
  afterEach(() => {
    // @ts-expect-error -- test-only cleanup of the global window.config
    delete window.config;
  });

  it("reads a required key when present", () => {
    // @ts-expect-error -- partial config is fine for this test
    window.config = { GID_AUTH_BASE_URL: "https://idp.example.com" };
    expect(windowConfig.authBaseUrl()).toBe("https://idp.example.com");
  });

  it("throws a descriptive error when a required key is missing", () => {
    // @ts-expect-error -- partial config is fine for this test
    window.config = {};
    expect(() => windowConfig.authBaseUrl()).toThrow(/GID_AUTH_BASE_URL/);
  });

  it("falls back to a default for optional keys", () => {
    // @ts-expect-error -- partial config is fine for this test
    window.config = {};
    expect(windowConfig.logLevel()).toBe("ERROR");
    expect(windowConfig.authScopes()).toBe("openid profile");
  });

  it("prefers an explicitly configured optional value over the default", () => {
    // @ts-expect-error -- partial config is fine for this test
    window.config = { GID_LOG_LEVEL: "DEBUG" };
    expect(windowConfig.logLevel()).toBe("DEBUG");
  });
});
