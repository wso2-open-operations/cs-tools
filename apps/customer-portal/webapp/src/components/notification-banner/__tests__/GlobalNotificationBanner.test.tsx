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

import { render, screen, waitFor, act } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import GlobalNotificationBanner from "@components/notification-banner/GlobalNotificationBanner";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  NotificationBanner: ({ visible, title, message }: any) =>
    visible ? (
      <div data-testid="notification-banner">
        <h3>{title}</h3>
        <p>{message}</p>
      </div>
    ) : null,
}));

const { mockConfig } = vi.hoisted(() => ({
  mockConfig: {
    actionLabel: "Test Action",
    actionUrl: "https://wso2.com",
    message: "Test Message",
    severity: "info" as const,
    title: "Test Title",
    visible: true,
  },
}));

vi.mock("@/config/notificationBannerConfig", () => ({
  notificationBannerConfig: mockConfig,
}));

describe("GlobalNotificationBanner", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockConfig.visible = true;
  });

  it("should render the banner when visible is true", () => {
    render(<GlobalNotificationBanner visible={true} />);

    expect(screen.getByTestId("notification-banner")).toBeInTheDocument();
    expect(screen.getByText("Test Title")).toBeInTheDocument();
    expect(screen.getByText("Test Message")).toBeInTheDocument();
  });

  it("should NOT render the banner when visible is false", () => {
    render(<GlobalNotificationBanner visible={false} />);

    expect(screen.queryByTestId("notification-banner")).toBeNull();
  });

  it("should synchronize visibility when configuration changes after mount", async () => {
    const { rerender } = render(<GlobalNotificationBanner visible={true} />);
    expect(screen.getByTestId("notification-banner")).toBeInTheDocument();

    // Change the visibility prop and rerender inside act
    await act(async () => {
      rerender(<GlobalNotificationBanner visible={false} />);
    });

    // Wait for the element to disappear
    await waitFor(() => {
      expect(screen.queryByTestId("notification-banner")).toBeNull();
    });
  });
});
