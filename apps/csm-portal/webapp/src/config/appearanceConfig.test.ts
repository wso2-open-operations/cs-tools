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
import {
  DEFAULT_APP_NAME,
  DEFAULT_LOGO_DARK_MODE_URL,
  DEFAULT_LOGO_LIGHT_MODE_URL,
  appName,
  logoDarkModeUrl,
  logoLightModeUrl,
} from "@config/appearanceConfig";

function setConfig(config: Record<string, unknown>): void {
  window.config = config as unknown as typeof window.config;
}

afterEach(() => {
  window.config = {} as typeof window.config;
});

describe("appName", () => {
  it("defaults to CSM Portal when unset", () => {
    setConfig({});
    expect(appName()).toBe(DEFAULT_APP_NAME);
  });

  it("reads and trims the configured name", () => {
    setConfig({ CSM_PORTAL_APP_NAME: "  Acme Support  " });
    expect(appName()).toBe("Acme Support");
  });

  it("treats a blank or non-string value as unset", () => {
    for (const raw of ["", "   ", null, undefined, 42]) {
      setConfig({ CSM_PORTAL_APP_NAME: raw });
      expect(appName()).toBe(DEFAULT_APP_NAME);
    }
  });
});

describe("logoLightModeUrl", () => {
  it("defaults to /logo-dark.svg when unset", () => {
    setConfig({});
    expect(logoLightModeUrl()).toBe(DEFAULT_LOGO_LIGHT_MODE_URL);
  });

  it("returns an absolute path or URL unchanged", () => {
    setConfig({ CSM_PORTAL_LOGO_LIGHT_MODE_URL: "/logos/light.svg" });
    expect(logoLightModeUrl()).toBe("/logos/light.svg");
    setConfig({ CSM_PORTAL_LOGO_LIGHT_MODE_URL: "https://cdn.test/logo.svg" });
    expect(logoLightModeUrl()).toBe("https://cdn.test/logo.svg");
  });

  it("anchors a bare relative path to the site root", () => {
    setConfig({ CSM_PORTAL_LOGO_LIGHT_MODE_URL: "./logos/light.svg" });
    expect(logoLightModeUrl()).toBe("/logos/light.svg");
    setConfig({ CSM_PORTAL_LOGO_LIGHT_MODE_URL: "logos/light.svg" });
    expect(logoLightModeUrl()).toBe("/logos/light.svg");
  });
});

describe("logoDarkModeUrl", () => {
  it("defaults to /logo-white.svg when nothing is set", () => {
    setConfig({});
    expect(logoDarkModeUrl()).toBe(DEFAULT_LOGO_DARK_MODE_URL);
  });

  it("falls back to the light logo when only that is configured", () => {
    setConfig({ CSM_PORTAL_LOGO_LIGHT_MODE_URL: "https://cdn.test/logo.svg" });
    expect(logoDarkModeUrl()).toBe("https://cdn.test/logo.svg");
  });

  it("prefers its own configured value over the light logo", () => {
    setConfig({
      CSM_PORTAL_LOGO_LIGHT_MODE_URL: "https://cdn.test/light.svg",
      CSM_PORTAL_LOGO_DARK_MODE_URL: "https://cdn.test/dark.svg",
    });
    expect(logoDarkModeUrl()).toBe("https://cdn.test/dark.svg");
  });

  it("anchors a bare relative path to the site root", () => {
    setConfig({ CSM_PORTAL_LOGO_DARK_MODE_URL: "./logos/dark.svg" });
    expect(logoDarkModeUrl()).toBe("/logos/dark.svg");
  });
});
