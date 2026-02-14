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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import AttachmentListItem from "@case-details-attachments/AttachmentListItem";
import type { CaseAttachment } from "@models/responses";

const mockAttachment: CaseAttachment = {
  id: "att-1",
  name: "screenshot.png",
  type: "image/png",
  size: 102400,
  sizeBytes: "102400",
  createdBy: "user@example.com",
  createdOn: "2026-02-13 10:00:00",
  downloadUrl: "https://example.com/download/att-1",
};

function renderItem(att = mockAttachment, onDownload = vi.fn()) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <AttachmentListItem attachment={att} onDownload={onDownload} />
    </ThemeProvider>,
  );
}

describe("AttachmentListItem", () => {
  it("should render attachment name and meta", () => {
    renderItem();
    expect(screen.getByText("screenshot.png")).toBeInTheDocument();
    expect(screen.getByText(/100 KB/)).toBeInTheDocument();
    expect(screen.getByText(/Uploaded by user@example.com/)).toBeInTheDocument();
    expect(screen.getByText("2026-02-13 10:00:00")).toBeInTheDocument();
  });

  it("should call onDownload when Download button is clicked", () => {
    const onDownload = vi.fn();
    renderItem(mockAttachment, onDownload);
    fireEvent.click(screen.getByRole("button", { name: /download/i }));
    expect(onDownload).toHaveBeenCalledWith(mockAttachment);
  });
});
