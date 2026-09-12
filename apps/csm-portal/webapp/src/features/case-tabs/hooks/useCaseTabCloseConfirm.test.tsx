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
import { beforeEach, describe, expect, it } from "vitest";
import type { JSX } from "react";
import "@testing-library/jest-dom/vitest";
import { useCaseTabCloseConfirm } from "@features/case-tabs/hooks/useCaseTabCloseConfirm";
import { CaseTabsProvider, useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import { CaseTabsBehaviorProvider } from "@context/case-tabs/CaseTabsBehaviorContext";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";

const TAB_NO_DRAFT: CaseTabState = {
  id: "t1",
  caseId: "CS1",
  kind: "case",
  path: "/cases/CS1",
  label: "CS1 · First case",
  hasDraft: false,
};
const TAB_WITH_DRAFT: CaseTabState = {
  id: "t2",
  caseId: "CS2",
  kind: "case",
  path: "/cases/CS2",
  label: "CS2 · Second case",
  hasDraft: true,
};

function Harness({ tab }: { tab: CaseTabState }): JSX.Element {
  const { requestClose, dialog } = useCaseTabCloseConfirm();
  const { tabs } = useCaseTabsController();
  return (
    <div>
      <div data-testid="open-count">{tabs.length}</div>
      <button onClick={() => requestClose(tab)}>close</button>
      {dialog}
    </div>
  );
}

/** Opens two real tabs (one with a draft, one without) via the actual
 * controller, then exposes the bulk-close actions under test — so these
 * assert on real post-close tab state, not just whether a dialog rendered. */
function BulkHarness(): JSX.Element {
  const { openTab, tabs, setTabDraft } = useCaseTabsController();
  const { requestCloseAll, requestCloseOthers, dialog } = useCaseTabCloseConfirm();
  return (
    <div>
      <div data-testid="open-case-ids">{tabs.map((t) => t.caseId).join(",")}</div>
      <button
        onClick={() => {
          openTab("CS1", "case", "/cases/CS1");
          openTab("CS2", "case", "/cases/CS2");
        }}
      >
        open-both
      </button>
      <button onClick={() => setTabDraft(tabs.find((t) => t.caseId === "CS2")!.id, true)}>
        mark-cs2-draft
      </button>
      <button onClick={() => requestCloseAll(tabs)}>close-all</button>
      <button
        onClick={() => requestCloseOthers(tabs, tabs.find((t) => t.caseId === "CS1")!.id)}
      >
        close-others-keep-cs1
      </button>
      {dialog}
    </div>
  );
}

describe("useCaseTabCloseConfirm", () => {
  beforeEach(() => {
    sessionStorage.clear();
    localStorage.setItem("csm.caseTabs.enabled", "1");
  });

  it("closes immediately when the tab has no draft, without confirming", () => {
    render(
      <CaseTabsProvider>
        <Harness tab={TAB_NO_DRAFT} />
      </CaseTabsProvider>,
    );
    fireEvent.click(screen.getByText("close"));
    expect(screen.queryByText("Close this case tab?")).not.toBeInTheDocument();
  });

  it("asks for confirmation when the tab has a draft, and respects Keep tab open", async () => {
    render(
      <CaseTabsProvider>
        <Harness tab={TAB_WITH_DRAFT} />
      </CaseTabsProvider>,
    );
    fireEvent.click(screen.getByText("close"));
    expect(screen.getByText("Close this case tab?")).toBeInTheDocument();
    fireEvent.click(screen.getByText("Keep tab open"));
    // MUI's Dialog unmounts its content after an exit transition rather than
    // synchronously on the click — wait for that instead of asserting
    // immediately.
    await waitFor(() =>
      expect(screen.queryByText("Close this case tab?")).not.toBeInTheDocument(),
    );
  });

  // Regression test: this dialog used to have its own copy of the tab-label
  // fallback logic, and its copy fell back to the raw `caseId`/UUID instead
  // of "Loading…" (unlike `CaseTabStrip`'s own chips, which already showed
  // "Loading…") — the two have since been consolidated onto the same shared
  // `tabDisplayLabel` helper, so both must show the same fallback text.
  it("shows the 'Loading…' fallback (not the raw caseId/UUID) when the tab's label hasn't resolved yet", () => {
    render(
      <CaseTabsProvider>
        <Harness tab={{ ...TAB_WITH_DRAFT, label: undefined }} />
      </CaseTabsProvider>,
    );
    fireEvent.click(screen.getByText("close"));
    expect(screen.getByText(/^Loading… has a reply in progress\./)).toBeInTheDocument();
    expect(screen.queryByText(/^CS2 has a reply in progress\./)).not.toBeInTheDocument();
  });

  describe("bulk close (Close all tabs / Close other tabs)", () => {
    function renderBulk() {
      return render(
        <CaseTabsBehaviorProvider>
          <CaseTabsProvider>
            <BulkHarness />
          </CaseTabsProvider>
        </CaseTabsBehaviorProvider>,
      );
    }

    it("closes all tabs immediately when none have a draft, without confirming", () => {
      renderBulk();
      fireEvent.click(screen.getByText("open-both"));
      expect(screen.getByTestId("open-case-ids")).toHaveTextContent("CS1,CS2");
      fireEvent.click(screen.getByText("close-all"));
      expect(screen.queryByText("Close all tabs?")).not.toBeInTheDocument();
      expect(screen.getByTestId("open-case-ids")).toHaveTextContent("");
    });

    // Regression test: "Close all tabs" used to discard every affected
    // draft unconditionally — this is the fix.
    it("asks for confirmation before Close all tabs discards a draft, and Keep tabs open leaves every tab untouched", async () => {
      renderBulk();
      fireEvent.click(screen.getByText("open-both"));
      fireEvent.click(screen.getByText("mark-cs2-draft"));

      fireEvent.click(screen.getByText("close-all"));
      expect(screen.getByText("Close all tabs?")).toBeInTheDocument();
      // The opened-via-`openTab` tab never had a page report a real label
      // (see `useReportCaseTabMeta`, not exercised by this harness), so it
      // falls back to "Loading…" the same as everywhere else this fallback
      // applies — not the raw caseId/UUID.
      expect(screen.getByText(/^Loading… has a reply in progress\./)).toBeInTheDocument();
      // Still open — nothing closed while the confirm is pending.
      expect(screen.getByTestId("open-case-ids")).toHaveTextContent("CS1,CS2");

      fireEvent.click(screen.getByText("Keep tabs open"));
      await waitFor(() =>
        expect(screen.queryByText("Close all tabs?")).not.toBeInTheDocument(),
      );
      expect(screen.getByTestId("open-case-ids")).toHaveTextContent("CS1,CS2");
    });

    it("Close anyway on the bulk confirm actually closes every tab, including the drafted one", async () => {
      renderBulk();
      fireEvent.click(screen.getByText("open-both"));
      fireEvent.click(screen.getByText("mark-cs2-draft"));

      fireEvent.click(screen.getByText("close-all"));
      fireEvent.click(screen.getByText("Close anyway"));
      await waitFor(() =>
        expect(screen.queryByText("Close all tabs?")).not.toBeInTheDocument(),
      );
      expect(screen.getByTestId("open-case-ids")).toHaveTextContent("");
    });

    it("closes other tabs immediately when none of them have a draft", () => {
      renderBulk();
      fireEvent.click(screen.getByText("open-both"));
      fireEvent.click(screen.getByText("close-others-keep-cs1"));
      expect(screen.queryByText("Close other tabs?")).not.toBeInTheDocument();
      expect(screen.getByTestId("open-case-ids")).toHaveTextContent("CS1");
    });

    it("asks for confirmation before Close other tabs discards the OTHER tab's draft — the kept tab's own draft is irrelevant", async () => {
      renderBulk();
      fireEvent.click(screen.getByText("open-both"));
      fireEvent.click(screen.getByText("mark-cs2-draft"));

      // CS2 (the one with the draft) is the one being closed here — CS1 is
      // kept, so CS1's own draft state (it has none) is irrelevant.
      fireEvent.click(screen.getByText("close-others-keep-cs1"));
      expect(screen.getByText("Close other tabs?")).toBeInTheDocument();
      expect(screen.getByTestId("open-case-ids")).toHaveTextContent("CS1,CS2");

      fireEvent.click(screen.getByText("Close anyway"));
      await waitFor(() =>
        expect(screen.queryByText("Close other tabs?")).not.toBeInTheDocument(),
      );
      expect(screen.getByTestId("open-case-ids")).toHaveTextContent("CS1");
    });
  });
});
