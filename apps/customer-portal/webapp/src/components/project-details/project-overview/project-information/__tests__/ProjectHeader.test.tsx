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
import ProjectHeader from "@components/project-details/project-overview/project-information/ProjectHeader";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  colors: {
    blue: { 700: "#1D4ED8" },
    purple: { 400: "#A78BFA" },
  },
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Building: () => <svg data-testid="icon-building" />,
}));

describe("ProjectHeader", () => {
  it("should render the header title and icon", () => {
    render(<ProjectHeader />);

    expect(screen.getByText("Project Information")).toBeInTheDocument();
    expect(screen.getByTestId("icon-building")).toBeInTheDocument();
  });
});
