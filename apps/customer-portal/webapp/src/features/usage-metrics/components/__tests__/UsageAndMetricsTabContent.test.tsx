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

import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";
import UsageAndMetricsTabContent from "@features/usage-metrics/components/UsageAndMetricsTabContent";

// Mutable availability the mocked stats hook reads (hoisted for vi.mock).
const availability = vi.hoisted(() => ({ live: 0, upload: 0 }));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchAll: () => ({
    data: [{ id: "dep-1", name: "Production", productCount: 1 }],
    isLoading: false,
    isError: false,
    isSuccess: true,
    status: "success",
  }),
}));

// DataSource.API_CALL === 1, DataSource.FILE_UPLOAD === 2.
vi.mock(
  "@features/project-details/api/usePostProjectInstancesUsagesStats",
  () => ({
    default: (_projectId: string | undefined, payload: { filters: { dataSource?: number } }) => {
      const total =
        payload?.filters?.dataSource === 1 ? availability.live : availability.upload;
      return { data: { stats: {}, totalRecords: total, startDate: "", endDate: "" }, isLoading: false };
    },
  }),
);

vi.mock(
  "@features/usage-metrics/components/UsageEnvironmentProductsPanel",
  () => ({
    default: () => <div data-testid="usage-environment-products-panel" />,
  }),
);

vi.mock("@features/usage-metrics/components/DeploymentUsageUploadDialog", () => ({
  default: () => null,
}));

function renderTab() {
  return render(
    <MemoryRouter initialEntries={["/projects/p1"]}>
      <Routes>
        <Route path="/projects/:projectId" element={<UsageAndMetricsTabContent />} />
      </Routes>
    </MemoryRouter>,
  );
}

function dataSourceGroup(): HTMLElement | null {
  return screen.queryByRole("group", { name: "data source filter" });
}

describe("UsageAndMetricsTabContent data source toggle", () => {
  beforeEach(() => {
    availability.live = 0;
    availability.upload = 0;
  });

  it("hides the toggle when neither source has data", () => {
    renderTab();
    expect(dataSourceGroup()).not.toBeInTheDocument();
  });

  it("shows only Live when only live data exists", () => {
    availability.live = 5;
    renderTab();
    const group = dataSourceGroup();
    expect(group).toBeInTheDocument();
    expect(within(group!).getByRole("button", { name: "Live" })).toBeInTheDocument();
    expect(within(group!).queryByRole("button", { name: "Upload" })).not.toBeInTheDocument();
  });

  it("shows only Upload when only upload data exists", () => {
    availability.upload = 5;
    renderTab();
    const group = dataSourceGroup();
    expect(group).toBeInTheDocument();
    expect(within(group!).getByRole("button", { name: "Upload" })).toBeInTheDocument();
    expect(within(group!).queryByRole("button", { name: "Live" })).not.toBeInTheDocument();
  });

  it("shows both when both sources have data", () => {
    availability.live = 5;
    availability.upload = 5;
    renderTab();
    const group = dataSourceGroup();
    expect(group).toBeInTheDocument();
    expect(within(group!).getByRole("button", { name: "Live" })).toBeInTheDocument();
    expect(within(group!).getByRole("button", { name: "Upload" })).toBeInTheDocument();
  });
});
