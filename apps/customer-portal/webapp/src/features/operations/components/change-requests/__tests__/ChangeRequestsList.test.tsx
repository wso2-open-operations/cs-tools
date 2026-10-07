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
import ChangeRequestsList from "@features/operations/components/change-requests/ChangeRequestsList";
import { CHANGE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE } from "@features/operations/constants/operationsConstants";

describe("ChangeRequestsList", () => {
  it("renders empty state when there are no change requests", () => {
    render(
      <ChangeRequestsList changeRequests={[]} isLoading={false} isError={false} />,
    );
    expect(screen.getByText(CHANGE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE)).toBeInTheDocument();
  });

  it("lists a change request waiting in Authorize with its state", () => {
    render(
      <ChangeRequestsList
        changeRequests={[
          {
            id: "cr-authorize",
            number: "CHG0001234",
            title: "Rotate the gateway certificates",
            startDate: "2031-03-15 09:00:00",
            endDate: "2031-03-15 11:00:00",
            duration: null,
            hasServiceOutage: false,
            impact: null,
            state: { id: "-3", label: "Authorize" },
            type: null,
          } as never,
        ]}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("Rotate the gateway certificates")).toBeInTheDocument();
    expect(screen.getByText("Authorize")).toBeInTheDocument();
    expect(screen.queryByText(CHANGE_REQUESTS_LIST_EMPTY_DEFAULT_MESSAGE)).not.toBeInTheDocument();
  });
});
