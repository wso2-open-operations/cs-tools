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

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { SalesforceResyncApplyResponse, SalesforceResyncPreview } from "@features/csm-admin/salesforce-resync/useSalesforceResync";

const postMock = vi.fn();

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => ({ post: postMock }),
}));

import { BackendApiError } from "@api/backend/client";
import SalesforceResyncDialog from "@features/csm-admin/salesforce-resync/SalesforceResyncDialog";

const ZERO_COUNTS = { inSync: 0, differs: 0, missingInCsm: 0, onlyInCsm: 0 };

function preview(overrides: Partial<SalesforceResyncPreview> = {}): SalesforceResyncPreview {
  return {
    target: { scope: "account", id: "00000000-0000-0000-0000-000000000000", sfId: "001000000000000AAA", name: "Acme Corp" },
    capped: false,
    cappedReason: null,
    components: [
      {
        key: "account",
        label: "Account",
        status: "differs",
        counts: { ...ZERO_COUNTS, differs: 1 },
        records: [
          {
            sfId: "001000000000000AAA",
            label: "Acme Corp",
            state: "differs",
            fields: [
              { field: "region", csmValue: null, sfValue: "EMEA", wouldTruncate: false },
              { field: "notes", csmValue: "old", sfValue: "x".repeat(500), wouldTruncate: true },
            ],
          },
        ],
      },
      {
        key: "opportunities",
        label: "Opportunities",
        status: "in_sync",
        counts: { ...ZERO_COUNTS, inSync: 3 },
        records: [],
      },
      {
        key: "invoices",
        label: "Invoices",
        status: "skipped",
        skippedReason: "Salesforce call cap reached",
        counts: ZERO_COUNTS,
        records: [],
      },
      {
        key: "contacts",
        label: "Contacts",
        status: "differs",
        counts: { ...ZERO_COUNTS, differs: 1, onlyInCsm: 1 },
        records: [
          { sfId: "003000000000000AAA", label: "Jane Doe", state: "differs", fields: [{ field: "email", csmValue: "a@example.com", sfValue: "jane.doe@example.com", wouldTruncate: false }] },
          { sfId: "003000000000000BBB", label: "John Roe", state: "only_in_csm", fields: [] },
        ],
      },
      {
        key: "memberships",
        label: "Memberships",
        status: "differs",
        counts: { ...ZERO_COUNTS, missingInCsm: 1 },
        records: [],
      },
    ],
    ...overrides,
  };
}

function renderDialog(onClose = vi.fn()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const view = render(
    <QueryClientProvider client={queryClient}>
      <SalesforceResyncDialog open scope="account" id="00000000-0000-0000-0000-000000000000" onClose={onClose} />
    </QueryClientProvider>,
  );
  return { ...view, queryClient, onClose };
}

const box = (name: string): HTMLInputElement => screen.getByRole("checkbox", { name: `Overwrite ${name} from Salesforce` });

beforeEach(() => {
  postMock.mockReset();
});

