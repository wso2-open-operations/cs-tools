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
import ListItems from "@components/list-view/ListItems";
import type { CaseListItem } from "@features/support/types/cases";

const caseItem = {
  id: "case-1",
  number: "CS0001",
  title: "Test case",
  status: { id: "1", label: "Open" },
  severity: { id: "10", label: "S1" },
  createdOn: "2026-01-01",
  description: "Details",
  assignedEngineer: null,
  project: { id: "p1", label: "Project" },
  issueType: null,
  deployedProduct: null,
  deployment: null,
} satisfies CaseListItem;

describe("ListItems", () => {
  it("renders case card title", () => {
    render(<ListItems cases={[caseItem]} isLoading={false} />);
    expect(screen.getByText("Test case")).toBeInTheDocument();
  });
});
