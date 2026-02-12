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
  Alert: ({ children, onClose, severity }: any) => (
    <div data-testid="error-banner" role="alert" data-severity={severity}>
      {children}
      <button data-testid="close-button" onClick={onClose}>
        Close
      </button>
    </div>
  ),
  Box: ({ children, sx }: any) => (
    <div data-testid="box" style={sx}>
      {children}
    </div>
  ),
}));

vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  X: () => <span data-testid="x-icon" />,
}));

describe("ErrorBanner", () => {
  it("should render message from caller", () => {
    const onClose = vi.fn();
    render(
      <ErrorBanner
        message="Could not load dashboard statistics."
        onClose={onClose}
      />,
    );

    expect(screen.getByTestId("error-banner")).toBeInTheDocument();
    expect(
      screen.getByText("Could not load dashboard statistics."),
    ).toBeInTheDocument();
  });

  it("should call onClose when close button is clicked", () => {
    const onClose = vi.fn();
    render(<ErrorBanner message="Something went wrong." onClose={onClose} />);

    fireEvent.click(screen.getByTestId("close-button"));

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
