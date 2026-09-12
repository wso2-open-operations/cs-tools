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

import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { useCaseTabsController } from "@context/case-tabs/CaseTabsContext";
import type { CaseTabState } from "@context/case-tabs/caseTabsTypes";
import { tabDisplayLabel } from "@features/case-tabs/utils/tabDisplayLabel";

type PendingClose =
  | { kind: "single"; tab: CaseTabState }
  | { kind: "all"; draftedTabs: CaseTabState[] }
  | { kind: "others"; keepId: string; draftedTabs: CaseTabState[] };

function draftedLabelList(tabs: CaseTabState[]): string {
  return tabs.map(tabDisplayLabel).join(", ");
}

/**
 * Owns the close confirmation for any case tab whose reply composer is open
 * (a best-effort "might have unsaved text" signal — see
 * `CaseTabState.hasDraft` and `useReportCaseTabDraft`'s own doc comment on
 * the limits of that signal) — a single tab's own × (`requestClose`), the
 * tab strip's right-click "Close all tabs" (`requestCloseAll`), and "Close
 * other tabs" (`requestCloseOthers`). All three route through the SAME
 * confirm: bulk-closing used to skip it entirely and discard every affected
 * draft unconditionally, which is exactly the data-loss `hasDraft` exists to
 * guard against — there is no size of "how many tabs" that makes silently
 * discarding a reply-in-progress an acceptable default.
 *
 * Split out from `CaseTabStrip` (a plain component) into its own hook module
 * so that file can stay a component-only export (fast-refresh requires
 * this).
 */
export function useCaseTabCloseConfirm(): {
  requestClose: (tab: CaseTabState) => void;
  /** Right-click "Close all tabs" — `openTabs` is every currently open case
   * tab (never includes the permanent pinned one, which isn't part of this
   * array to begin with — see `CaseTabState`'s own doc comment). */
  requestCloseAll: (openTabs: CaseTabState[]) => void;
  /** Right-click "Close other tabs" — every open tab except `keepId`. */
  requestCloseOthers: (openTabs: CaseTabState[], keepId: string) => void;
  dialog: JSX.Element;
} {
  const { closeTab, closeAllTabs, closeOtherTabs } = useCaseTabsController();
  const [pending, setPending] = useState<PendingClose | null>(null);

  const requestClose = (tab: CaseTabState): void => {
    if (tab.hasDraft) {
      setPending({ kind: "single", tab });
      return;
    }
    closeTab(tab.id);
  };

  const requestCloseAll = (openTabs: CaseTabState[]): void => {
    const draftedTabs = openTabs.filter((t) => t.hasDraft);
    if (draftedTabs.length === 0) {
      closeAllTabs();
      return;
    }
    setPending({ kind: "all", draftedTabs });
  };

  const requestCloseOthers = (openTabs: CaseTabState[], keepId: string): void => {
    const draftedTabs = openTabs.filter((t) => t.id !== keepId && t.hasDraft);
    if (draftedTabs.length === 0) {
      closeOtherTabs(keepId);
      return;
    }
    setPending({ kind: "others", keepId, draftedTabs });
  };

  const closeConfirmed = (): void => {
    if (!pending) return;
    if (pending.kind === "single") closeTab(pending.tab.id);
    else if (pending.kind === "all") closeAllTabs();
    else closeOtherTabs(pending.keepId);
    setPending(null);
  };

  const title =
    pending?.kind === "single"
      ? "Close this case tab?"
      : pending?.kind === "all"
        ? "Close all tabs?"
        : "Close other tabs?";

  const bodyText =
    pending?.kind === "single"
      ? `${tabDisplayLabel(pending.tab)} has a reply in progress. Closing this tab will discard it.`
      : pending
        ? pending.draftedTabs.length === 1
          ? `${draftedLabelList(pending.draftedTabs)} has a reply in progress. Closing ${
              pending.kind === "all" ? "all tabs" : "these tabs"
            } will discard it.`
          : `${pending.draftedTabs.length} of these tabs (${draftedLabelList(
              pending.draftedTabs,
            )}) have a reply in progress. Closing ${
              pending.kind === "all" ? "all tabs" : "these tabs"
            } will discard them.`
        : "";

  const dialog = (
    <Dialog open={pending !== null} onClose={() => setPending(null)} maxWidth="xs" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <DialogContentText>{bodyText}</DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={() => setPending(null)}>
          {pending?.kind === "single" ? "Keep tab open" : "Keep tabs open"}
        </Button>
        <Button variant="contained" color="error" onClick={closeConfirmed}>
          Close anyway
        </Button>
      </DialogActions>
    </Dialog>
  );

  return { requestClose, requestCloseAll, requestCloseOthers, dialog };
}
