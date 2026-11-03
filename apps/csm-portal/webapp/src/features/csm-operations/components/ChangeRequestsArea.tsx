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

import { Box, Tab, Tabs } from "@wso2/oxygen-ui";
import { useEffect, type JSX } from "react";
import { useSearchParams } from "react-router";
import ChangeRequestsTab from "@features/csm-operations/components/ChangeRequestsTab";
import StandardChangesTab from "@features/csm-operations/components/StandardChangesTab";
import {
  canonicalStandardChangesSearch,
  changeRequestListSearch,
  changeRequestsView,
  isLegacyStandardChangesView,
  STANDARD_CHANGES_LABELS,
  standardChangesSearch,
  standardChangesShow,
  type ChangeRequestsView,
} from "@features/csm-operations/utils/standardChanges";

const SUB_VIEWS: ReadonlyArray<{ value: ChangeRequestsView; label: string }> = [
  { value: "list", label: "Change requests" },
  { value: "standard", label: STANDARD_CHANGES_LABELS.templates },
];

/**
 * Operations > Change requests. Standard changes are templated change requests,
 * so they are not a tab of their own beside this one: they are one sub-view of
 * it, next to the change request list (`?view=standard`), which opens on the
 * template catalogue and holds the proposals to the CAB as a secondary view
 * (`StandardChangesTab`). The sub-view is in the URL (`changeRequestsView`), so
 * it is linkable and Back from a template or a proposal returns to it. Without
 * it this is the change request list, with its own `cr*` URL filters, exactly
 * as before.
 *
 * The list's `cr*` filters do not apply to Standard changes, but they stay in
 * the URL while there (unread), so switching back to "Change requests" finds
 * the list exactly as it was left; Standard changes' own parameters are
 * dropped on the way out. A pin of Standard changes ignores the `cr*` ones, so
 * it reads "Standard changes", not "Standard changes: 3 filters". A link in
 * the old two-sub-view form (`?view=catalogue|proposals`) is rewritten in
 * place to the current one.
 */
export default function ChangeRequestsArea(): JSX.Element {
  const [searchParams, setSearchParams] = useSearchParams();
  const view = changeRequestsView(searchParams);
  const legacy = isLegacyStandardChangesView(searchParams);

  useEffect(() => {
    if (legacy) {
      setSearchParams((prev) => canonicalStandardChangesSearch(prev), { replace: true });
    }
  }, [legacy, setSearchParams]);

  const select = (next: ChangeRequestsView): void =>
    setSearchParams((prev) =>
      next === "list" ? changeRequestListSearch(prev) : standardChangesSearch(prev, "templates"),
    );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={view}
          onChange={(_, next: ChangeRequestsView) => select(next)}
          aria-label="Change requests"
          variant="scrollable"
          allowScrollButtonsMobile
        >
          {SUB_VIEWS.map((v) => (
            <Tab key={v.value} value={v.value} label={v.label} sx={{ textTransform: "none" }} />
          ))}
        </Tabs>
      </Box>

      {view === "list" ? <ChangeRequestsTab /> : <StandardChangesTab show={standardChangesShow(searchParams)} />}
    </Box>
  );
}
