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

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import ListFiltersPanel from "@components/list-view/ListFiltersPanel";
import { CHANGE_REQUEST_FILTER_DEFINITIONS } from "@features/operations/constants/operationsConstants";
import { resolveChangeRequestFilterListOptions } from "@features/operations/utils/operationsPages";

/**
 * The Change Requests page renders its filters through `ListFiltersPanel` with
 * `CHANGE_REQUEST_FILTER_DEFINITIONS` (not through `ChangeRequestsFilters`), so
 * this is the path the "Status" label has to reach.
 */
describe("Change requests filter panel (as ChangeRequestsPage renders it)", () => {
  const renderPanel = () =>
    render(
      <ListFiltersPanel
        filterDefinitions={CHANGE_REQUEST_FILTER_DEFINITIONS}
        filters={{}}
        resolveOptions={(def) =>
          resolveChangeRequestFilterListOptions(def, undefined)
        }
        onFilterChange={() => {}}
      />,
    );

  it("labels the state filter Status, and Impact is unchanged", () => {
    renderPanel();
    expect(document.getElementById("state-label")?.textContent).toBe("Status");
    expect(document.getElementById("impact-label")?.textContent).toBe("Impact");
    expect(screen.queryByText("State")).toBeNull();
  });

  it("keeps the #state select the e2e page object finds the filter by", () => {
    renderPanel();
    expect(document.getElementById("state")).not.toBeNull();
  });
});
