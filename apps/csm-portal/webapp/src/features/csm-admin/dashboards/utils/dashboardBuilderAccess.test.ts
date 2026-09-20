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
import { hasDashboardBuilderAccess } from "@features/csm-admin/dashboards/utils/dashboardBuilderAccess";

describe("hasDashboardBuilderAccess", () => {
  it("is true when roles include admin", () => {
    expect(hasDashboardBuilderAccess(["agent", "admin"])).toBe(true);
  });

  it("is true when roles include dashboard_designer (without the full admin role)", () => {
    expect(hasDashboardBuilderAccess(["agent", "dashboard_designer"])).toBe(true);
  });

  it("is true when roles include both admin and dashboard_designer", () => {
    expect(hasDashboardBuilderAccess(["admin", "dashboard_designer"])).toBe(true);
  });

  it("matches case-insensitively", () => {
    expect(hasDashboardBuilderAccess(["Admin"])).toBe(true);
    expect(hasDashboardBuilderAccess(["Dashboard_Designer"])).toBe(true);
  });

  it("is false without the admin or dashboard_designer role", () => {
    expect(hasDashboardBuilderAccess(["agent", "commenter"])).toBe(false);
  });

  it("is false for undefined/empty roles", () => {
    expect(hasDashboardBuilderAccess(undefined)).toBe(false);
    expect(hasDashboardBuilderAccess([])).toBe(false);
  });
});
