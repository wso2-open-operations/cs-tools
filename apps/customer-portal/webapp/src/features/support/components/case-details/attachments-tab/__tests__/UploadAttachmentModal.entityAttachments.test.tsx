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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import UploadAttachmentModal from "@case-details-attachments/UploadAttachmentModal";
import { UploadConfirmPendingError } from "@api/storedAttachments";

const uploadMutate = vi.fn();
const confirmMutate = vi.fn();

vi.mock("@features/support/api/usePostAttachments", () => ({
  usePostAttachments: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@features/project-details/api/usePostDeploymentAttachment", () => ({
  usePostDeploymentAttachment: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@api/useStoredAttachments", () => ({
  useStoredAttachmentUpload: () => ({
    upload: { mutate: uploadMutate, isPending: false },
    confirm: { mutate: confirmMutate, isPending: false },
  }),
}));

function renderModal(props: { caseId?: string; deploymentId?: string } = {}) {
  const onClose = vi.fn();
  render(
    <ThemeProvider theme={createTheme()}>
      <UploadAttachmentModal open onClose={onClose} {...props} />
    </ThemeProvider>,
  );
  // The dialog renders in a portal, outside the render container.
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  const pick = (file: File) =>
    fireEvent.change(input, { target: { files: [file] } });
  return { onClose, pick };
}

describe("UploadAttachmentModal with entity-service attachments on", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (window as unknown as { config: unknown }).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
      CUSTOMER_PORTAL_ENTITY_ATTACHMENTS_ENABLED: true,
    };
  });

  it("uploads a case attachment as a work_item through the stored flow", () => {
    const { pick } = renderModal({ caseId: "case-1" });
    const file = new File(["hello"], "notes.txt", { type: "text/plain" });
    pick(file);
    fireEvent.click(screen.getByRole("button", { name: /^upload$/i }));
    expect(uploadMutate).toHaveBeenCalledTimes(1);
    const [vars] = uploadMutate.mock.calls[0];
    expect(vars).toMatchObject({
      type: "work_item",
      parentId: "case-1",
      caseId: "case-1",
      file,
      body: { name: "notes.txt" },
    });
  });

  it("uploads a deployment document as a deployment", () => {
    const { pick } = renderModal({ deploymentId: "dep-1" });
    pick(new File(["x"], "guide.pdf", { type: "application/pdf" }));
    fireEvent.click(screen.getByRole("button", { name: /^upload$/i }));
    expect(uploadMutate.mock.calls[0][0]).toMatchObject({
      type: "deployment",
      parentId: "dep-1",
      deploymentId: "dep-1",
    });
  });

  it("allows files over 10 MB up to 100 MB", () => {
    const { pick } = renderModal({ caseId: "case-1" });
    const big = new File(["x"], "big.bin");
    Object.defineProperty(big, "size", { value: 50 * 1024 * 1024 });
    pick(big);
    expect(screen.getByRole("button", { name: /^upload$/i })).toBeEnabled();
    const tooBig = new File(["x"], "huge.bin");
    Object.defineProperty(tooBig, "size", { value: 101 * 1024 * 1024 });
    pick(tooBig);
    expect(screen.getByText(/exceeds 100 MB limit/i)).toBeInTheDocument();
  });

  it("shows the server's message and retries only the confirm step", async () => {
    const { pick } = renderModal({ caseId: "case-1" });
    pick(new File(["hello"], "notes.txt", { type: "text/plain" }));
    fireEvent.click(screen.getByRole("button", { name: /^upload$/i }));
    const options = uploadMutate.mock.calls[0][1];
    options.onError(
      new UploadConfirmPendingError(
        "work_item",
        "att-9",
        "The service is temporarily unavailable.",
      ),
    );
    expect(
      await screen.findByText("The service is temporarily unavailable."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^retry$/i }));
    await waitFor(() => expect(confirmMutate).toHaveBeenCalledTimes(1));
    expect(confirmMutate.mock.calls[0][0]).toMatchObject({
      type: "work_item",
      attachmentId: "att-9",
      caseId: "case-1",
    });
    expect(uploadMutate).toHaveBeenCalledTimes(1);
  });
});
