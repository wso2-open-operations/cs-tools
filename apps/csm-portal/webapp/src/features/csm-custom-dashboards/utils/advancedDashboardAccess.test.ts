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

import { afterEach, describe, expect, it, vi } from "vitest";
import { getPortalAccess } from "@context/current-user/portalAccess";
import {
  canGenerateAdvancedDashboards,
  canViewAdvancedDashboards,
} from "@features/csm-custom-dashboards/utils/advancedDashboardAccess";

// The hook reads the signed-in user; these tests pass the access in.
vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => getPortalAccess(undefined) }));

describe("canGenerateAdvancedDashboards", () => {
  const original = window.config;
  afterEach(() => {
    window.config = original;
  });

  it("is true for a CS engineer or an admin when the code approach is on", () => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: true };
    expect(canGenerateAdvancedDashboards(getPortalAccess(["cs_engineer"]))).toBe(true);
    expect(canGenerateAdvancedDashboards(getPortalAccess(["admin"]))).toBe(true);
  });

  it("is false for any other role, or no roles", () => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: true };
    expect(canGenerateAdvancedDashboards(getPortalAccess(["viewer"]))).toBe(false);
    expect(canGenerateAdvancedDashboards(getPortalAccess([]))).toBe(false);
    expect(canGenerateAdvancedDashboards(getPortalAccess(undefined))).toBe(false);
  });

  it("is false for everyone while the code approach is off", () => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: undefined };
    expect(canGenerateAdvancedDashboards(getPortalAccess(["cs_engineer"]))).toBe(false);
    expect(canGenerateAdvancedDashboards(getPortalAccess(["admin"]))).toBe(false);
  });
});

describe("canViewAdvancedDashboards", () => {
  const original = window.config;
  afterEach(() => {
    window.config = original;
  });

  it("is true while the code approach is on", () => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: true };
    expect(canViewAdvancedDashboards()).toBe(true);
  });

  it("is false while the code approach is off", () => {
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: undefined };
    expect(canViewAdvancedDashboards()).toBe(false);
    window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: false };
    expect(canViewAdvancedDashboards()).toBe(false);
  });
});
