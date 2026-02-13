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
import { describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";

vi.mock("@api/usePostAttachments", () => ({
  usePostAttachments: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

vi.mock("@providers/MockConfigProvider", () => ({
  useMockConfig: () => ({ isMockEnabled: true }),
}));

function renderModal(props: { open?: boolean; caseId?: string } = {}) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <UploadAttachmentModal
        open={props.open ?? true}
        caseId={props.caseId ?? "case-1"}
        onClose={vi.fn()}
      />
    </ThemeProvider>,
  );
}

describe("UploadAttachmentModal", () => {
  it("should show dialog title and drag-and-drop area when open", () => {
    renderModal();
    expect(screen.getByRole("dialog", { name: /upload attachment/i })).toBeInTheDocument();
    expect(screen.getByText(/drag and drop a file here/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /choose file/i })).toBeInTheDocument();
  });

  it("should have Attachment name field and Upload disabled when mock is enabled", () => {
    renderModal();
    expect(screen.getByLabelText(/attachment name/i)).toBeInTheDocument();
    const uploadBtn = screen.getByRole("button", { name: /^upload$/i });
    expect(uploadBtn).toBeDisabled();
  });

  it("should not render dialog when open is false", () => {
    renderModal({ open: false });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
