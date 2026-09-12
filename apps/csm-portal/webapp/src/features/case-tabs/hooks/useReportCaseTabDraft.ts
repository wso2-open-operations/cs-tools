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

import { useEffect } from "react";
import { useCaseTabsController } from "@context/case-tabs/CaseTabsContext";

/**
 * Reports whether this case's reply composer is currently open up to
 * `CaseTabsContext`, so the tab strip can confirm before discarding it on
 * close (see `useCaseTabCloseConfirm`).
 *
 * A best-effort signal, not a true "has unsaved text" check: it's "the
 * composer is open" (`composerOpen` in `CsmCaseDetailPage`), not "the
 * composer has non-empty content" — `CsmCaseCommentInput` doesn't currently
 * expose its own draft text to its parent, and adding that plumbing was out
 * of scope for this pass (see this feature's own notes). An open, empty
 * composer prompts a confirm it doesn't strictly need to; that false
 * positive is judged safer than a false negative (silently discarding real
 * typed text).
 *
 * No-ops when this page isn't part of any open tab (e.g. it's being shown as
 * the un-tabbed fallback for a case opened past the open-tab cap — see
 * `CaseDetailRouteSync`).
 */
export function useReportCaseTabDraft(
  caseId: string | undefined,
  composerOpen: boolean,
): void {
  const { tabs, setTabDraft } = useCaseTabsController();
  const tabId = caseId ? tabs.find((t) => t.caseId === caseId)?.id : undefined;

  useEffect(() => {
    if (!tabId) return;
    setTabDraft(tabId, composerOpen);
    // Clear the flag on unmount so a stale "has draft" doesn't linger if
    // this hook's owning instance goes away without the tab itself closing
    // (not expected in the isolated-tab case, but cheap insurance).
    return () => setTabDraft(tabId, false);
  }, [tabId, composerOpen, setTabDraft]);
}
