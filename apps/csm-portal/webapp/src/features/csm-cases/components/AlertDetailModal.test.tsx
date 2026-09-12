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
import { MemoryRouter } from "react-router";
import AlertDetailModal from "@features/csm-cases/components/AlertDetailModal";
import type { BeAlertDetail } from "@api/backend/types";
import { useGetAlert } from "@features/csm-cases/api/useSnLinkEntities";

vi.mock("@features/csm-cases/api/useSnLinkEntities", () => ({
  useGetAlert: vi.fn(),
  useGetSmartAlert: vi.fn(),
}));

const mockedUseGetAlert = vi.mocked(useGetAlert);

const ALERT: BeAlertDetail = {
  id: "alert-1",
  number: "ALT0001",
  environment: "Production",
  metricName: "cpu.usage",
  source: "Prometheus",
  category: "Performance",
  severity: "Critical",
  description: '{"threshold":90,"observed":97}',
  incidentId: "incident-1",
  createdOn: "2026-07-01T00:00:00Z",
};

function renderModal(onClose = vi.fn()): ReturnType<typeof render> {
  return render(
    <MemoryRouter>
      <AlertDetailModal alertId="alert-1" onClose={onClose} />
    </MemoryRouter>,
  );
}

describe("AlertDetailModal", () => {
  it("shows a loading state while the alert is being fetched", () => {
    mockedUseGetAlert.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    renderModal();
    expect(document.querySelector(".MuiCircularProgress-root")).toBeInTheDocument();
  });

  it("renders alert fields, including a pretty-printed JSON description", () => {
    mockedUseGetAlert.mockReturnValue({
      data: ALERT,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    renderModal();
    expect(screen.getByText("Alert · ALT0001")).toBeInTheDocument();
    expect(screen.getByText("Critical")).toBeInTheDocument();
    expect(screen.getByText("Prometheus")).toBeInTheDocument();
    expect(screen.getByText("Production")).toBeInTheDocument();
    // JSON.stringify(..., null, 2) pretty-prints across lines; assert on the
    // rendered <pre> block's combined text content rather than one exact line.
    const description = screen.getByText(/"threshold": 90/);
    expect(description.textContent).toContain('"observed": 97');
  });

  it("falls back to the raw string when the opaque description is not valid JSON", () => {
    mockedUseGetAlert.mockReturnValue({
      data: { ...ALERT, description: "plain text, not json" },
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    renderModal();
    expect(screen.getByText("plain text, not json")).toBeInTheDocument();
  });

  it("shows a not-found state on a 404 (null data)", () => {
    mockedUseGetAlert.mockReturnValue({
      data: null,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    renderModal();
    expect(screen.getByText(/could no longer be found/)).toBeInTheDocument();
  });

  it("navigates to the linked incident and closes the modal", () => {
    mockedUseGetAlert.mockReturnValue({
      data: ALERT,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    const onClose = vi.fn();
    renderModal(onClose);
    fireEvent.click(screen.getByRole("link", { name: "Open incident" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("calls onClose from the Close button", () => {
    mockedUseGetAlert.mockReturnValue({
      data: ALERT,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    const onClose = vi.fn();
    renderModal(onClose);
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });
});
