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
import EngagementsStatCards from "@features/engagements/components/EngagementsStatCards";

vi.mock("@components/list-view/ListStatGrid", () => ({
  default: ({
    entityName,
    configs,
  }: {
    entityName: string;
    configs: { label: string }[];
  }) => (
    <div data-testid="stat-grid">
      <span>{entityName}</span>
      {configs.map((c) => (
        <span key={c.label}>{c.label}</span>
      ))}
    </div>
  ),
}));

describe("EngagementsStatCards", () => {
  it("renders stat grid with engagement stat labels", () => {
    render(
      <EngagementsStatCards
        stats={{ totalCount: 5, activeCount: 2, stateCount: [] } as never}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByTestId("stat-grid")).toBeInTheDocument();
    expect(screen.getByText("engagement statistics")).toBeInTheDocument();
    expect(screen.getByText("Outstanding Engagements")).toBeInTheDocument();
    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText("On Hold")).toBeInTheDocument();
  });
});
