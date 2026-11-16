// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { AlertMuteRuleInput } from "@features/csm-operations/types/alertMuteRules";
import { createFakeAlertMuteRulesApi } from "@features/csm-operations/api/alertMuteRulesFakeApi.test-utils";
import AlertMuteRulesTab from "@features/csm-operations/components/AlertMuteRulesTab";

const backend = vi.hoisted(() => ({ current: null as ReturnType<typeof createFakeAlertMuteRulesApi> | null }));
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => backend.current!.api,
}));

const H = 3_600_000;

function rule(name: string, startOffsetH: number, lengthH: number, extra: Partial<AlertMuteRuleInput> = {}): AlertMuteRuleInput {
  const start = Date.now() + startOffsetH * H;
  return {
    name,
    description: `About ${name}`,
    timeZone: "Asia/Colombo",
    startOn: new Date(start).toISOString(),
    endOn: new Date(start + lengthH * H).toISOString(),
    conditions: [
      { field: "service", value: "payments" },
      { field: "environment", value: "prod" },
    ],
    ...extra,
  };
}

function renderTab(): void {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/operations/alert-mute-rules"]}>
        <Routes>
          <Route path="/operations/alert-mute-rules" element={<AlertMuteRulesTab />} />
          <Route path="/operations/alert-mute-rules/:id" element={<div>Detail page</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("AlertMuteRulesTab", () => {
  beforeEach(() => {
    backend.current = createFakeAlertMuteRulesApi();
  });

  it("shows the empty state", async () => {
    renderTab();
    expect(await screen.findByText(/No alert mute rules yet/)).toBeInTheDocument();
  });

  it("counts rules per state and filters by it, filing early-ended rules under Expired", async () => {
    const b = backend.current!;
    b.seed(rule("Running", -1, 2));
    b.seed(rule("Later", 5, 1));
    b.seed(rule("Past", -10, 2));
    b.seed(rule("Stopped", -1, 4), { endedOn: new Date().toISOString(), endedBy: "a@example.test" });
    renderTab();

    expect(await screen.findByText("Running")).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "All (4)" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Active (1)" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Scheduled (1)" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Expired (2)" }));
    await waitFor(() => expect(screen.queryByText("Running")).not.toBeInTheDocument());
    expect(screen.getByText("Past")).toBeInTheDocument();
    expect(screen.getByText("Stopped")).toBeInTheDocument();
    expect(screen.getByText("Ended early")).toBeInTheDocument();
  }, 30_000);

  it("shows each rule's conditions", async () => {
    backend.current!.seed(
      rule("Site24x7 only", -1, 2, {
        conditions: [
          { field: "service", value: "payments" },
          { field: "environment", value: "prod" },
          { field: "source", value: "Site24x7" },
        ],
      }),
    );
    renderTab();
    expect(await screen.findByText("service = payments, environment = prod, source = Site24x7")).toBeInTheDocument();
  });

  it("searches on the backend by condition value", async () => {
    const b = backend.current!;
    b.seed(rule("DB upgrade", -1, 2));
    b.seed(rule("Cert rotation", -1, 2, { conditions: [{ field: "service", value: "gateway" }, { field: "environment", value: "prod" }] }));
    renderTab();
    await screen.findByText("DB upgrade");

    fireEvent.change(screen.getByRole("textbox", { name: "Search alert mute rules" }), { target: { value: "gateway" } });
    await waitFor(() => expect(screen.queryByText("DB upgrade")).not.toBeInTheDocument());
    expect(screen.getByText("Cert rotation")).toBeInTheDocument();
  }, 30_000);

  it("ends an active rule from its ⋮ menu after confirming", async () => {
    const running = backend.current!.seed(rule("Running", -1, 2));
    renderTab();
    await screen.findByText("Running");
    fireEvent.click(screen.getByRole("button", { name: "Actions for Running" }));
    const menu = screen.getByRole("menu");
    expect(within(menu).queryByText("Edit")).not.toBeInTheDocument();
    fireEvent.click(within(menu).getByText("End now"));

    const dialog = screen.getByRole("dialog", { name: /End "Running" now\?/ });
    fireEvent.click(within(dialog).getByRole("button", { name: "End now" }));

    expect(await screen.findByText("Ended early")).toBeInTheDocument();
    expect(backend.current!.all().find((r) => r.id === running.id)?.endedBy).toBe("engineer@example.test");
  }, 30_000);

  it("opens a rule when its row is clicked", async () => {
    backend.current!.seed(rule("Running", -1, 2));
    renderTab();
    await screen.findByText("Running");
    fireEvent.click(screen.getByRole("row", { name: "View alert mute rule Running" }));
    expect(await screen.findByText("Detail page")).toBeInTheDocument();
  });
});
