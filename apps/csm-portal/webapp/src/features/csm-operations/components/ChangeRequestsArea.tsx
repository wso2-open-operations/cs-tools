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
import { type JSX } from "react";
import { useSearchParams } from "react-router";
import ChangeRequestsTab from "@features/csm-operations/components/ChangeRequestsTab";
import StandardChangesTab from "@features/csm-operations/components/StandardChangesTab";
import {
  CHANGE_REQUESTS_VIEW_PARAM,
  changeRequestsView,
  STANDARD_CHANGES_VIEW_LABELS,
  type ChangeRequestsView,
} from "@features/csm-operations/utils/standardChanges";

const SUB_VIEWS: ReadonlyArray<{ value: ChangeRequestsView; label: string }> = [
  { value: "list", label: "Change requests" },
  { value: "catalogue", label: STANDARD_CHANGES_VIEW_LABELS.catalogue },
  { value: "proposals", label: STANDARD_CHANGES_VIEW_LABELS.proposals },
];

/**
 * Operations > Change requests. Standard changes are templated change requests,
 * so they are not a tab of their own beside this one: they are two sub-views of
 * it, next to the change request list -- the standard change catalogue and the
 * proposals to the CAB. The sub-view is the `?view=` query parameter
 * (`changeRequestsView`), so each is linkable and Back from a template or a
 * proposal returns to it. Without the parameter this is the change request list,
 * with its own `cr*` URL filters, exactly as before the sub-views existed.
 *
 * Switching sub-view only sets or drops `view`; the list's own filter
 * parameters stay in the URL, so going back to the list finds them as they were.
 */
export default function ChangeRequestsArea(): JSX.Element {
  const [searchParams, setSearchParams] = useSearchParams();
  const view = changeRequestsView(searchParams);

  const select = (next: ChangeRequestsView): void =>
    setSearchParams((prev) => {
      const params = new URLSearchParams(prev);
      if (next === "list") params.delete(CHANGE_REQUESTS_VIEW_PARAM);
      else params.set(CHANGE_REQUESTS_VIEW_PARAM, next);
      return params;
    });

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

      {view === "list" ? <ChangeRequestsTab /> : <StandardChangesTab view={view} />}
    </Box>
  );
}
