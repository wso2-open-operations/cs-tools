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
import ProjectCardStats from "@/components/projectHub/projectCard/ProjectCardStats";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Divider: () => <hr />,
  Form: {
    CardContent: ({ children }: any) => <div>{children}</div>,
  },
  Typography: ({ children }: any) => <span>{children}</span>,
  colors: {
    blue: { 500: "#2196f3" },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Calendar: () => <svg data-testid="calendar-icon" />,
  CircleAlert: () => <svg data-testid="alert-icon" />,
  MessageSquare: () => <svg data-testid="message-icon" />,
}));

// Mock utils
vi.mock("@/utils/projectCard", () => ({
  formatProjectDate: vi.fn((date) => `Formatted ${date}`),
}));

describe("ProjectCardStats", () => {
  it("should render counts and formatted date", () => {
    const props = {
      openCases: 10,
      activeChats: 5,
      date: "2025-07-17",
    };

    render(<ProjectCardStats {...props} />);

    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText(`Formatted ${props.date}`)).toBeInTheDocument();
  });

  it("should render icons", () => {
    const props = {
      openCases: 0,
      activeChats: 0,
      date: "2025-07-17",
    };

    render(<ProjectCardStats {...props} />);

    expect(screen.getByTestId("calendar-icon")).toBeInTheDocument();
    expect(screen.getByTestId("alert-icon")).toBeInTheDocument();
    expect(screen.getByTestId("message-icon")).toBeInTheDocument();
  });
});
