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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter } from "react-router";
import SmartAlertDetailModal from "@features/csm-cases/components/SmartAlertDetailModal";
import type { BeSmartAlertDetail } from "@api/backend/types";
import { useGetSmartAlert } from "@features/csm-cases/api/useSnLinkEntities";

vi.mock("@features/csm-cases/api/useSnLinkEntities", () => ({
  useGetAlert: vi.fn(),
  useGetSmartAlert: vi.fn(),
}));

const mockedUseGetSmartAlert = vi.mocked(useGetSmartAlert);

const SMART_ALERT: BeSmartAlertDetail = {
  id: "sa-1",
  alertId: "alert-1",
  sourceAlertId: "SRC-9001",
  alertStatus: "Open",
  windowStatus: "Active",
  severity: "Major",
  urgency: "High",
  impact: "High",
  category: "Availability",
  source: "Datadog",
  environment: "Production",
  resourceName: "api-gateway-1",
  shortDescription: "Elevated 5xx rate",
  details: "not json",
  monitorUrl: "https://monitor.example.com/alerts/SRC-9001",
  firedAt: "2026-07-01T00:00:00Z",
  receivedAt: "2026-07-01T00:01:00Z",
  fireCount: 3,
  incidentId: "incident-1",
};

function renderModal(onClose = vi.fn()): ReturnType<typeof render> {
  return render(
    <MemoryRouter>
      <SmartAlertDetailModal smartAlertId="sa-1" onClose={onClose} />
    </MemoryRouter>,
  );
}

describe("SmartAlertDetailModal", () => {
  it("shows a loading state while the smart alert is being fetched", () => {
    mockedUseGetSmartAlert.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    renderModal();
    expect(document.querySelector(".MuiCircularProgress-root")).toBeInTheDocument();
  });

  it("renders smart-alert fields and the fire count", () => {
    mockedUseGetSmartAlert.mockReturnValue({
      data: SMART_ALERT,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    renderModal();
    expect(screen.getByText("Smart alert · SRC-9001")).toBeInTheDocument();
    expect(screen.getByText("Major")).toBeInTheDocument();
    expect(screen.getByText("Elevated 5xx rate")).toBeInTheDocument();
    expect(screen.getByText("Datadog")).toBeInTheDocument();
    expect(screen.getByText("api-gateway-1")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("not json")).toBeInTheDocument();
  });

  it("shows a not-found state on a 404 (null data)", () => {
    mockedUseGetSmartAlert.mockReturnValue({
      data: null,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    renderModal();
    expect(screen.getByText(/could no longer be found/)).toBeInTheDocument();
  });

  it("links out to the monitoring tool and the linked incident", () => {
    mockedUseGetSmartAlert.mockReturnValue({
      data: SMART_ALERT,
      isLoading: false,
      isError: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any -- partial UseQueryResult stub
    } as any);
    renderModal();
    const monitorLink = screen.getByRole("link", { name: "Open in monitoring tool" });
    expect(monitorLink).toHaveAttribute("href", SMART_ALERT.monitorUrl);
    expect(monitorLink).toHaveAttribute("target", "_blank");
    expect(monitorLink).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getByRole("link", { name: "Open incident" })).toBeInTheDocument();
  });
});
