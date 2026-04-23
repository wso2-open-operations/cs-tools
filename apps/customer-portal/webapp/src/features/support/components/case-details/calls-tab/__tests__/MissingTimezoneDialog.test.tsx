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
import MissingTimezoneDialog from "@case-details-calls/MissingTimezoneDialog";

const defaultProps = {
  open: true,
  onClose: vi.fn(),
  onSetTimeZone: vi.fn(),
};

describe("MissingTimezoneDialog", () => {
  it("should render the dialog when open", () => {
    render(<MissingTimezoneDialog {...defaultProps} />);
    expect(screen.getByText("Time Zone Not Set")).toBeInTheDocument();
    expect(
      screen.getByText(/Please set your time zone/i),
    ).toBeInTheDocument();
  });

  it("should not render when open is false", () => {
    render(<MissingTimezoneDialog {...defaultProps} open={false} />);
    expect(screen.queryByText("Time Zone Not Set")).not.toBeInTheDocument();
  });

  it("should call onClose when Later is clicked", () => {
    const onClose = vi.fn();
    render(<MissingTimezoneDialog {...defaultProps} onClose={onClose} />);
    fireEvent.click(screen.getByRole("button", { name: /Later/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it("should call onSetTimeZone when Set Time Zone is clicked", () => {
    const onSetTimeZone = vi.fn();
    render(
      <MissingTimezoneDialog {...defaultProps} onSetTimeZone={onSetTimeZone} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Set Time Zone/i }));
    expect(onSetTimeZone).toHaveBeenCalled();
  });

  it("should omit Later and show required copy when variant is required", () => {
    render(<MissingTimezoneDialog {...defaultProps} variant="required" />);
    expect(
      screen.getByText(/Set your time zone first to request or reschedule a call/i),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Later/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Set Time Zone/i })).toBeInTheDocument();
  });
});
