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

import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DeviceType, MobileOs } from "@/types/mobileDevice";
import MobileAppGate from "@providers/MobileAppGate";

vi.mock("@utils/deviceDetection", () => ({
  detectMobileDevice: vi.fn(),
}));

vi.mock("@config/mobileAppConfig", () => ({
  getMobileAppConfig: vi.fn(() => ({
    enabled: true,
    iosStoreUrl: "https://apps.apple.com/app/example",
    androidStoreUrl: "https://play.google.com/store/apps/details?id=example",
    includeTablets: false,
  })),
  getMobileAppStoreUrl: (os: string) =>
    os === "ios"
      ? "https://apps.apple.com/app/example"
      : "https://play.google.com/store/apps/details?id=example",
}));

import { detectMobileDevice } from "@utils/deviceDetection";

describe("MobileAppGate", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("should render children on desktop", () => {
    vi.mocked(detectMobileDevice).mockReturnValue(null);

    render(
      <MobileAppGate>
        <div data-testid="portal-app">Portal</div>
      </MobileAppGate>,
    );

    expect(screen.getByTestId("portal-app")).toBeInTheDocument();
    expect(
      screen.queryByTestId("mobile-app-download-button"),
    ).toBeNull();
  });

  it("should show mobile prompt on iOS phone", () => {
    vi.mocked(detectMobileDevice).mockReturnValue({
      os: MobileOs.Ios,
      deviceType: DeviceType.Phone,
    });

    render(
      <MobileAppGate>
        <div data-testid="portal-app">Portal</div>
      </MobileAppGate>,
    );

    expect(screen.getByTestId("mobile-app-download-button")).toBeInTheDocument();
    expect(screen.queryByTestId("portal-app")).toBeNull();
  });
});
