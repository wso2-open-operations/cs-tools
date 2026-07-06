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
  getAvatarColor,
  getInitials,
  getRoleBadges,
  getRoleChipSx,
} from "@features/settings/utils/settings";

describe("settings utils", () => {
  it("derives expected role badges", () => {
    const badges = getRoleBadges({
      isCsAdmin: true,
      isCsIntegrationUser: false,
      isPortalUser: false,
      isSecurityContact: true,
      account: { classification: "Partner" },
    } as never);
    expect(badges.map((badge) => badge.label)).toEqual([
      "Admin",
      "Security User",
      "Partner",
    ]);
  });

  it("returns default portal badge when no explicit end-user role exists", () => {
    const badges = getRoleBadges({
      isCsIntegrationUser: false,
      isPortalUser: false,
      isSecurityContact: false,
    } as never);
    expect(badges[0]?.label).toBe("Portal User");
  });

  it("resolves initials from names and email fallback", () => {
    expect(getInitials("Jane", "Doe")).toBe("JD");
    expect(getInitials(undefined, undefined, "ab@example.com")).toBe("AB");
  });

  it("creates stable avatar colors and role chip sx", () => {
    expect(getAvatarColor("user-1")).toBe(getAvatarColor("user-1"));
    expect(getRoleChipSx("primary")).toMatchObject({ borderColor: expect.any(String) });
  });
});

