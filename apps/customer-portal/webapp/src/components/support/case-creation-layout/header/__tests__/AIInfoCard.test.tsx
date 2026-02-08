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
import { AIInfoCard } from "@components/support/case-creation-layout/header/AIInfoCard";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Card: ({ children }: any) => <div data-testid="card">{children}</div>,
  CardContent: ({ children }: any) => (
    <div data-testid="card-content">{children}</div>
  ),
  colors: {
    orange: { 700: "#C2410C" },
  },
  Typography: ({ children }: any) => <span>{children}</span>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  CircleCheck: () => <svg data-testid="icon-check" />,
}));

describe("AIInfoCard", () => {
  it("should render correctly", () => {
    render(<AIInfoCard />);

    expect(screen.getByTestId("card")).toBeInTheDocument();
    expect(
      screen.getByText("Case details auto-populated from your conversation"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "All fields below have been filled based on your chat with Novera. Please review and edit as needed.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByTestId("icon-check")).toBeInTheDocument();
  });
});
