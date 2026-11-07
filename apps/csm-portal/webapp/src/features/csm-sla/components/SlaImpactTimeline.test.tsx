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
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { SlaImpactTimeline } from "@features/csm-sla/components/SlaImpactTimeline";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

const ENTRIES: CaseAuditEntry[] = [
  {
    id: "a1",
    kind: "field_change",
    actor: "Jane Doe",
    createdAt: "2026-07-01T10:00:00Z",
    changes: [
      { field: "state", fieldLabel: "State", previousValue: "Work In Progress", newValue: "Awaiting Info" },
    ],
  },
  {
    id: "a2",
    kind: "field_change",
    actor: "Jane Doe",
    createdAt: "2026-07-02T10:00:00Z",
    changes: [{ field: "workaround_provided_on", fieldLabel: "Workaround Provided", newValue: "2026-07-02T10:00:00Z" }],
  },
];

function renderTimeline(overrides: Partial<Parameters<typeof SlaImpactTimeline>[0]> = {}) {
  return render(
    <SlaImpactTimeline entries={ENTRIES} isLoading={false} isError={false} onRetry={vi.fn()} {...overrides} />,
  );
}

afterEach(() => {
  window.config = {} as typeof window.config;
});

describe("SlaImpactTimeline", () => {
  it("renders events with side label, actor and from -> to values", () => {
    renderTimeline();
    expect(screen.getByText("Waiting on customer")).toBeInTheDocument();
    expect(screen.getByText("Customer side")).toBeInTheDocument();
    expect(screen.getByText("Work in progress")).toBeInTheDocument();
    expect(screen.getByText("Awaiting info")).toBeInTheDocument();
    expect(screen.getByText("Workaround provided")).toBeInTheDocument();
    expect(screen.getAllByText(/Jane Doe/)).toHaveLength(2);
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
  });

  it("shows a loading state", () => {
    renderTimeline({ isLoading: true, entries: undefined });
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
    expect(screen.queryByRole("listitem")).not.toBeInTheDocument();
  });

  it("shows an empty state when nothing SLA-relevant was recorded", () => {
    renderTimeline({ entries: [] });
    expect(screen.getByText(/No SLA-affecting changes/)).toBeInTheDocument();
  });

  it("shows an error state with a working retry", () => {
    const onRetry = vi.fn();
    renderTimeline({ isError: true, onRetry });
    expect(screen.getByText(/Could not load the SLA impact timeline/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("is shown when the flag is true and when it is absent", () => {
    window.config = { CSM_PORTAL_SLA_TIMELINE_ENABLED: true } as unknown as typeof window.config;
    const { unmount } = renderTimeline();
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
    unmount();
    window.config = {} as typeof window.config;
    renderTimeline();
    expect(screen.getByText("SLA impact timeline")).toBeInTheDocument();
  });

  it("renders nothing when the flag is explicitly false", () => {
    window.config = { CSM_PORTAL_SLA_TIMELINE_ENABLED: false } as unknown as typeof window.config;
    const { container } = renderTimeline();
    expect(container).toBeEmptyDOMElement();
  });
});
