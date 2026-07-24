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

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import LinkCaseDialog from "@features/csm-cases/components/LinkCaseDialog";
import { useQuickCaseSearch } from "@features/csm-cases/api/useQuickCaseSearch";

// Mocked wholesale (no `vi.importActual`) — the real module transitively pulls
// in the auth-aware API client, which throws at import time without a runtime
// `window.config` (only ever provided by the gitignored `public/config.js` at
// app startup, never in this test environment). `QUICK_CASE_MIN_QUERY_LEN` is
// re-declared here rather than imported from the real module for that reason.
vi.mock("@features/csm-cases/api/useQuickCaseSearch", () => ({
  QUICK_CASE_MIN_QUERY_LEN: 2,
  useQuickCaseSearch: vi.fn(),
}));

const mockUseQuickCaseSearch = vi.mocked(useQuickCaseSearch);

function mockHits(): void {
  mockUseQuickCaseSearch.mockReturnValue({
    data: [
      {
        id: "case-2002",
        caseNumber: "CS-2002",
        subject: "Related latency issue",
        severity: "S2",
        state: "open",
      },
    ],
    isFetching: false,
    isError: false,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
  } as any);
}

describe("LinkCaseDialog — search-and-select", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  /** Types into the search box and lets the 300ms debounce settle. */
  function typeAndDebounce(value: string): void {
    fireEvent.change(screen.getByPlaceholderText(/search by case number/i), {
      target: { value },
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
  }

  it("prompts to type at least the minimum query length before searching", () => {
    mockUseQuickCaseSearch.mockReturnValue({
      data: [],
      isFetching: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    render(
      <LinkCaseDialog
        currentCaseId="case-1001"
        isLinking={false}
        onClose={() => {}}
        onLink={() => {}}
      />,
    );
    expect(screen.getByText(/type at least/i)).toBeInTheDocument();
  });

  it("defaults to linking as parent and calls onLink with the chosen case", () => {
    mockHits();
    const onLink = vi.fn();
    render(
      <LinkCaseDialog
        currentCaseId="case-1001"
        isLinking={false}
        onClose={() => {}}
        onLink={onLink}
      />,
    );
    typeAndDebounce("CS-2002");
    fireEvent.click(screen.getByText(/CS-2002 — Related latency issue/i));
    expect(onLink).toHaveBeenCalledWith("case-2002", "parent");
  });

  it("links as related once that option is picked", () => {
    mockHits();
    const onLink = vi.fn();
    render(
      <LinkCaseDialog
        currentCaseId="case-1001"
        isLinking={false}
        onClose={() => {}}
        onLink={onLink}
      />,
    );
    fireEvent.click(screen.getByRole("radio", { name: /as related case/i }));
    typeAndDebounce("CS-2002");
    fireEvent.click(screen.getByText(/CS-2002 — Related latency issue/i));
    expect(onLink).toHaveBeenCalledWith("case-2002", "related");
  });

  it("calls onClose on Cancel without calling onLink", () => {
    mockHits();
    const onLink = vi.fn();
    const onClose = vi.fn();
    render(
      <LinkCaseDialog
        currentCaseId="case-1001"
        isLinking={false}
        onClose={onClose}
        onLink={onLink}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onLink).not.toHaveBeenCalled();
  });
});
