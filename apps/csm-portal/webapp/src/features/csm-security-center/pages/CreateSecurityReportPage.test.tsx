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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";

const navigateMock = vi.fn();
const postCaseMutateAsyncMock = vi.fn();
const showErrorMock = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
  useLocation: () => ({ state: undefined }),
  useSearchParams: () => [new URLSearchParams()],
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-cases/api/usePostCsmCase", () => ({
  usePostCsmCase: () => ({ mutateAsync: postCaseMutateAsyncMock }),
}));
// The project/deployment/product cascade is exercised elsewhere
// (CsmCaseCreatePage) — stub them here to fixed, already-loaded options so
// this file can focus on the attachment-required submit path.
vi.mock("@features/csm-cases/api/useSearchDeployments", () => ({
  useSearchDeployments: () => ({
    data: [{ id: "dep-1", name: "Production" }],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-cases/api/useDeployedProductOptions", () => ({
  useDeployedProductOptions: () => ({
    data: [{ id: "dp-1", label: "API Manager 4.3.0" }],
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
vi.mock("@features/csm-cases/components/ProjectSelectionField", () => ({
  default: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (next: string) => void;
  }) => (
    <input
      aria-label="Project"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea aria-label="editor" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));
// CreateSecurityReportPage imports BackendApiError from the real API client
// module, which reads window.config at module load and throws outside a
// configured runtime. Mock it with a real class (so `instanceof` still
// works), mirroring CreateChangeRequestPage.test.tsx.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

// Imported after the mocks above so the module picks them up.
import CreateSecurityReportPage from "@features/csm-security-center/pages/CreateSecurityReportPage";

/** A minimal File the jsdom FileReader used by AttachmentsField can read. */
function makeFile(name: string, content = "scan results"): File {
  return new File([content], name, { type: "text/plain" });
}

/** Fills every field the form requires except attachments. */
function fillRequiredFieldsExceptAttachments(): void {
  fireEvent.change(screen.getByLabelText("Project"), { target: { value: "proj-1" } });
  fireEvent.mouseDown(screen.getByLabelText(/deployment/i));
  fireEvent.click(screen.getByRole("option", { name: "Production" }));
  fireEvent.mouseDown(screen.getByLabelText(/deployed product/i));
  fireEvent.click(screen.getByRole("option", { name: "API Manager 4.3.0" }));
  fireEvent.change(screen.getByLabelText(/subject/i), {
    target: { value: "Suspicious access pattern" },
  });
  fireEvent.change(screen.getByLabelText("editor"), {
    target: { value: "<p>Please review the attached scan.</p>" },
  });
}

async function addAttachment(name = "scan.txt"): Promise<void> {
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  await fireEvent.change(input, { target: { files: [makeFile(name)] } });
  // Reading the file to base64 is async; let the microtask settle.
  await screen.findByText(name);
}

describe("CreateSecurityReportPage — attachment required at create time", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postCaseMutateAsyncMock.mockReset();
    showErrorMock.mockReset();
  });

  it("keeps Create disabled until every required field, including an attachment, is filled", () => {
    render(<CreateSecurityReportPage />);
    const submit = screen.getByRole("button", { name: /create security report/i });
    expect(submit).toBeDisabled();

    fillRequiredFieldsExceptAttachments();
    // Every other required field is now filled, but there is still no attachment.
    expect(submit).toBeDisabled();
  });

  it("shows the Attachments field as required, not optional", () => {
    render(<CreateSecurityReportPage />);
    expect(screen.getByText(/attachments \*/i)).toBeInTheDocument();
    expect(screen.getByText(/at least one required/i)).toBeInTheDocument();
  });

  it("enables Create once an attachment is added, and sends it inside the POST /cases payload", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "sra-1" });
    render(<CreateSecurityReportPage />);

    fillRequiredFieldsExceptAttachments();
    await addAttachment("scan.txt");

    const submit = screen.getByRole("button", { name: /create security report/i });
    expect(submit).toBeEnabled();

    fireEvent.click(submit);
    await screen.findByRole("button", { name: /creating/i });

    expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1);
    const payload = postCaseMutateAsyncMock.mock.calls[0][0];
    expect(payload.type).toBe("security_report_analysis");
    expect(payload.attachments).toEqual([
      { name: "scan.txt", file: expect.any(String) },
    ]);
    expect(navigateMock).toHaveBeenCalledWith("/security-center/security-reports/sra-1", {
      state: { from: "/security-center" },
    });
  });

  it("surfaces a create-mutation error via the shared error banner instead of navigating", async () => {
    postCaseMutateAsyncMock.mockRejectedValue(new Error("network down"));
    render(<CreateSecurityReportPage />);

    fillRequiredFieldsExceptAttachments();
    await addAttachment("scan.txt");
    fireEvent.click(screen.getByRole("button", { name: /create security report/i }));

    await vi.waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(
        "Could not create the security report. Please try again.",
        expect.any(Error),
      );
    });
    expect(navigateMock).not.toHaveBeenCalled();
  });
});
