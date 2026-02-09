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

import React from "react";
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { RichTextEditor } from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/RichTextEditor";

const mockTheme = {
  palette: {
    primary: { main: "#fa7b3f" },
    error: { main: "#d32f2f" },
    success: { main: "#388e3c" },
    warning: { main: "#f9a825" },
    info: { main: "#1976d2" },
    text: { primary: "#40404B", secondary: "#40404B" },
    action: { selected: "rgba(0,0,0,0.08)" },
    background: { default: "#f5f5f5", paper: "#ffffffe1" },
    divider: "#00000012",
  },
};

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: React.forwardRef(({ children, ...props }: any, ref: any) => (
    <div ref={ref} data-testid="box" {...props}>
      {children}
    </div>
  )),
  Button: ({ children, onClick }: any) => (
    <button onClick={onClick}>{children}</button>
  ),
  CodeBlock: ({ code, language }: any) => (
    <pre data-language={language}>
      <code>{code}</code>
    </pre>
  ),
  Dialog: ({ children, open }: any) =>
    open ? <div role="dialog">{children}</div> : null,
  DialogActions: ({ children }: any) => (
    <div data-testid="dialog-actions">{children}</div>
  ),
  DialogContent: ({ children }: any) => (
    <div data-testid="dialog-content">{children}</div>
  ),
  DialogTitle: ({ children }: any) => (
    <div data-testid="dialog-title">{children}</div>
  ),
  Divider: () => <hr />,
  IconButton: ({ onClick, disabled, "aria-label": ariaLabel }: any) => (
    <button
      onClick={onClick}
      disabled={disabled}
      aria-label={ariaLabel}
      type="button"
    />
  ),
  Menu: ({ children, open }: any) =>
    open ? <div role="menu">{children}</div> : null,
  MenuItem: ({ children, onClick }: any) => (
    <div role="menuitem" onClick={onClick}>
      {children}
    </div>
  ),
  Paper: ({ children, "data-testid": dataTestId }: any) => (
    <div data-testid={dataTestId}>{children}</div>
  ),
  Popover: ({ children, open }: any) =>
    open ? <div role="dialog">{children}</div> : null,
  Select: ({ children, value, onChange }: any) => (
    <select value={value} onChange={onChange}>
      {children}
    </select>
  ),
  TextField: (props: any) => <input {...props} />,
  Tooltip: ({ children }: any) => <span>{children}</span>,
  Typography: ({ children }: any) => <span>{children}</span>,
  useTheme: () => mockTheme,
}));

// Mock useLogger hook
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

// Mock icons (include icons used by supportConstants so the module loads)
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  AlignCenter: () => <span data-icon="align-center" />,
  AlignJustify: () => <span data-icon="align-justify" />,
  AlignLeft: () => <span data-icon="align-left" />,
  AlignRight: () => <span data-icon="align-right" />,
  Bold: () => <span data-icon="bold" />,
  Bot: () => <span data-icon="bot" />,
  ChevronDown: () => <span data-icon="chevron-down" />,
  CircleCheck: () => <span data-icon="circle-check" />,
  Clock: () => <span data-icon="clock" />,
  Code: () => <span data-icon="code" />,
  FileCode: () => <span data-icon="filecode" />,
  FileText: () => <span data-icon="file-text" />,
  Heading1: () => <span data-icon="heading1" />,
  Heading2: () => <span data-icon="heading2" />,
  Heading3: () => <span data-icon="heading3" />,
  Image: () => <span data-icon="image" />,
  IndentDecrease: () => <span data-icon="indent-decrease" />,
  IndentIncrease: () => <span data-icon="indent-increase" />,
  Italic: () => <span data-icon="italic" />,
  Link: () => <span data-icon="link" />,
  List: () => <span data-icon="list" />,
  ListOrdered: () => <span data-icon="list-ordered" />,
  MessageSquare: () => <span data-icon="message-square" />,
  Paperclip: () => <span data-icon="paperclip" />,
  Redo2: () => <span data-icon="redo" />,
  Strikethrough: () => <span data-icon="strikethrough" />,
  TrendingUp: () => <span data-icon="trending-up" />,
  Type: () => <span data-icon="type" />,
  Undo2: () => <span data-icon="undo" />,
  Underline: () => <span data-icon="underline" />,
  X: () => <span data-icon="x" />,
}));

