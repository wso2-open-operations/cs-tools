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
import SuccessBanner from "@components/common/success-banner/SuccessBanner";

vi.mock("@wso2/oxygen-ui", () => ({
  Alert: ({ children, onClose, severity }: any) => (
    <div data-testid="success-banner" role="alert" data-severity={severity}>
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

describe("SuccessBanner", () => {
  it("should render message", () => {
    const onClose = vi.fn();
    render(
      <SuccessBanner
        message="Case created successfully"
        onClose={onClose}
      />,
    );

    expect(screen.getByTestId("success-banner")).toBeInTheDocument();
    expect(screen.getByText("Case created successfully")).toBeInTheDocument();
  });

  it("should call onClose when close button is clicked", () => {
    const onClose = vi.fn();
    render(
      <SuccessBanner message="Saved" onClose={onClose} />,
    );

    fireEvent.click(screen.getByTestId("close-button"));

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
