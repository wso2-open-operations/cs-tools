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

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import EditCaseDetailsDialog from "@features/csm-cases/components/EditCaseDetailsDialog";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useDeployedProductOptions } from "@features/csm-cases/api/useDeployedProductOptions";

vi.mock("@features/csm-cases/api/useSearchDeployments", () => ({
  useSearchDeployments: vi.fn(),
}));
vi.mock("@features/csm-cases/api/useDeployedProductOptions", () => ({
  useDeployedProductOptions: vi.fn(),
}));
// The dialog reuses the Lexical rich-text editor from case create; it isn't
// under test here, so stub it down to a plain textarea (same technique the
// case-create page's own tests would use for the same dependency).
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (v: string) => void;
  }) => (
    <textarea
      aria-label="description-editor"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));

const mockUseSearchDeployments = vi.mocked(useSearchDeployments);
const mockUseDeployedProductOptions = vi.mocked(useDeployedProductOptions);

function setupHooks(): void {
  mockUseSearchDeployments.mockReturnValue({
    data: [{ id: "dep-1", name: "Production" }],
    isLoading: false,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
  } as any);
  mockUseDeployedProductOptions.mockReturnValue({
    data: [{ id: "dp-1", label: "WSO2 API Manager 4.3.0" }],
    isLoading: false,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
  } as any);
}