describe("RichTextEditor", () => {
  it("should render with value and data-testid", () => {
    render(
      <RichTextEditor
        value="<p>Initial content</p>"
        onChange={vi.fn()}
        data-testid="custom-editor"
      />,
    );

    const wrapper = screen.getByTestId("custom-editor");
    expect(wrapper).toBeInTheDocument();

    // Editor area is contentEditable; find by checking for placeholder or content
    const editable = wrapper.querySelector("[contenteditable=true]");
    expect(editable).toBeInTheDocument();
    expect(editable).toHaveAttribute("data-placeholder");
  });

  it("should render toolbar buttons", () => {
    render(
      <RichTextEditor
        value=""
        onChange={vi.fn()}
        data-testid="rich-text-editor"
      />,
    );

    expect(screen.getByRole("button", { name: /undo/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /redo/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /bold/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /italic/i })).toBeInTheDocument();
  });

  it("should call onChange when content changes", () => {
    const onChange = vi.fn();
    render(
      <RichTextEditor
        value=""
        onChange={onChange}
        data-testid="rich-text-editor"
      />,
    );

    const wrapper = screen.getByTestId("rich-text-editor");
    const editable = wrapper.querySelector("[contenteditable=true]");
    expect(editable).toBeInTheDocument();

    if (editable) {
      (editable as HTMLElement).innerHTML = "<p>New text</p>";
      fireEvent.input(editable);
      expect(onChange).toHaveBeenCalledWith("<p>New text</p>");
    }
  });

  it("should be disabled when disabled prop is true", () => {
    render(
      <RichTextEditor
        value=""
        onChange={vi.fn()}
        disabled
        data-testid="rich-text-editor"
      />,
    );

    const wrapper = screen.getByTestId("rich-text-editor");
    const editable = wrapper.querySelector("[contenteditable]");
    expect(editable).toHaveAttribute("contenteditable", "false");
  });

  it("should block unsafe URL schemes in handleInsertLink", async () => {
    render(
      <RichTextEditor
        value=""
        onChange={vi.fn()}
        data-testid="rich-text-editor"
      />,
    );

    const linkButton = screen.getByRole("button", {
      name: /insert or edit link/i,
    });
    fireEvent.click(linkButton);

    const urlInput = screen.getByPlaceholderText("https://...");
    const applyButton = screen.getByRole("button", { name: /apply/i });

    // Try javascript: scheme
    fireEvent.change(urlInput, {
      target: { value: "javascript:alert('XSS')" },
    });
    fireEvent.click(applyButton);

    const editable = screen
      .getByTestId("rich-text-editor")
      .querySelector("[contenteditable=true]");
    expect(editable?.innerHTML).not.toContain("javascript:alert");
  });

  it("should allow safe URL schemes in handleInsertLink", async () => {
    render(
      <RichTextEditor
        value=""
        onChange={vi.fn()}
        data-testid="rich-text-editor"
      />,
    );

    const linkButton = screen.getByRole("button", {
      name: /insert or edit link/i,
    });
    fireEvent.click(linkButton);

    const urlInput = screen.getByPlaceholderText("https://...");
    const applyButton = screen.getByRole("button", { name: /apply/i });

    // Try https: scheme
    fireEvent.change(urlInput, { target: { value: "https://wso2.com" } });
    fireEvent.click(applyButton);

    const editable = screen
      .getByTestId("rich-text-editor")
      .querySelector("[contenteditable=true]");
    expect(editable?.innerHTML).toContain('href="https://wso2.com"');
  });
});