describe("SalesforceResyncDialog", () => {
  it("runs the preview on open and shows the progress text while it runs", async () => {
    postMock.mockReturnValue(new Promise(() => {}));
    renderDialog();
    expect(await screen.findByText(/can take up to a minute/i)).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock.mock.calls[0][0]).toBe("/salesforce/resync/preview");
    expect(postMock.mock.calls[0][1]).toEqual({ scope: "account", id: "00000000-0000-0000-0000-000000000000" });
    expect(screen.getByRole("dialog", { name: "Compare with Salesforce" })).toBeInTheDocument();
  });

  it("ticks differing components except contacts and memberships, and cannot tick in-sync or skipped ones", async () => {
    postMock.mockResolvedValue(preview());
    renderDialog();
    await screen.findByText(/Acme Corp:/);
    expect(box("Account")).toBeChecked();
    expect(box("Contacts")).not.toBeChecked();
    expect(box("Memberships")).not.toBeChecked();
    expect(box("Opportunities")).toBeDisabled();
    expect(box("Opportunities")).not.toBeChecked();
    expect(box("Invoices")).toBeDisabled();
    expect(screen.getByText("Salesforce call cap reached")).toBeInTheDocument();
    expect(screen.getAllByText("Changes who can access what, tick deliberately")).toHaveLength(2);
    expect(screen.getByRole("button", { name: "Overwrite selected (1)" })).toBeEnabled();
    fireEvent.click(box("Account"));
    expect(screen.getByRole("button", { name: "Overwrite selected (0)" })).toBeDisabled();
  });

  it("shows the capped warning with its reason", async () => {
    postMock.mockResolvedValue(preview({ capped: true, cappedReason: "Stopped after 500 Salesforce calls" }));
    renderDialog();
    expect(await screen.findByText("Stopped after 500 Salesforce calls")).toBeInTheDocument();
  });

  it("shows field diffs with (empty) placeholders, a truncation chip, and the only-in-CSM note", async () => {
    postMock.mockResolvedValue(preview());
    renderDialog();
    await screen.findByText(/Acme Corp:/);
    fireEvent.click(screen.getByRole("button", { name: /Show 1 record for Account/ }));
    expect(screen.getByText("region")).toBeInTheDocument();
    expect(screen.getByText("(empty)")).toBeInTheDocument();
    expect(screen.getByText("Would be cut")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Show 2 records for Contacts/ }));
    expect(screen.getByText("Only in CSM")).toBeInTheDocument();
    expect(screen.getByText(/overwriting never deletes anything/)).toBeInTheDocument();
  });

  it("caps long record lists behind a Show more control", async () => {
    const records = Array.from({ length: 60 }, (_, i) => ({
      sfId: `006000000000${String(i).padStart(3, "0")}AAA`,
      label: `Deal ${i}`,
      state: "missing_in_csm" as const,
      fields: [],
    }));
    postMock.mockResolvedValue(
      preview({
        components: [{ key: "opportunities", label: "Opportunities", status: "differs", counts: { ...ZERO_COUNTS, missingInCsm: 60 }, records }],
      }),
    );
    renderDialog();
    await screen.findByText(/Acme Corp:/);
    fireEvent.click(screen.getByRole("button", { name: /Show 60 records/ }));
    expect(screen.getByText("Deal 24")).toBeInTheDocument();
    expect(screen.queryByText("Deal 25")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Show 25 more/ }));
    expect(screen.getByText("Deal 49")).toBeInTheDocument();
  });

  it("confirms with the ticked components, then applies only the ticked keys and shows the result", async () => {
    const applied: SalesforceResyncApplyResponse = {
      components: [
        { key: "account", status: "applied", succeeded: 1, failed: 0, skippedStale: 2 },
        { key: "contacts", status: "failed", succeeded: 0, failed: 1, skippedStale: 0, errors: [{ sfId: "003000000000000AAA", message: "column too short" }] },
      ],
    };
    postMock.mockImplementation((path: string) =>
      Promise.resolve(path === "/salesforce/resync/preview" ? preview() : applied),
    );
    const { queryClient } = renderDialog();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    await screen.findByText(/Acme Corp:/);
    fireEvent.click(box("Contacts"));
    fireEvent.click(screen.getByRole("button", { name: "Overwrite selected (2)" }));

    const confirm = await screen.findByRole("dialog", { name: "Overwrite from Salesforce?" });
    expect(within(confirm).getByText(/Salesforce values replace the CSM values/)).toBeInTheDocument();
    expect(within(confirm).getByText("Account")).toBeInTheDocument();
    expect(within(confirm).getByText("Contacts")).toBeInTheDocument();
    expect(within(confirm).queryByText("Memberships")).not.toBeInTheDocument();
    expect(within(confirm).getByText(/Nothing is deleted/)).toBeInTheDocument();
    expect(within(confirm).getByText(/changed since the comparison are left alone/)).toBeInTheDocument();

    fireEvent.click(within(confirm).getByRole("button", { name: "Overwrite" }));
    await screen.findByText("Overwrite result");
    const applyCall = postMock.mock.calls.find((c) => c[0] === "/salesforce/resync/apply");
    expect(applyCall?.[1]).toEqual({
      scope: "account",
      id: "00000000-0000-0000-0000-000000000000",
      components: ["account", "contacts"],
    });
    expect(screen.getByText("Applied")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
    expect(screen.getByText(/2 records changed since the comparison and were left alone/)).toBeInTheDocument();
    expect(screen.getByText(/column too short/)).toBeInTheDocument();
    await waitFor(() => expect(invalidate).toHaveBeenCalled());
    expect(await screen.findByRole("button", { name: "Run comparison again" })).toBeInTheDocument();
  });

  it("shows the friendly 409 message", async () => {
    postMock.mockRejectedValue(new BackendApiError(409, "resync already running"));
    renderDialog();
    expect(
      await screen.findByText("Another comparison or overwrite is already running for this record, try again shortly."),
    ).toBeInTheDocument();
  });

  it("handles 403 politely and other errors with the backend message", async () => {
    postMock.mockRejectedValueOnce(new BackendApiError(403, "forbidden"));
    renderDialog();
    expect(await screen.findByText(/Only administrators can compare or overwrite/)).toBeInTheDocument();
  });

  it("shows the backend message for a 503", async () => {
    postMock.mockRejectedValue(new BackendApiError(503, "Salesforce is temporarily unavailable"));
    renderDialog();
    expect(await screen.findByText("Salesforce is temporarily unavailable")).toBeInTheDocument();
  });

  it("shows a try-again message on a client timeout", async () => {
    const timeout = new Error("timed out");
    timeout.name = "TimeoutError";
    postMock.mockRejectedValue(timeout);
    renderDialog();
    expect(await screen.findByText(/This took too long, try again/)).toBeInTheDocument();
  });

  it("passes a timeout signal to both calls and re-runs the preview on Refresh comparison", async () => {
    postMock.mockResolvedValue(preview());
    renderDialog();
    await screen.findByText(/Acme Corp:/);
    expect(postMock.mock.calls[0][2]?.signal).toBeInstanceOf(AbortSignal);
    fireEvent.click(screen.getByRole("button", { name: "Refresh comparison" }));
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));
  });
});
