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
import { EditorContentArea } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/editor/EditorContentArea";
import { createRef } from "react";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  const React = await import("react");
  return {
    ...actual,
    Box: React.forwardRef(
      (
        {
          children,
          onInput,
          onKeyDown,
          onPaste,
          onClick,
          contentEditable,
          "data-placeholder": placeholder,
        }: any,
        ref: any,
      ) => (
        <div
          ref={ref}
          data-testid="editor-area"
          contentEditable={contentEditable}
          onInput={onInput}
          onKeyDown={onKeyDown}
          onPaste={onPaste}
          onClick={onClick}
          data-placeholder={placeholder}
        >
          {children}
        </div>
      ),
    ),
  };
});

describe("EditorContentArea", () => {
  const defaultProps = {
    editorRef: createRef<HTMLDivElement>(),
    placeholder: "Type here...",
    disabled: false,
    minHeight: 100,
    onInput: vi.fn(),
    onKeyDown: vi.fn(),
    onPaste: vi.fn(),
    onClick: vi.fn(),
  };

  it("should render correctly with placeholder", () => {
    render(<EditorContentArea {...defaultProps} />);
    const editor = screen.getByTestId("editor-area");
    expect(editor).toBeInTheDocument();
    expect(editor).toHaveAttribute("data-placeholder", "Type here...");
    expect(editor).toHaveAttribute("contentEditable", "true");
  });

  it("should respect disabled prop", () => {
    render(<EditorContentArea {...defaultProps} disabled={true} />);
    const editor = screen.getByTestId("editor-area");
    expect(editor).toHaveAttribute("contentEditable", "false");
  });

  it("should call onInput when content changes", () => {
    render(<EditorContentArea {...defaultProps} />);
    const editor = screen.getByTestId("editor-area");
    fireEvent.input(editor);
    expect(defaultProps.onInput).toHaveBeenCalled();
  });

  it("should call onKeyDown when key is pressed", () => {
    render(<EditorContentArea {...defaultProps} />);
    const editor = screen.getByTestId("editor-area");
    fireEvent.keyDown(editor, { key: "Enter" });
    expect(defaultProps.onKeyDown).toHaveBeenCalled();
  });

  it("should call onPaste when content is pasted", () => {
    render(<EditorContentArea {...defaultProps} />);
    const editor = screen.getByTestId("editor-area");
    fireEvent.paste(editor);
    expect(defaultProps.onPaste).toHaveBeenCalled();
  });

  it("should call onClick when clicked", () => {
    render(<EditorContentArea {...defaultProps} />);
    const editor = screen.getByTestId("editor-area");
    fireEvent.click(editor);
    expect(defaultProps.onClick).toHaveBeenCalled();
  });
});