describe("EditCaseDetailsDialog — change gating and sequential per-field submit", () => {
  it("disables Save changes until a field actually changes", () => {
    setupHooks();
    render(
      <EditCaseDetailsDialog
        projectId="prj-1"
        currentSubject="Original subject"
        currentDescriptionHtml="<p>Original</p>"
        isSaving={false}
        onClose={() => {}}
        onSubmit={vi.fn().mockResolvedValue([])}
      />,
    );
    expect(screen.getByRole("button", { name: /save changes/i })).toBeDisabled();
  });

  it("submits only the changed fields", () => {
    setupHooks();
    const onSubmit = vi.fn().mockResolvedValue([{ field: "subject", ok: true }]);
    render(
      <EditCaseDetailsDialog
        projectId="prj-1"
        currentSubject="Original subject"
        currentDescriptionHtml="<p>Original</p>"
        isSaving={false}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fireEvent.change(screen.getByRole("textbox", { name: /^subject$/i }), {
      target: { value: "Updated subject" },
    });
    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    expect(onSubmit).toHaveBeenCalledWith({ subject: "Updated subject" });
  });

  it("marks Subject required and blocks Save with an error once it is cleared", async () => {
    setupHooks();
    const onSubmit = vi.fn().mockResolvedValue([]);
    render(
      <EditCaseDetailsDialog
        projectId="prj-1"
        currentSubject="Original subject"
        currentDescriptionHtml="<p>Original</p>"
        isSaving={false}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    const subjectInput = screen.getByRole("textbox", { name: /^subject$/i });
    expect(subjectInput).toBeRequired();
    expect(screen.queryByText("Subject is required.")).not.toBeInTheDocument();

    // Another field changed, but the subject is cleared: Save stays blocked
    // instead of quietly saving the description and keeping the old subject.
    fireEvent.change(screen.getByLabelText("description-editor"), {
      target: { value: "<p>Changed</p>" },
    });
    fireEvent.change(subjectInput, { target: { value: "   " } });
    expect(screen.getByText("Subject is required.")).toBeInTheDocument();
    expect(subjectInput).toHaveAttribute("aria-invalid", "true");
    const save = screen.getByRole("button", { name: /save changes/i });
    expect(save).toBeDisabled();
    fireEvent.click(save);
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.change(subjectInput, { target: { value: "New subject" } });
    expect(screen.queryByText("Subject is required.")).not.toBeInTheDocument();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    });
    expect(onSubmit).toHaveBeenCalledWith({
      subject: "New subject",
      description: "<p>Changed</p>",
    });
  });

  it("still saves other fields on a case that never had a subject, until the subject is touched", async () => {
    setupHooks();
    const onSubmit = vi.fn().mockResolvedValue([]);
    render(
      <EditCaseDetailsDialog
        projectId="prj-1"
        currentSubject=""
        currentDescriptionHtml="<p>Original</p>"
        isSaving={false}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fireEvent.change(screen.getByLabelText("description-editor"), {
      target: { value: "<p>Changed</p>" },
    });
    expect(screen.queryByText("Subject is required.")).not.toBeInTheDocument();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    });
    expect(onSubmit).toHaveBeenCalledWith({ description: "<p>Changed</p>" });
  });

  it("does not mark an untouched blank subject required, only one the user has changed", () => {
    setupHooks();
    render(
      <EditCaseDetailsDialog
        projectId="prj-1"
        currentSubject=""
        currentDescriptionHtml="<p>Original</p>"
        isSaving={false}
        onClose={() => {}}
        onSubmit={vi.fn().mockResolvedValue([])}
      />,
    );
    const subjectInput = screen.getByRole("textbox", { name: /^subject$/i });
    expect(subjectInput).not.toBeRequired();

    // Typed into, it is held to the same rule as a stored subject.
    fireEvent.change(subjectInput, { target: { value: "  " } });
    expect(subjectInput).toBeRequired();
    expect(screen.getByText("Subject is required.")).toBeInTheDocument();

    // Back to exactly what was stored: untouched again, so neither.
    fireEvent.change(subjectInput, { target: { value: "" } });
    expect(subjectInput).not.toBeRequired();
    expect(screen.queryByText("Subject is required.")).not.toBeInTheDocument();
  });

  it("renders a per-field result once the submission settles, including a failure", async () => {
    setupHooks();
    const onSubmit = vi.fn().mockResolvedValue([
      { field: "subject", ok: true },
      { field: "deploymentId", ok: false, error: "SLA conflict" },
    ]);
    render(
      <EditCaseDetailsDialog
        projectId="prj-1"
        currentSubject="Original subject"
        currentDescriptionHtml="<p>Original</p>"
        isSaving={false}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    fireEvent.change(screen.getByRole("textbox", { name: /^subject$/i }), {
      target: { value: "Updated subject" },
    });
    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    expect(await screen.findByText(/subject saved\./i)).toBeInTheDocument();
    expect(await screen.findByText(/deployment: sla conflict/i)).toBeInTheDocument();
  });

  describe("onAllSaved", () => {
    function renderAndSave(
      results: Array<{ field: string; ok: boolean; error?: string }>,
      onAllSaved: () => void,
    ): void {
      setupHooks();
      render(
        <EditCaseDetailsDialog
          projectId="prj-1"
          currentSubject="Original subject"
          currentDescriptionHtml="<p>Original</p>"
          isSaving={false}
          onClose={() => {}}
          onAllSaved={onAllSaved}
          onSubmit={vi.fn().mockResolvedValue(results)}
        />,
      );
      fireEvent.change(screen.getByRole("textbox", { name: /^subject$/i }), {
        target: { value: "Updated subject" },
      });
      fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    }

    it("is called once every changed field has saved", async () => {
      const onAllSaved = vi.fn();
      renderAndSave([{ field: "subject", ok: true }], onAllSaved);
      await waitFor(() => expect(onAllSaved).toHaveBeenCalledTimes(1));
    });

    it("is not called on a partial failure, and the per-field results stay on screen", async () => {
      const onAllSaved = vi.fn();
      renderAndSave(
        [
          { field: "subject", ok: true },
          { field: "deploymentId", ok: false, error: "SLA conflict" },
        ],
        onAllSaved,
      );
      expect(await screen.findByText(/deployment: sla conflict/i)).toBeInTheDocument();
      expect(onAllSaved).not.toHaveBeenCalled();
    });

    it("is not called when the submission reports no results", async () => {
      const onAllSaved = vi.fn();
      renderAndSave([], onAllSaved);
      await waitFor(() => expect(screen.getByRole("button", { name: /save changes/i })).toBeInTheDocument());
      expect(onAllSaved).not.toHaveBeenCalled();
    });
  });
});
