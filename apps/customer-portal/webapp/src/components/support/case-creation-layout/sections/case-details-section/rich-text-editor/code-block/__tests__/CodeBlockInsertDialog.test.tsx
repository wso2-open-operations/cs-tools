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
import { CodeBlockInsertDialog } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/code-block/CodeBlockInsertDialog";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Button: ({ children, onClick, disabled }: any) => (
    <button onClick={onClick} disabled={disabled}>
      {children}
    </button>
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
      data-testid="code-input"
      value={value}
      onChange={onChange}
      placeholder={placeholder}
    />
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  X: () => <svg data-testid="icon-x" />,
}));

describe("CodeBlockInsertDialog", () => {
  it("should not render when open is false", () => {
    render(
      <CodeBlockInsertDialog
        open={false}
        onInsert={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    expect(screen.queryByTestId("dialog")).not.toBeInTheDocument();
  });

  it("should render correctly when open is true", () => {
    render(
      <CodeBlockInsertDialog
        open={true}
        onInsert={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByTestId("dialog")).toBeInTheDocument();
    expect(screen.getByText("Insert Code Block")).toBeInTheDocument();
    expect(screen.getByTestId("code-input")).toBeInTheDocument();
  });

  it("should call onInsert with code when Insert button is clicked", () => {
    const onInsert = vi.fn();
    render(
      <CodeBlockInsertDialog
        open={true}
        onInsert={onInsert}
        onClose={vi.fn()}
      />,
    );

    const input = screen.getByTestId("code-input");
    fireEvent.change(input, { target: { value: "const x = 10;" } });

    const insertButton = screen.getByText("Insert");
    fireEvent.click(insertButton);

    expect(onInsert).toHaveBeenCalledWith("const x = 10;");
  });

  it("should disable Insert button when code is empty", () => {
    render(
      <CodeBlockInsertDialog
        open={true}
        onInsert={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    const insertButton = screen.getByText("Insert");
    expect(insertButton).toBeDisabled();
  });

  it("should call onClose when Cancel button is clicked", () => {
    const onClose = vi.fn();
    render(
      <CodeBlockInsertDialog
        open={true}
        onInsert={vi.fn()}
        onClose={onClose}
      />,
    );

    const cancelButton = screen.getByText("Cancel");
    fireEvent.click(cancelButton);

    expect(onClose).toHaveBeenCalled();
  });
});
