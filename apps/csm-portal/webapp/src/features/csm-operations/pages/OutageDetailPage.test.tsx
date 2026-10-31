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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import type { BeOutageDetail } from "@api/backend/types";

// The real client reads runtime config at module load; the page only needs
// BackendApiError for `instanceof`.
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({}),
}));

const OUTAGE: BeOutageDetail = {
  id: "out-1",
  number: "OUT0001",
  type: "outage",
  status: "in_progress",
  begin: "2026-03-01 10:00:00",
  end: null,
  duration: null,
  shortDescription: "Login errors",
  configurationItem: null,
  incident: null,
  affectedConfigurationItems: null,
  publishesToStatusPage: false,
  statusPageCloud: null,
  createdOn: "2026-03-01 10:00:00",
  createdBy: "jane",
  updatedOn: "2026-03-01 10:00:00",
  updatedBy: "jane",
  communicationCounts: { external: 0, internal: 0, additional: 0 },
};

const addMutateMock = vi.fn();
const recordViewMock = vi.fn();

vi.mock("@features/csm-operations/api/useOutages", () => ({
  useGetOutage: () => ({ data: OUTAGE, isLoading: false, isError: false }),
  usePatchOutage: () => ({ isPending: false, isError: false, error: null, mutate: vi.fn() }),
}));
vi.mock("@features/csm-operations/api/useOutageCommunications", () => ({
  useGetOutageCommunications: () => ({ data: { communications: [] } }),
  useAddOutageCommunication: () => ({ isPending: false, mutate: addMutateMock }),
}));
vi.mock("@features/csm-operations/components/EditOutageDialog", () => ({ default: () => null }));
vi.mock("@features/csm-operations/components/CloseOutageDialog", () => ({ default: () => null }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => vi.fn() }));
vi.mock("@hooks/useNormalizedIdParam", () => ({ useNormalizedIdParam: () => "out-1" }));
vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({ useRecordRecentView: () => recordViewMock }));

import OutageDetailPage from "@features/csm-operations/pages/OutageDetailPage";

function renderPage(): void {
  render(
    <MemoryRouter>
      <OutageDetailPage />
    </MemoryRouter>,
  );
}

describe("OutageDetailPage post communication", () => {
  it("marks Channel and Communication required, matching the backend's channel + body rule", () => {
    renderPage();

    expect(screen.getByRole("combobox", { name: /^channel$/i })).toBeRequired();
    expect(screen.getByRole("textbox", { name: /^communication$/i })).toBeRequired();
  });

  it("keeps Post disabled until the communication has text, then posts it on the chosen channel", () => {
    renderPage();
    const post = screen.getByRole("button", { name: /^post communication$/i });
    expect(post).toBeDisabled();

    fireEvent.change(screen.getByRole("textbox", { name: /^communication$/i }), { target: { value: "   " } });
    expect(post).toBeDisabled();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^channel$/i }));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: "Additional notes" }));
    fireEvent.change(screen.getByRole("textbox", { name: /^communication$/i }), { target: { value: " Investigating " } });
    expect(post).toBeEnabled();
    fireEvent.click(post);

    expect(addMutateMock).toHaveBeenCalledWith(
      { outageId: "out-1", payload: { channel: "additional", body: "Investigating" } },
      expect.anything(),
    );
  });
});
