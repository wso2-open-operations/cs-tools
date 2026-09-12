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
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";

import { LinkedIncidentWidget } from "@features/csm-cases/components/LinkedIncidentWidget";

/** This widget only needs a router context (the linked-incident row is a
 * `RouterLink`); no query client, unlike its sibling widgets — it reads
 * `parentCase` straight off the already-loaded case detail. */
function renderWithProviders(ui: ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe("LinkedIncidentWidget", () => {
  it("renders the linked incident as a link when the parent is an incident", () => {
    renderWithProviders(
      <LinkedIncidentWidget
        caseId="case-1"
        parentCase={{ id: "inc-1", caseNumber: "INC0000001", type: "incident" }}
        onLinkIncident={vi.fn()}
      />,
    );
    const link = screen.getByRole("link", { name: "INC0000001" });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute("href", "/operations/incidents/inc-1");
    expect(
      screen.queryByRole("button", { name: /link to incident/i }),
    ).not.toBeInTheDocument();
  });

  it("renders the empty state and a Link-to-incident button when there is no parent", () => {
    renderWithProviders(
      <LinkedIncidentWidget caseId="case-1" parentCase={undefined} onLinkIncident={vi.fn()} />,
    );
    expect(screen.getByText("No incident linked to this case.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /link to incident/i })).toBeInTheDocument();
  });

  it("renders the empty state when the parent is a different record kind (e.g. a parent case)", () => {
    // This widget only owns the incident relationship — a parent case,
    // change request, or problem stays on CaseMetaBand's generic chip and is
    // not shown here as "linked".
    renderWithProviders(
      <LinkedIncidentWidget
        caseId="case-1"
        parentCase={{ id: "case-2", caseNumber: "CS-0002", type: "case" }}
        onLinkIncident={vi.fn()}
      />,
    );
    expect(screen.getByText("No incident linked to this case.")).toBeInTheDocument();
  });

  it("calls onLinkIncident when the button is clicked", () => {
    const onLinkIncident = vi.fn();
    renderWithProviders(
      <LinkedIncidentWidget caseId="case-1" parentCase={undefined} onLinkIncident={onLinkIncident} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /link to incident/i }));
    expect(onLinkIncident).toHaveBeenCalledTimes(1);
  });

  it("disables the button and shows a tooltip when the case is closed", () => {
    renderWithProviders(
      <LinkedIncidentWidget
        caseId="case-1"
        parentCase={undefined}
        onLinkIncident={vi.fn()}
        linkDisabled
      />,
    );
    const button = screen.getByRole("button", { name: /link to incident/i });
    expect(button).toBeDisabled();
  });
});
