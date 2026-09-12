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

export interface CaseTabMeta {
  /** Short chip label — the record's number only (e.g. "CS0001"). */
  label: string | undefined;
  /** Internal/project-scoped id shown in the tab's hover tooltip (e.g.
   * "CPASUB-8" — `wso2CaseId` for cases, or the incident/CR equivalent).
   * Distinct from `label`: never shown as the chip's own text, only in the
   * tooltip alongside `subject`. */
  internalId: string | undefined;
  /** Subject/title shown in the tab's hover tooltip alongside `internalId`. */
  subject: string | undefined;
}

/**
 * Reports this record's display label ("CS0001") and its fuller tooltip
 * identity (internal id + subject) up to `CaseTabsContext`, so the tab strip
 * can show both without a second, independent data fetch of its own.
 *
 * An EARLIER version of this had the tab strip resolve each tab's label via
 * its OWN separate `useGetCsmCaseDetail(tab.caseId)` call (`CaseTabLabel`,
 * now removed), reasoning that it would share `CsmCaseDetailPage`'s React
 * Query cache entry for that case and so cost nothing extra. In practice
 * that produced a real bug: a tab's chip stayed on the raw caseId until the
 * user switched away and back, i.e. the label query's own render cycle
 * wasn't reliably picking up data the page had already loaded. Having the
 * page that's ACTUALLY rendering (and already computes this exact data for
 * its own header) report it directly removes that whole class of "two
 * independent consumers of the same query, but only one renders it"
 * staleness risk — there is now exactly one source of truth, the same
 * `data` the page uses for everything else it renders.
 *
 * No-ops when this page isn't part of any open tab (e.g. the un-tabbed
 * fallback for a record opened past the open-tab cap — see
 * `CaseDetailRouteSync`), same as `useReportCaseTabDraft`.
 */
export function useReportCaseTabMeta(
  caseId: string | undefined,
  meta: CaseTabMeta,
): void {
  const { tabs, setTabMeta } = useCaseTabsController();
  const tabId = caseId ? tabs.find((t) => t.caseId === caseId)?.id : undefined;
  const { label, internalId, subject } = meta;

  useEffect(() => {
    if (!tabId) return;
    setTabMeta(tabId, { label, internalId, subject });
  }, [tabId, label, internalId, subject, setTabMeta]);
}
