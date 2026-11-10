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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useParams } from "react-router";
import type { JSX } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { clearUserPreferredTimeZone, setUserPreferredTimeZone } from "@utils/dateTime";
import { createFakeAlertMuteRulesApi } from "@features/csm-operations/api/alertMuteRulesFakeApi.test-utils";
import AlertMuteRuleFormPage from "@features/csm-operations/pages/AlertMuteRuleFormPage";

const backend = vi.hoisted(() => ({ current: null as ReturnType<typeof createFakeAlertMuteRulesApi> | null }));
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => backend.current!.api,
}));
vi.mock("@api/useSearchItServices", () => ({
  useSearchItServices: () => ({ data: [{ id: "svc-1", name: "Payments" }], isFetching: false, isError: false }),
}));

function Detail(): JSX.Element {
  return <div>{`Detail ${useParams().id}`}</div>;
}

function renderAt(path: string): void {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/operations/alert-mute-rules/new" element={<AlertMuteRuleFormPage />} />
          <Route path="/operations/alert-mute-rules/:id/edit" element={<AlertMuteRuleFormPage />} />
          <Route path="/operations/alert-mute-rules/:id" element={<Detail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("AlertMuteRuleFormPage", () => {
  beforeEach(() => {
    backend.current = createFakeAlertMuteRulesApi();
    setUserPreferredTimeZone("Asia/Colombo");
  });
  afterEach(() => clearUserPreferredTimeZone());

  it("is laid out like a mute rule: details, schedule and what to mute, with no recurrence or target alarms", () => {
    renderAt("/operations/alert-mute-rules/new");
    expect(screen.getByRole("heading", { name: "Alert mute rule details" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Schedule" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "What to mute" })).toBeInTheDocument();
    expect(screen.queryByText(/recurring/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/target alarms/i)).not.toBeInTheDocument();
  }, 30_000);

  it("defaults to 1 h from Create, with no date or time to pick, and offers 30 min, 2 h and Custom", () => {
    renderAt("/operations/alert-mute-rules/new");
    expect(screen.getByRole("button", { name: "1 h" })).toHaveAttribute("aria-pressed", "true");
    for (const name of ["30 min", "2 h", "Custom"]) {
      expect(screen.getByRole("button", { name })).toHaveAttribute("aria-pressed", "false");
    }
    expect(screen.queryByRole("combobox", { name: "Time zone" })).not.toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Date" })).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/from when you create the rule until about/);
  }, 30_000);

  it("shows the time zone and date and time pickers only for Custom", () => {
    renderAt("/operations/alert-mute-rules/new");
    fireEvent.click(screen.getByRole("button", { name: "Custom" }));
    // The time zone defaults to the user's.
    expect(screen.getByRole("combobox", { name: "Time zone" })).toHaveValue("(UTC+05:30) Asia/Colombo");
    expect(screen.getAllByRole("group", { name: "Date" })).toHaveLength(2);
    fireEvent.click(screen.getByRole("button", { name: "Create alert mute rule" }));
    expect(screen.getByText("Enter a start date and time.")).toBeInTheDocument();
    expect(screen.getByText("Enter an end date and time.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "2 h" }));
    expect(screen.queryByRole("combobox", { name: "Time zone" })).not.toBeInTheDocument();
    expect(screen.queryByText("Enter a start date and time.")).not.toBeInTheDocument();
  }, 30_000);

  it("creates a quick 30 min rule starting at Create", async () => {
    renderAt("/operations/alert-mute-rules/new");
    fireEvent.change(screen.getByRole("textbox", { name: "Alert mute rule name" }), { target: { value: "Quick mute" } });
    fireEvent.click(screen.getByRole("button", { name: "30 min" }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Service" }));
    fireEvent.click(await screen.findByRole("option", { name: "Payments" }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Environment" }));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: "Production" }));

    const before = Date.now();
    fireEvent.click(screen.getByRole("button", { name: "Create alert mute rule" }));
    await screen.findByText(/^Detail /, undefined, { timeout: 5_000 });

    const [created] = backend.current!.all();
    expect(created.conditions).toEqual([
      { field: "service", value: "Payments" },
      { field: "environment", value: "production" },
    ]);
    expect(Date.parse(created.startOn)).toBeGreaterThanOrEqual(before - 1);
    expect(Date.parse(created.endOn) - Date.parse(created.startOn)).toBe(30 * 60_000);
  }, 30_000);

  it("counts characters and marks what is required", () => {
    renderAt("/operations/alert-mute-rules/new");
    const name = screen.getByRole("textbox", { name: "Alert mute rule name" });
    expect(name).toBeRequired();
    expect(screen.getByRole("textbox", { name: "Description (optional)" })).not.toBeRequired();
    fireEvent.change(name, { target: { value: "DB upgrade" } });
    expect(screen.getByText(/Character count: 10\/255/)).toBeInTheDocument();
  }, 30_000);

  it("refuses to create without a name and condition values", () => {
    renderAt("/operations/alert-mute-rules/new");
    fireEvent.click(screen.getByRole("button", { name: "Create alert mute rule" }));
    expect(screen.getByText("Enter a name.")).toBeInTheDocument();
    expect(screen.getByText("Pick a service.")).toBeInTheDocument();
    expect(screen.getByText("Pick an environment.")).toBeInTheDocument();
  }, 30_000);

  it("has a Service picker and an Environment list, then adds other fields below as key/value rows", () => {
    renderAt("/operations/alert-mute-rules/new");
    expect(screen.getByRole("combobox", { name: "Service" })).toBeRequired();
    expect(screen.getByRole("combobox", { name: "Environment" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Remove (service|environment) condition/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Add condition" }));
    const removeSource = screen.getByRole("button", { name: "Remove source condition" });
    expect(removeSource).toBeEnabled();
    fireEvent.change(screen.getByRole("combobox", { name: "source value" }), { target: { value: "Site24x7" } });
    expect(screen.getByRole("combobox", { name: "source value" })).toHaveValue("Site24x7");

    fireEvent.click(removeSource);
    expect(screen.queryByRole("combobox", { name: "source value" })).not.toBeInTheDocument();
  }, 30_000);

  it("edits a scheduled rule and keeps its times", async () => {
    const start = Date.now() + 24 * 3_600_000;
    const rule = backend.current!.seed({
      name: "DB upgrade",
      description: "",
      timeZone: "Asia/Colombo",
      startOn: new Date(start - (start % 60_000)).toISOString(),
      endOn: new Date(start - (start % 60_000) + 2 * 3_600_000).toISOString(),
      conditions: [
        { field: "service", value: "payments" },
        { field: "environment", value: "prod" },
        { field: "source", value: "Site24x7" },
      ],
    });
    renderAt(`/operations/alert-mute-rules/${rule.id}/edit`);

    const name = await screen.findByRole("textbox", { name: "Alert mute rule name" });
    expect(name).toHaveValue("DB upgrade");
    fireEvent.change(name, { target: { value: "Payments DB upgrade" } });
    fireEvent.change(screen.getByRole("textbox", { name: "Description (optional)" }), {
      target: { value: "Minor version bump." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));

    expect(await screen.findByText(`Detail ${rule.id}`, undefined, { timeout: 5_000 })).toBeInTheDocument();
    const saved = backend.current!.all().find((r) => r.id === rule.id)!;
    expect(saved).toMatchObject({ name: "Payments DB upgrade", description: "Minor version bump." });
    expect(saved.conditions).toEqual(rule.conditions);
    expect(saved.startOn).toBe(rule.startOn);
    expect(saved.endOn).toBe(rule.endOn);
  }, 30_000);
});
