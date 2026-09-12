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

import { Button } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";
import { useSearchParams } from "react-router";

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";
import { readWidgetTitleParam } from "@features/csm-dashboard/utils/widgetPreviewUrl";
import { useNavTransition } from "@hooks/useNavTransition";

/**
 * All-cases list — the shared issues view across every case type.
 *
 * `title` defaults to "Cases", the page's own real identity for a normal
 * left-nav visit, but is overridden by a dashboard widget's own
 * `displayName` when this page was reached via a `case`-resourceType
 * widget's tile click (see `WIDGET_RESOURCE_CONFIG.case.buildHref` in
 * `widgetResourceConfig.ts`, and `appendWidgetTitleParam`'s own doc comment)
 * — digiops-cs#2914: several dashboard widgets all drill through to this one
 * page, and a hardcoded "Cases" heading made it unclear which widget's
 * filtered result set was actually being shown.
 */
export default function CsmCasesPage(): JSX.Element {
  const navigate = useNavTransition();
  const [searchParams] = useSearchParams();
  const title = readWidgetTitleParam(searchParams) ?? "Cases";

  return (
    <CsmIssuesView
      title={title}
      entityNoun="cases"
      // Cases list defaults to support cases (`caseTypes: ["case"]`) on a
      // fresh visit, but, unlike the other issue-type pages (Operations/
      // Security Center/Engagements, which exist purely to be locked to one
      // type and hide the control), is the one unlocked, multi-type
      // `CsmIssuesView`: the type control is left visible and fully
      // changeable — `defaultCaseTypes` only seeds the initial selection
      // when the URL carries no `types` param at all; picking a different
      // type (or clearing back to no selection, which falls through to
      // "every type" via `CsmIssuesView`'s own `ALL_CASE_TYPES` fallback)
      // genuinely narrows/broadens the results, per digiops-cs#2907.
      // `lockedFilters.caseTypes` is kept in lockstep purely so the severity
      // filter/column stay visible (that hint is keyed off `lockedFilters`,
      // not the live selection or `defaultCaseTypes` — see
      // `CsmIssuesView`'s own `showSeverityFilter`).
      defaultCaseTypes={["case"]}
      lockedFilters={{ caseTypes: ["case"] }}
      enableColumnCustomization
      columnsViewId="cases"
      actions={
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => navigate("/cases/new")}
        >
          Create case
        </Button>
      }
    />
  );
}
