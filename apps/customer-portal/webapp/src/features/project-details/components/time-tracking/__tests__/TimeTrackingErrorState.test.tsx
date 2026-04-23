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
import { describe, it, expect, vi } from "vitest";
import TimeTrackingErrorState from "@time-tracking/TimeTrackingErrorState";

// Mock @components/error/Error500Page
vi.mock("@components/error/Error500Page", () => ({
  default: () => <div data-testid="error-icon" />,
}));

describe("TimeTrackingErrorState", () => {
  it("should render error icon and error message", () => {
    render(<TimeTrackingErrorState />);

    expect(screen.getByTestId("error-icon")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Error loading time tracking details. Please try again later.",
      ),
    ).toBeInTheDocument();
  });
});
