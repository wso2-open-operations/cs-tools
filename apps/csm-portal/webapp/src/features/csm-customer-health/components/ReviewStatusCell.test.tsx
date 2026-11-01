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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ReviewStatusCell from "./ReviewStatusCell";
import type { ProjectDetail, ProjectHealthStatus } from "../api/customerHealthTypes";

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

const mutation = () => ({ mutateAsync: vi.fn(), isPending: false });
vi.mock("../api/useProjectRisk", () => ({
  useOpenProjectRisk: () => mutation(),
  useMarkProjectHealthy: () => mutation(),
  useCloseProjectRisk: () => mutation(),
  useProjectRiskHistory: () => ({ data: undefined, isLoading: false }),
  closeProjectRiskErrorMessage: (_err: unknown, fallback: string) => fallback,
}));

const project = { sysId: "p1", name: "Acme Cloud" } as ProjectDetail;

function status(over: Partial<ProjectHealthStatus> = {}): ProjectHealthStatus {
  return {
    projectSysId: "p1",
    healthStatus: { id: 1, projectSysId: "p1", accountSysId: "a1", status: "to_be_reviewed", reviewedByEmail: null, reviewedOn: null },
    openRisk: null,
    ...over,
  };
}

/** The dialog's reason box: named by its label, which carries the asterisk. */
function expectRequiredReason(dialogName: RegExp, reason: string) {
  const dialog = screen.getByRole("dialog", { name: dialogName });
  const box = within(dialog).getByRole("textbox", { name: reason });
  expect(box).toBeRequired();
  const label = dialog.querySelector(`label[for="${box.id}"]`);
  expect(label).toHaveTextContent(reason);
  expect(label?.querySelector(".MuiFormLabel-asterisk")).toBeInTheDocument();
  // Confirm waits for a reason.
  const confirm = within(dialog).getByRole("button", { name: "Confirm" });
  expect(confirm).toBeDisabled();
  fireEvent.change(box, { target: { value: "Because." } });
  expect(confirm).toBeEnabled();
}

describe("ReviewStatusCell: the reasons it asks for are marked required", () => {
  it("asks for a reason to mark a project at risk", () => {
    render(<ReviewStatusCell project={project} accountSysId="a1" healthStatus={status()} onStatusChanged={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Mark At Risk" }));
    expectRequiredReason(/as At Risk$/, "Reason for marking at risk");
  });

  it("asks for a reason to mark a project healthy", () => {
    render(<ReviewStatusCell project={project} accountSysId="a1" healthStatus={status()} onStatusChanged={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Mark Healthy" }));
    expectRequiredReason(/as Healthy$/, "Reason for marking healthy");
  });

  it("asks for a reason to close a risk", () => {
    const atRisk = status({
      healthStatus: { id: 1, projectSysId: "p1", accountSysId: "a1", status: "at_risk", reviewedByEmail: null, reviewedOn: null },
      openRisk: {
        id: 9,
        projectSysId: "p1",
        accountSysId: "a1",
        status: "open",
        openedComment: "Renewal stalled",
        openedByEmail: "csm@example.test",
        openedOn: "2026-10-01T09:00:00Z",
        closedComment: null,
        actionItems: [],
      },
    });
    render(<ReviewStatusCell project={project} accountSysId="a1" healthStatus={atRisk} onStatusChanged={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Close Risk" }));
    expectRequiredReason(/^Close Risk for/, "Reason for closing risk");
  });
});
