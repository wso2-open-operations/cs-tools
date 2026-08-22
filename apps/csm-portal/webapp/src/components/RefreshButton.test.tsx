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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import RefreshButton from "@components/RefreshButton";

describe("RefreshButton", () => {
  it("does not show the 'Last refreshed' hint on initial load, even when updatedAt is already set", () => {
    const onRefresh = vi.fn();
    render(
      <RefreshButton
        onRefresh={onRefresh}
        isFetching={false}
        updatedAt={Date.now()}
        label="Refresh"
      />,
    );

    expect(screen.queryByText(/last refreshed/i)).not.toBeInTheDocument();
  });

  it("shows the 'Last refreshed' hint after the user manually clicks refresh, and calls onRefresh", () => {
    const onRefresh = vi.fn();
    render(
      <RefreshButton
        onRefresh={onRefresh}
        isFetching={false}
        updatedAt={Date.now()}
        label="Refresh"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));

    expect(onRefresh).toHaveBeenCalledTimes(1);
    expect(screen.getByText(/last refreshed/i)).toBeInTheDocument();
  });

  it("does not show the hint after a click if updatedAt is still unset", () => {
    const onRefresh = vi.fn();
    render(
      <RefreshButton onRefresh={onRefresh} isFetching={false} label="Refresh" />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));

    expect(onRefresh).toHaveBeenCalledTimes(1);
    expect(screen.queryByText(/last refreshed/i)).not.toBeInTheDocument();
  });
});
