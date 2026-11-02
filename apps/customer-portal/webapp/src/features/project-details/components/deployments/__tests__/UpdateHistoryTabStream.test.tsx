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
import { beforeEach, describe, expect, it, vi } from "vitest";
import UpdateHistoryTab from "@features/project-details/components/deployments/UpdateHistoryTab";
import { useGetProductUpdateLevels } from "@features/updates/api/useGetProductUpdateLevels";

vi.mock("@features/updates/api/useGetProductUpdateLevels");

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

// Streams as GET /updates/product-update-levels returns them: API Manager's
// control plane, traffic manager and universal gateway number their levels on
// their own, apart from the product's.
const streams = [
  {
    productName: "wso2am",
    productUpdateLevels: [
      { productBaseVersion: "4.6.0", channel: "full", updateLevels: [53, 54, 55] },
    ],
  },
  {
    productName: "wso2am-acp",
    productUpdateLevels: [
      { productBaseVersion: "4.6.0", channel: "full", updateLevels: [54, 55, 56] },
    ],
  },
  {
    productName: "wso2am-tm",
    productUpdateLevels: [
      { productBaseVersion: "4.5.0", channel: "full", updateLevels: [40, 41, 42] },
    ],
  },
];

function offeredLevels(): string[] {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: /Update Level/i }));
  return screen
    .getAllByRole("option", { hidden: true })
    .map((o) => o.textContent ?? "")
    .filter((text) => /^\d+$/.test(text));
}

describe("UpdateHistoryTab update-level streams", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useGetProductUpdateLevels).mockReturnValue({
      data: streams,
      isLoading: false,
    } as unknown as ReturnType<typeof useGetProductUpdateLevels>);
  });

  const baseProps = {
    updates: [],
    productVersion: "4.6.0",
    isLoading: false,
    onSaveUpdates: vi.fn(),
  };

  it("offers the levels of the profile's own stream", () => {
    render(
      <UpdateHistoryTab
        {...baseProps}
        productName="wso2am-acp"
        fallbackProductName="wso2am"
      />,
    );

    // wso2am-acp runs to 56; wso2am stops at 55.
    expect(offeredLevels()).toEqual(["56", "55", "54"]);
  });

  it("offers the product's levels when no stream is named for the profile", () => {
    render(
      <UpdateHistoryTab
        {...baseProps}
        productName="wso2am"
        fallbackProductName="wso2am"
      />,
    );

    expect(offeredLevels()).toEqual(["55", "54", "53"]);
  });

  it("falls back to the product's stream when the profile's stream has nothing for this version", () => {
    // wso2am-tm only has 4.5.0, so a 4.6.0 traffic manager reads wso2am's.
    render(
      <UpdateHistoryTab
        {...baseProps}
        productName="wso2am-tm"
        fallbackProductName="wso2am"
      />,
    );

    expect(offeredLevels()).toEqual(["55", "54", "53"]);
  });

  it("does not fall back to another version's levels", () => {
    render(
      <UpdateHistoryTab
        {...baseProps}
        productVersion="9.9.9"
        productName="wso2am-acp"
        fallbackProductName="wso2am"
      />,
    );

    expect(offeredLevels()).toEqual([]);
  });

  it("keeps working without a fallback name", () => {
    render(<UpdateHistoryTab {...baseProps} productName="wso2am-acp" />);

    expect(offeredLevels()).toEqual(["56", "55", "54"]);
  });
});
