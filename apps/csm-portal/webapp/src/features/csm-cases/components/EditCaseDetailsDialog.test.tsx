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
    fireEvent.change(screen.getByLabelText(/^subject$/i), {
      target: { value: "Updated subject" },
    });
    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    expect(onSubmit).toHaveBeenCalledWith({ subject: "Updated subject" });
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
    fireEvent.change(screen.getByLabelText(/^subject$/i), {
      target: { value: "Updated subject" },
    });
    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));
    expect(await screen.findByText(/subject saved\./i)).toBeInTheDocument();
    expect(await screen.findByText(/deployment: sla conflict/i)).toBeInTheDocument();
  });
});
