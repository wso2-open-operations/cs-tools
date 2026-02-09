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
import { MarkdownEditorDialog } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/markdown-editor/MarkdownEditorDialog";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Button: ({ children, onClick }: any) => (
    <button onClick={onClick}>{children}</button>
  ),
  Dialog: ({ children, open }: any) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogActions: ({ children }: any) => <div>{children}</div>,
  DialogContent: ({ children }: any) => <div>{children}</div>,
  DialogTitle: ({ children }: any) => <div>{children}</div>,
  IconButton: ({ children, onClick }: any) => (
    <button onClick={onClick}>{children}</button>
  ),
  TextField: ({ value, onChange, placeholder }: any) => (
    <textarea
      data-testid="markdown-input"
      value={value}
      onChange={onChange}
      placeholder={placeholder}
    />
  ),
  Typography: ({ children }: any) => <span>{children}</span>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  X: () => <svg data-testid="icon-x" />,
}));

describe("MarkdownEditorDialog", () => {
  const defaultProps = {
    open: true,
    content: "## Hello World",
    onChange: vi.fn(),
    onApply: vi.fn(),
    onClose: vi.fn(),
  };

  it("should not render when open is false", () => {
    render(<MarkdownEditorDialog {...defaultProps} open={false} />);
    expect(screen.queryByTestId("dialog")).not.toBeInTheDocument();
  });

  it("should render correctly with content", () => {
    render(<MarkdownEditorDialog {...defaultProps} />);
    expect(screen.getByTestId("dialog")).toBeInTheDocument();
    expect(screen.getByTestId("markdown-input")).toHaveValue("## Hello World");
  });

  it("should call onApply when Apply button is clicked", () => {
    const onApply = vi.fn();
    render(<MarkdownEditorDialog {...defaultProps} onApply={onApply} />);
    fireEvent.click(screen.getByText("Apply"));
    expect(onApply).toHaveBeenCalled();
  });

  it("should call onClose when Cancel button or close icon is clicked", () => {
    const onClose = vi.fn();
    render(<MarkdownEditorDialog {...defaultProps} onClose={onClose} />);

    // Cancel button
    fireEvent.click(screen.getByText("Cancel"));
    expect(onClose).toHaveBeenCalledTimes(1);

    // Close icon (mocked button)
    const buttons = screen.getAllByRole("button");
    fireEvent.click(buttons[0]); // Header close button
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("should call onChange when content is edited", () => {
    const onChange = vi.fn();
    render(<MarkdownEditorDialog {...defaultProps} onChange={onChange} />);
    fireEvent.change(screen.getByTestId("markdown-input"), {
      target: { value: "New content" },
    });
    expect(onChange).toHaveBeenCalledWith("New content");
  });
});
