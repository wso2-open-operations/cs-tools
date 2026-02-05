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
import { describe, expect, it, vi } from "vitest";
import SubscriptionWidget from "@/components/common/sideNavBar/SubscriptionWidget";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: { children: any }) => <div>{children}</div>,
  Button: ({ children }: { children: any }) => <button>{children}</button>,
  Paper: ({ children }: { children: any }) => <div>{children}</div>,
  Typography: ({ children }: { children: any }) => <span>{children}</span>,
  colors: {
    blue: { 700: "#1d4ed8" },
    purple: { 400: "#a78bfa" },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Crown: () => <svg data-testid="icon-Crown" />,
  Info: () => <svg data-testid="icon-Info" />,
  Server: () => <svg data-testid="icon-Server" />,
  Clock: () => <svg data-testid="icon-Clock" />,
  Zap: () => <svg data-testid="icon-Zap" />,
}));

describe("SubscriptionWidget", () => {
  it("should render the widget when not collapsed", () => {
    render(<SubscriptionWidget collapsed={false} />);

    expect(screen.getByText("Subscription")).toBeInTheDocument();
    expect(screen.getByText("Information about")).toBeInTheDocument();
    expect(screen.getByText("subscription")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /view details/i }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("icon-Crown")).toBeInTheDocument();
  });

  it("should return null when collapsed", () => {
    const { container } = render(<SubscriptionWidget collapsed={true} />);
    expect(container.firstChild).toBeNull();
  });
});
