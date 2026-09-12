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
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";

const navigateMock = vi.fn();
const postCaseMutateAsyncMock = vi.fn();
const showErrorMock = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-cases/api/usePostCsmCase", () => ({
  usePostCsmCase: () => ({ mutateAsync: postCaseMutateAsyncMock }),
}));
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea aria-label="editor" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));
// The real multi-select searches the backend as the user types; stub it with
// a plain multi-value control so this file stays focused on the create form's
// own required-field and fan-out-submit behavior (the picker itself has its
// own tests via CsmAnnouncementsPage.test.tsx's filter-bar coverage).
vi.mock("@features/csm-cases/components/AsyncProjectMultiSelect", () => ({
  default: ({
    values,
    onChange,
  }: {
    values: string[];
    onChange: (next: string[]) => void;
  }) => (
    <select
      aria-label="Projects"
      multiple
      value={values}
      onChange={(e) =>
        onChange(Array.from(e.target.selectedOptions).map((o) => o.value))
      }
    >
      <option value="proj-1">Project One</option>
      <option value="proj-2">Project Two</option>
    </select>
  ),
}));
// CsmAnnouncementCreatePage imports BackendApiError from the real API client
// module, which reads window.config at module load and throws outside a
// configured runtime — mirrors CreateSecurityReportPage.test.tsx.
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
import CsmAnnouncementCreatePage from "@features/csm-announcements/pages/CsmAnnouncementCreatePage";

function selectProjects(...values: string[]): void {
  const select = screen.getByLabelText("Projects") as HTMLSelectElement;
  Array.from(select.options).forEach((o) => {
    o.selected = values.includes(o.value);
  });
  fireEvent.change(select);
}

function fillSubjectAndDescription(): void {
  fireEvent.change(screen.getByLabelText(/subject/i), {
    target: { value: "Scheduled maintenance" },
  });
  fireEvent.change(screen.getByLabelText("editor"), {
    target: { value: "<p>Maintenance window details.</p>" },
  });
}

describe("CsmAnnouncementCreatePage", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postCaseMutateAsyncMock.mockReset();
    showErrorMock.mockReset();
  });

  it("keeps Create disabled until title, description, and at least one project are filled", () => {
    render(<CsmAnnouncementCreatePage />);
    const submit = screen.getByRole("button", { name: /create announcement/i });
    expect(submit).toBeDisabled();

    fillSubjectAndDescription();
    // Subject + description are filled, but no project is selected yet.
    expect(submit).toBeDisabled();

    selectProjects("proj-1");
    expect(submit).toBeEnabled();
  });

  it("fans out one POST /cases call per selected project, with the same subject/description", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "ann-1" });
    render(<CsmAnnouncementCreatePage />);

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");

    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));
    await screen.findByRole("button", { name: /creating/i });

    await waitFor(() => {
      expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(2);
    });
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith({
      type: "announcement",
      projectId: "proj-1",
      subject: "Scheduled maintenance",
      description: "<p>Maintenance window details.</p>",
    });
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith({
      type: "announcement",
      projectId: "proj-2",
      subject: "Scheduled maintenance",
      description: "<p>Maintenance window details.</p>",
    });

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
    });
  });

  it("reports which project failed on a partial failure, keeps the succeeded one, and still navigates back", async () => {
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      projectId === "proj-1"
        ? Promise.resolve({ id: "ann-1" })
        : Promise.reject(new Error("network down")),
    );
    render(<CsmAnnouncementCreatePage />);

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(
        expect.stringContaining("proj-2"),
      );
    });
    expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
  });

  it("surfaces a single error and does not navigate when every project fails", async () => {
    postCaseMutateAsyncMock.mockRejectedValue(new Error("network down"));
    render(<CsmAnnouncementCreatePage />);

    fillSubjectAndDescription();
    selectProjects("proj-1");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(
        "Could not create the announcement. Please try again.",
      );
    });
    expect(navigateMock).not.toHaveBeenCalled();
  });
});
