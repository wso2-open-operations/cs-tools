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
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DeviceType, MobileOs } from "@/types/mobileDevice";
import MobileAppPromptPage from "@components/mobile-app/MobileAppPromptPage";
import { getMobileAppStoreUrl } from "@config/mobileAppConfig";

const mockGetMobileAppStoreUrl = vi.hoisted(() =>
  vi.fn((os: string) =>
    os === "ios"
      ? "https://apps.apple.com/app/example"
      : "https://play.google.com/store/apps/details?id=example",
  ),
);

vi.mock("@config/mobileAppConfig", () => ({
  getMobileAppStoreUrl: mockGetMobileAppStoreUrl,
}));

describe("MobileAppPromptPage", () => {
  beforeEach(() => {
    mockGetMobileAppStoreUrl.mockImplementation((os: string) =>
      os === "ios"
        ? "https://apps.apple.com/app/example"
        : "https://play.google.com/store/apps/details?id=example",
    );
  });

  it("should show micro app instructions", () => {
    render(
      <MobileAppPromptPage
        device={{ os: MobileOs.Ios, deviceType: DeviceType.Phone }}
      />,
    );

    expect(screen.getByText("Get the WSO2 Super App")).toBeInTheDocument();
    expect(
      screen.getByText(/micro app inside the WSO2 Super App/i),
    ).toBeInTheDocument();
  });

  it("should show App Store download for iOS phones", () => {
    render(
      <MobileAppPromptPage
        device={{ os: MobileOs.Ios, deviceType: DeviceType.Phone }}
      />,
    );

    const link = screen.getByTestId("mobile-app-download-button");
    expect(link).toHaveAttribute(
      "href",
      "https://apps.apple.com/app/example",
    );
    expect(link).toHaveTextContent("Download on the App Store");
  });

  it("should show Google Play download for Android phones", () => {
    render(
      <MobileAppPromptPage
        device={{ os: MobileOs.Android, deviceType: DeviceType.Phone }}
      />,
    );

    const link = screen.getByTestId("mobile-app-download-button");
    expect(link).toHaveAttribute(
      "href",
      "https://play.google.com/store/apps/details?id=example",
    );
    expect(link).toHaveTextContent("Get it on Google Play");
  });

  it("should show fallback when the store URL is not configured", () => {
    vi.mocked(getMobileAppStoreUrl).mockReturnValueOnce(undefined);

    render(
      <MobileAppPromptPage
        device={{ os: MobileOs.Ios, deviceType: DeviceType.Phone }}
      />,
    );

    expect(
      screen.getByText(/download link is not configured/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("mobile-app-download-button"),
    ).toBeNull();
  });

  it("should not offer a continue-in-browser option", () => {
    render(
      <MobileAppPromptPage
        device={{ os: MobileOs.Android, deviceType: DeviceType.Phone }}
      />,
    );

    expect(
      screen.queryByTestId("mobile-app-continue-web-button"),
    ).toBeNull();
    expect(screen.queryByText("Continue in browser")).toBeNull();
  });
});
