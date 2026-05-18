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
import { DeviceType, MobileOs, NavigatorPlatform } from "@/types/mobileDevice";
import { detectMobileDevice } from "@utils/deviceDetection";

function mockNavigator(partial: Partial<Navigator> & { userAgent: string }): void {
  vi.stubGlobal("navigator", {
    platform: "iPhone",
    maxTouchPoints: 1,
    ...partial,
  });
}

describe("detectMobileDevice", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("should detect iPhone as iOS phone", () => {
    mockNavigator({
      userAgent:
        "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15",
    });

    expect(detectMobileDevice()).toEqual({
      os: MobileOs.Ios,
      deviceType: DeviceType.Phone,
    });
  });

  it("should detect Android phone when Mobile is present in user agent", () => {
    mockNavigator({
      userAgent:
        "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Mobile Safari/537.36",
    });

    expect(detectMobileDevice()).toEqual({
      os: MobileOs.Android,
      deviceType: DeviceType.Phone,
    });
  });

  it("should ignore Android tablets by default", () => {
    mockNavigator({
      userAgent:
        "Mozilla/5.0 (Linux; Android 14; SM-X900) AppleWebKit/537.36 Safari/537.36",
    });

    expect(detectMobileDevice()).toBeNull();
  });

  it("should detect Android tablets when includeTablets is true", () => {
    mockNavigator({
      userAgent:
        "Mozilla/5.0 (Linux; Android 14; SM-X900) AppleWebKit/537.36 Safari/537.36",
    });

    expect(detectMobileDevice({ includeTablets: true })).toEqual({
      os: MobileOs.Android,
      deviceType: DeviceType.Tablet,
    });
  });

  it("should detect iPad as iOS tablet when includeTablets is true", () => {
    mockNavigator({
      userAgent:
        "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15",
      platform: NavigatorPlatform.MacIntel,
      maxTouchPoints: 5,
    });

    expect(detectMobileDevice({ includeTablets: true })).toEqual({
      os: MobileOs.Ios,
      deviceType: DeviceType.Tablet,
    });
  });

  it("should return null for desktop Chrome", () => {
    mockNavigator({
      userAgent:
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
      platform: NavigatorPlatform.MacIntel,
      maxTouchPoints: 0,
    });

    expect(detectMobileDevice()).toBeNull();
  });
});
