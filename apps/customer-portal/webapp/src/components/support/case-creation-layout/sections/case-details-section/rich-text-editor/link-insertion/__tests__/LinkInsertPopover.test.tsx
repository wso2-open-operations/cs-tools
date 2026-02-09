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
import { LinkInsertPopover } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/link-insertion/LinkInsertPopover";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Button: ({ children, onClick, disabled }: any) => (
    <button onClick={onClick} disabled={disabled}>
      {children}
    </button>
  ),
  Popover: ({ children, open, onClose }: any) =>
    open ? (
      <div data-testid="popover">
        <button onClick={onClose} data-testid="popover-close">
          Close
        </button>
        {children}
      </div>
    ) : null,
  TextField: ({ value, onChange, placeholder }: any) => (
    <input
      data-testid={`input-${placeholder}`}
      value={value}
      onChange={onChange}
      placeholder={placeholder}
    />
  ),
  Typography: ({ children }: any) => <span>{children}</span>,
}));

describe("LinkInsertPopover", () => {
  const defaultProps = {
    open: true,
    anchor: document.createElement("div"),
    onInsert: vi.fn(),
    onClose: vi.fn(),
  };

  it("should not render when open is false", () => {
    render(<LinkInsertPopover {...defaultProps} open={false} />);
    expect(screen.queryByTestId("popover")).not.toBeInTheDocument();
  });

  it("should render correctly with default values", () => {
    render(
      <LinkInsertPopover
        {...defaultProps}
        defaultUrl="https://example.com"
        defaultText="Example"
      />,
    );
    expect(screen.getByTestId("popover")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Link text")).toHaveValue("Example");
    expect(screen.getByPlaceholderText("https://...")).toHaveValue(
      "https://example.com",
    );
  });

  it("should call onInsert with URL and text when Apply is clicked", () => {
    const onInsert = vi.fn();
    render(<LinkInsertPopover {...defaultProps} onInsert={onInsert} />);

    fireEvent.change(screen.getByPlaceholderText("Link text"), {
      target: { value: "My Link" },
    });
    fireEvent.change(screen.getByPlaceholderText("https://..."), {
      target: { value: "https://google.com" },
    });

    fireEvent.click(screen.getByText("Apply"));
    expect(onInsert).toHaveBeenCalledWith("https://google.com", "My Link");
  });

  it("should disable Apply button when URL is empty", () => {
    render(<LinkInsertPopover {...defaultProps} />);
    const applyButton = screen.getByText("Apply");
    expect(applyButton).toBeDisabled();
  });

  it("should call onClose when popover closes", () => {
    const onClose = vi.fn();
    render(<LinkInsertPopover {...defaultProps} onClose={onClose} />);
    fireEvent.click(screen.getByTestId("popover-close"));
    expect(onClose).toHaveBeenCalled();
  });
});
