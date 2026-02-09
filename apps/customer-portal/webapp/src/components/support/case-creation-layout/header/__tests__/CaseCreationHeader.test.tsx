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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CaseCreationHeader } from "@components/support/case-creation-layout/header/CaseCreationHeader";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Button: ({ children, onClick, startIcon }: any) => (
    <button onClick={onClick}>
      {startIcon}
      {children}
    </button>
  ),
  Chip: ({ label, icon }: any) => (
    <div data-testid="chip">
      {icon}
      {label}
    </div>
  ),
  Typography: ({ children }: any) => <span>{children}</span>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  ArrowLeft: () => <svg data-testid="icon-arrow-left" />,
  Sparkles: () => <svg data-testid="icon-sparkles" />,
}));

describe("CaseCreationHeader", () => {
  it("should render titles and description", () => {
    const onBack = vi.fn();
    render(<CaseCreationHeader onBack={onBack} />);

    expect(screen.getByText("Review Case Details")).toBeInTheDocument();
    expect(
      screen.getByText(/Please review and edit the auto-populated information/),
    ).toBeInTheDocument();
    expect(screen.getByText("AI Generated")).toBeInTheDocument();
    expect(screen.getByTestId("icon-sparkles")).toBeInTheDocument();
  });

  it("should call onBack when back button is clicked", () => {
    const onBack = vi.fn();
    render(<CaseCreationHeader onBack={onBack} />);

    const backButton = screen.getByText("Back to Chat");
    fireEvent.click(backButton);
    expect(onBack).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("icon-arrow-left")).toBeInTheDocument();
  });
});
