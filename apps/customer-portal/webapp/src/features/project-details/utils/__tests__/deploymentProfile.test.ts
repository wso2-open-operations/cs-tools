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

import { describe, expect, it } from "vitest";
import {
  formatVersionWithProfile,
  getDisplayDeploymentProfile,
  getUpdateStreamKey,
  normalizeDeploymentProfile,
} from "@features/project-details/utils/deploymentProfile";

describe("normalizeDeploymentProfile", () => {
  it("trims a profile and treats blank, null and undefined as none", () => {
    expect(normalizeDeploymentProfile("  Traffic Manager ")).toBe(
      "Traffic Manager",
    );
    expect(normalizeDeploymentProfile("   ")).toBeNull();
    expect(normalizeDeploymentProfile("")).toBeNull();
    expect(normalizeDeploymentProfile(null)).toBeNull();
    expect(normalizeDeploymentProfile(undefined)).toBeNull();
  });
});

describe("getDisplayDeploymentProfile", () => {
  it("returns the profile only when it distinguishes", () => {
    expect(getDisplayDeploymentProfile("Traffic Manager")).toBe(
      "Traffic Manager",
    );
    expect(getDisplayDeploymentProfile(" Key Manager ")).toBe("Key Manager");
    expect(getDisplayDeploymentProfile("All In One")).toBeNull();
    expect(getDisplayDeploymentProfile("Standard")).toBeNull();
    expect(getDisplayDeploymentProfile("")).toBeNull();
    expect(getDisplayDeploymentProfile(null)).toBeNull();
    expect(getDisplayDeploymentProfile(undefined)).toBeNull();
  });
});

describe("formatVersionWithProfile", () => {
  it("appends the profile so records sharing a version can be told apart", () => {
    expect(formatVersionWithProfile("4.6.0", "API Control Plane")).toBe(
      "4.6.0 · API Control Plane",
    );
    expect(formatVersionWithProfile("4.6.0", "Traffic Manager")).toBe(
      "4.6.0 · Traffic Manager",
    );
  });

  it("keeps the plain version label for the default profiles most records carry", () => {
    // "All In One" and "Standard" are the profile of most version records;
    // suffixing them would change every existing label (and what the e2e
    // spec selects by exact text) for nothing.
    expect(formatVersionWithProfile("4.4.0", "All In One")).toBe("4.4.0");
    expect(formatVersionWithProfile("4.4.0", "all in one")).toBe("4.4.0");
    expect(formatVersionWithProfile("7.0.0", "Standard")).toBe("7.0.0");
    expect(formatVersionWithProfile("7.0.0", "  STANDARD ")).toBe("7.0.0");
  });

  it("still shows every profile that tells records apart", () => {
    for (const profile of [
      "Traffic Manager",
      "Key Manager",
      "API Publisher",
      "Developer Portal",
      "Gateway worker",
      "Micro-Integrator",
      "Universal Gateway",
    ]) {
      expect(formatVersionWithProfile("4.1.0", profile)).toBe(
        `4.1.0 · ${profile}`,
      );
    }
  });

  it("is just the version when there is no profile", () => {
    expect(formatVersionWithProfile("4.5.0")).toBe("4.5.0");
    expect(formatVersionWithProfile("4.5.0", null)).toBe("4.5.0");
    expect(formatVersionWithProfile("4.5.0", "  ")).toBe("4.5.0");
  });
});

describe("getUpdateStreamKey", () => {
  it("maps each API Manager profile that has its own stream to that stream", () => {
    expect(getUpdateStreamKey("wso2am", "API Control Plane")).toBe(
      "wso2am-acp",
    );
    expect(getUpdateStreamKey("wso2am", "Universal Gateway")).toBe(
      "wso2am-universal-gw",
    );
    expect(getUpdateStreamKey("wso2am", "Traffic Manager")).toBe("wso2am-tm");
  });

  it("matches the profile case-insensitively and ignores padding", () => {
    expect(getUpdateStreamKey("wso2am", "  traffic manager ")).toBe(
      "wso2am-tm",
    );
    expect(getUpdateStreamKey("wso2am", "API CONTROL PLANE")).toBe(
      "wso2am-acp",
    );
  });

  it("keeps the product's own stream for All In One, no profile or a profile without a stream", () => {
    expect(getUpdateStreamKey("wso2am", "All In One")).toBe("wso2am");
    expect(getUpdateStreamKey("wso2am", "Key Manager")).toBe("wso2am");
    expect(getUpdateStreamKey("wso2am", null)).toBe("wso2am");
    expect(getUpdateStreamKey("wso2am")).toBe("wso2am");
  });

  it("leaves a product with no catalogue key exactly as it was", () => {
    // A product whose `code` is null reaches here with its display label as the
    // key (recommendedLevelsProductKey). That never matched an update stream,
    // before profiles or now; the profile must not change that either way.
    expect(getUpdateStreamKey("WSO2 API Manager", "Traffic Manager")).toBe(
      "WSO2 API Manager",
    );
    expect(getUpdateStreamKey("WSO2 API Manager", null)).toBe(
      "WSO2 API Manager",
    );
  });

  it("never remaps another product, even for a profile name that matches", () => {
    expect(getUpdateStreamKey("wso2is", "Traffic Manager")).toBe("wso2is");
    expect(getUpdateStreamKey("", "Traffic Manager")).toBe("");
  });
});
