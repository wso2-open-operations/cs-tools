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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { UseQueryResult } from "@tanstack/react-query";
import type { DeploymentAttachment } from "@features/csm-projects/types/csmProjects";

const useSearchDeploymentAttachmentsMock = vi.fn();
const createMutateAsync = vi.fn();
const updateMutate = vi.fn();
const deleteMutate = vi.fn();
const downloadMock = vi.fn();

vi.mock("@features/csm-projects/api/useSearchDeploymentAttachments", () => ({
  useSearchDeploymentAttachments: () => useSearchDeploymentAttachmentsMock(),
}));
vi.mock("@features/csm-projects/api/useCreateDeploymentAttachment", () => ({
  useCreateDeploymentAttachment: () => ({
    mutateAsync: createMutateAsync,
    isPending: false,
  }),
}));
vi.mock("@features/csm-projects/api/useUpdateDeploymentAttachment", () => ({
  useUpdateDeploymentAttachment: () => ({ mutate: updateMutate, isPending: false }),
}));
vi.mock("@features/csm-projects/api/useDeleteDeploymentAttachment", () => ({
  useDeleteDeploymentAttachment: () => ({ mutate: deleteMutate, isPending: false }),
}));
vi.mock("@features/csm-projects/api/useDownloadDeploymentAttachment", () => ({
  useDownloadDeploymentAttachment: () => downloadMock,
}));
// `AttachmentsField` pulls `MAX_ATTACHMENT_SIZE_BYTES` from the case
// attachments module, which reads runtime config (`CSM_PORTAL_BACKEND_BASE_URL`)
// at module load via `@api/backend/client` — not present under vitest. Stub
// the module so the import chain doesn't throw (same approach as
// ProjectContactsTab.test.tsx / CsmChangeRequestDetailPage.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({}),
}));

import DeploymentAttachmentsPanel from "@features/csm-projects/components/DeploymentAttachmentsPanel";

const ATTACHMENT: DeploymentAttachment = {
  id: "att-1",
  name: "runbook.pdf",
  contentType: "application/pdf",
  sizeBytes: 2048,
  description: "Deployment runbook",
  uploadedBy: "Jane Doe",
  uploadedOn: "2026-01-01T00:00:00Z",
  downloadUrl: null,
};

function mockList(overrides: Partial<UseQueryResult<DeploymentAttachment[], Error>>): void {
  useSearchDeploymentAttachmentsMock.mockReturnValue({
    data: [],
    isLoading: false,
    isError: false,
    error: null,
    ...overrides,
  });
}

describe("DeploymentAttachmentsPanel", () => {
  it("shows an empty state when there are no attachments", () => {
    mockList({ data: [] });
    render(<DeploymentAttachmentsPanel deploymentId="dep-1" />);
    expect(screen.getByText(/no attachments on this deployment/i)).toBeInTheDocument();
  });

  it("lists existing attachments with size, uploader, and description", () => {
    mockList({ data: [ATTACHMENT] });
    render(<DeploymentAttachmentsPanel deploymentId="dep-1" />);
    expect(screen.getByText("runbook.pdf")).toBeInTheDocument();
    expect(screen.getByText("Deployment runbook")).toBeInTheDocument();
    expect(screen.getByText(/jane doe/i)).toBeInTheDocument();
  });

  it("opens the edit dialog and saves a changed name", async () => {
    mockList({ data: [ATTACHMENT] });
    render(<DeploymentAttachmentsPanel deploymentId="dep-1" />);

    fireEvent.click(screen.getByRole("button", { name: /edit runbook\.pdf/i }));
    const nameField = screen.getByLabelText(/^name$/i);
    fireEvent.change(nameField, { target: { value: "runbook-v2.pdf" } });
    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));

    expect(updateMutate).toHaveBeenCalledWith(
      { deploymentId: "dep-1", attachmentId: "att-1", name: "runbook-v2.pdf" },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    );
  });

  it("confirms and deletes an attachment", () => {
    mockList({ data: [ATTACHMENT] });
    render(<DeploymentAttachmentsPanel deploymentId="dep-1" />);

    fireEvent.click(screen.getByRole("button", { name: /delete runbook\.pdf/i }));
    fireEvent.click(screen.getByRole("button", { name: /^delete$/i }));

    expect(deleteMutate).toHaveBeenCalledWith(
      { deploymentId: "dep-1", attachmentId: "att-1" },
      expect.objectContaining({ onSuccess: expect.any(Function), onError: expect.any(Function) }),
    );
  });

  it("downloads an attachment", async () => {
    downloadMock.mockResolvedValue(undefined);
    mockList({ data: [ATTACHMENT] });
    render(<DeploymentAttachmentsPanel deploymentId="dep-1" />);

    fireEvent.click(screen.getByRole("button", { name: /download runbook\.pdf/i }));

    await waitFor(() => expect(downloadMock).toHaveBeenCalledWith(ATTACHMENT));
  });

  it("shows a load error via QueryErrorState", () => {
    mockList({ data: undefined, isError: true, error: new Error("network down") });
    render(<DeploymentAttachmentsPanel deploymentId="dep-1" />);
    expect(screen.getByText(/network down/i)).toBeInTheDocument();
  });
});
