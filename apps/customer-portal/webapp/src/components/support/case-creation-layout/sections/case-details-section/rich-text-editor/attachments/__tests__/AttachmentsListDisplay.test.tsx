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
import {
  AttachmentsListDisplay,
  type AttachmentItem,
} from "@components/support/case-creation-layout/sections/case-details-section/rich-text-editor/attachments/AttachmentsListDisplay";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, onClick, component, src, alt }: any) => {
    if (component === "img") {
      return <img src={src} alt={alt} onClick={onClick} />;
    }
    return <div onClick={onClick}>{children}</div>;
  },
  IconButton: ({ children, onClick }: any) => (
    <button onClick={onClick}>{children}</button>
  ),
  Paper: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  FileText: () => <svg data-testid="icon-file" />,
  Image: () => <svg data-testid="icon-image" />,
  X: () => <svg data-testid="icon-x" />,
}));

describe("AttachmentsListDisplay", () => {
  const mockAttachments: AttachmentItem[] = [
    { id: "1", name: "test-file.txt", type: "file" },
    {
      id: "2",
      name: "test-image.png",
      type: "image",
      dataUrl: "data:image/png;base64,...",
    },
  ];

  it("should render nothing when no attachments provided", () => {
    const { container } = render(
      <AttachmentsListDisplay attachments={[]} onRemove={vi.fn()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("should render attachment list correctly", () => {
    render(
      <AttachmentsListDisplay
        attachments={mockAttachments}
        onRemove={vi.fn()}
      />,
    );

    expect(screen.getByText("test-file.txt")).toBeInTheDocument();
    expect(screen.getByText("test-image.png")).toBeInTheDocument();
    expect(screen.getByTestId("icon-file")).toBeInTheDocument();
    expect(screen.getByAltText("test-image.png")).toBeInTheDocument();
  });

  it("should call onRemove when delete button is clicked", () => {
    const onRemove = vi.fn();
    render(
      <AttachmentsListDisplay
        attachments={mockAttachments}
        onRemove={onRemove}
      />,
    );

    const removeButtons = screen.getAllByRole("button");
    fireEvent.click(removeButtons[0]);

    expect(onRemove).toHaveBeenCalledWith("1");
  });
});
