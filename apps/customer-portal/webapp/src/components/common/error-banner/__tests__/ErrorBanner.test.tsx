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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ErrorBanner from "@components/common/error-banner/ErrorBanner";

vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  IconButton: ({ children, onClick, "aria-label": ariaLabel }: any) => (
    <button data-testid="close-button" onClick={onClick} aria-label={ariaLabel}>
      {children}
    </button>
  ),
  LinearProgress: ({ value }: any) => (
    <div data-testid="linear-progress" data-value={value} />
  ),
  Paper: ({ children }: any) => (
    <div data-testid="error-banner" role="alert">
      {children}
    </div>
  ),
  Stack: ({ children }: any) => <div data-testid="stack">{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
}));

vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  X: () => <span data-testid="x-icon" />,
}));

describe("ErrorBanner", () => {
  it("should render api name and progress", () => {
    const onClose = vi.fn();
    render(
      <ErrorBanner
        apiName="dashboard statistics"
        progress={75}
        onClose={onClose}
      />,
    );

    expect(screen.getByTestId("error-banner")).toBeInTheDocument();
    expect(
      screen.getByText("Error loading dashboard statistics"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("linear-progress")).toHaveAttribute(
      "data-value",
      "75",
    );
  });

  it("should call onClose when close button is clicked", () => {
    const onClose = vi.fn();
    render(<ErrorBanner apiName="projects" progress={50} onClose={onClose} />);

    fireEvent.click(screen.getByTestId("close-button"));

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
