/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { CaseAlertSettings as Settings } from "../types";
// vi.mock below is hoisted above this import, so the section gets the mocks.
import CaseAlertSettings from "./CaseAlertSettings";

const settings: { data?: Settings } = {};
const noop = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const saveValues = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const deleteValue = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };

vi.mock("../api/useCaseAlertSettings", () => ({
  useCaseAlertSettings: () => ({ data: settings.data, isLoading: false, isError: false }),
  useSaveCaseAlertRoom: () => noop,
  useSetCaseAlertWebhook: () => noop,
  useTestCaseAlertRoom: () => noop,
  useDeleteCaseAlertRoom: () => noop,
  useSaveCaseAlertRoute: () => noop,
  useDeleteCaseAlertRoute: () => noop,
  useSaveCaseAlertValues: () => saveValues,
  useDeleteCaseAlertValue: () => deleteValue,
}));

const ABTS_ID = "33333333-3333-4333-8333-333333333333";
const HOOK = "https://chat.googleapis.com/v1/spaces/AAAAtest01/messages?key=k&token=t";
const AT = "2026-10-13T10:00:00Z";

function make(over: Partial<Settings> = {}): Settings {
  return {
    canManage: { view: true, roomLadders: ["CRE", "SRE"], routeLadders: ["CRE", "SRE"] },
    environment: "staging",
    webhooksEnabled: true,
    revision: { saved: 4 },
    rooms: [],
    routes: [],
    values: [
      { id: ABTS_ID, ladder: "CRE", key: "teams.abts", value: ["atlas", "castor"], updatedBy: "a@b", updatedOn: AT },
      {
        id: "44444444-4444-4444-8444-444444444444",
        ladder: "CRE",
        key: "heads.cre",
        value: { name: "Nimal Perera", email: "nimal@example.test" },
        updatedBy: "a@b",
        updatedOn: AT,
      },
      {
        id: "66666666-6666-4666-8666-666666666666",
        ladder: "SRE",
        key: "timing.interval",
        value: "5m",
        updatedBy: "a@b",
        updatedOn: AT,
      },
    ],
    running: {
      reportedAt: AT,
      mode: "shadow",
      ladders: {
        CRE: {
          "teams.abts": { value: ["atlas"], source: "file" },
          "trigger.priorities": { value: ["S0", "S1"], source: "file" },
          "teams.teamLeadsToCall": { value: 3, source: "file" },
          onLeave: { value: "skip", source: "file" },
          "acknowledgement.requireBoth": { value: true, source: "saved" },
          // The file leaves this head unnamed: nothing to show or copy.
          "heads.cs": { value: { name: "", email: "", phone: "+94770000000" }, source: "file" },
          // A key the page doesn't manage is neither shown nor copied.
          chat: { value: { webhookUrl: HOOK }, source: "file" },
        },
        SRE: { "timing.interval": { value: "5m", source: "saved" } },
      },
      fileOnly: {
        CRE: {
          enabled: true,
          channel: "both",
          "safety.maxCallsPerLadder": 32,
          "safety.maxLevel": 3,
          "safety.allowedNumbers": 2,
          "safety.callWithoutVerifiedLeads": false,
          "safety.callHeadsWithoutLowerTiers": true,
          "phones.source": "profile",
        },
        SRE: {
          enabled: false,
          channel: "log",
          "safety.maxCallsPerLadder": 0,
          "safety.allowedNumbers": 0,
          "safety.callWithoutVerifiedLeads": true,
          "safety.callHeadsWithoutLowerTiers": false,
          "phones.source": "none",
        },
      },
    },
    ...over,
  };
}

function row(name: string): HTMLElement {
  const cell = screen.getByText(name);
  const tr = cell.closest("tr");
  if (!tr) throw new Error(`no row for ${name}`);
  return tr;
}

/** A row's value in the file-only group. */
function fileValue(group: HTMLElement, label: string): string | null | undefined {
  return within(group).getByText(label).closest("tr")?.querySelector(".cas-val")?.textContent;
}

/** Presses the family's Submit and waits until it has saved everything. */
async function submit(family = "CRE"): Promise<void> {
  fireEvent.click(screen.getByRole("button", { name: `Submit ${family} changes` }));
  await waitFor(() =>
    expect(screen.getByRole("region", { name: `${family} changes to submit` })).toHaveTextContent(
      "No changes to submit.",
    ),
  );
}

/** Opens a row's edit dialog; the row is looked up first, as the dialog
 *  repeats its label. */
function openEdit(name: string): { row: HTMLElement; edit: HTMLElement; dialog: HTMLElement } {
  const r = row(name);
  const edit = within(r).getByRole("button", { name: "Edit" });
  edit.focus();
  fireEvent.click(edit);
  return { row: r, edit, dialog: screen.getByRole("dialog", { name }) };
}

const OPEN_KEY = "csm-ts.alert-settings.open";
const GROUPS = ["Who gets alerts", "When a ladder starts", "How it climbs", "Heads", "Chat messages", "other"];

/** Every section open, as for a person who opened them all before: most of
 *  these tests work inside them. */
function openEverything(): void {
  const ids = ["CRE", "SRE", "SME"].flatMap((l) => [
    `${l}.ladder`,
    `${l}.file`,
    ...GROUPS.map((g) => `${l}.ladder.${g}`),
  ]);
  window.localStorage.setItem(OPEN_KEY, JSON.stringify(ids));
}

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
  openEverything();
  settings.data = make();
});

describe("CaseAlertLadderSettings", () => {
  it("shows one value per row: what's in use, where it came from, and a saved one not in use yet", () => {
    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.getByRole("region", { name: "CRE ladder settings" })).toBeInTheDocument();
    expect(screen.getByText(/in shadow mode\. In shadow mode it still uses its file/)).toBeInTheDocument();
    for (const group of ["Who gets alerts", "When a ladder starts", "How it climbs", "Heads"]) {
      expect(screen.getByRole("region", { name: group })).toBeInTheDocument();
    }
    // SRE's chat messages are not on the CRE chain.
    expect(screen.queryByRole("region", { name: "Chat messages" })).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Saved here" })).not.toBeInTheDocument();

    const abts = row("ABT teams");
    expect(within(abts).getByText("atlas")).toBeInTheDocument();
    expect(within(abts).getByText("from the file")).toBeInTheDocument();
    expect(within(abts).getByText(/^Saved:/)).toHaveTextContent(
      "Saved: atlas, castor · used after the alert service restarts",
    );
    expect(within(row("Priorities")).getByText("S0, S1")).toBeInTheDocument();
    expect(within(row("Priorities")).queryByText(/^Saved:/)).not.toBeInTheDocument();
    expect(within(row("Team leads to call")).getByText("3")).toBeInTheDocument();
    expect(within(row("People on leave")).getByText("Skip them")).toBeInTheDocument();
    expect(within(row("Needs both to stop")).getByText("Yes")).toBeInTheDocument();
    expect(within(row("Needs both to stop")).getByText("saved here")).toBeInTheDocument();
    // Saved, but the alert service hasn't reported a value for it.
    expect(within(row("Head of CRE")).getByText(/^Saved:/)).toHaveTextContent(
      "Saved: Nimal Perera <nimal@example.test> · used after the alert service restarts",
    );
    expect(within(row("Head of CS")).getByText(/not named/)).toBeInTheDocument();

    expect(document.body.textContent).not.toContain("+9477");
    expect(document.body.textContent).not.toContain("token=");
    expect(document.body.textContent).not.toContain("chat.googleapis.com");
    expect(document.body.textContent).not.toContain("Copy what's in use");
  });

  it("says the alert service hasn't reported yet, with no setup step", () => {
    settings.data = make({ running: undefined });
    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.getByText(/hasn't reported what it uses yet/)).toBeInTheDocument();
    expect(within(row("ABT teams")).getByText("not reported yet")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save them here" })).not.toBeInTheDocument();
  });

  it("shows nothing to somebody who may not change CRE's settings", () => {
    settings.data = make({ canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] } });
    const { container } = render(<CaseAlertSettings chain="CRE" />);
    expect(container).toBeEmptyDOMElement();
  });


  it("offers the setup step: the file's values not saved yet, in one request, after asking", async () => {
    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.getByText("3 settings still come from the alert service's file.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Save them here" }));
    expect(screen.getByText("Save 3 settings here?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Save 3 settings" }));
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: 3 CRE settings from the file. Submit to save them.",
    );
    expect(saveValues.mutateAsync).not.toHaveBeenCalled();
    // Queued: the bar goes, rather than offering the same settings again.
    expect(screen.queryByText(/still come/)).not.toBeInTheDocument();
    await submit();
    expect(saveValues.mutateAsync).toHaveBeenCalledTimes(1);
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({
      ladder: "CRE",
      values: { "trigger.priorities": ["S0", "S1"], "teams.teamLeadsToCall": 3, onLeave: "skip" },
    });
    expect(screen.getByText(/^✓ Saved/)).toHaveTextContent("✓ Saved 1 change.");
  });

  it("hides the setup step once everything in use is saved", () => {
    settings.data = make({
      running: {
        reportedAt: AT,
        mode: "db",
        ladders: { CRE: { "teams.abts": { value: ["atlas"], source: "saved" } } },
      },
    });
    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.queryByText(/still come/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save them here" })).not.toBeInTheDocument();
  });

  it("edits a list in a dialog, then gives focus back to Edit", async () => {
    render(<CaseAlertSettings chain="CRE" />);
    const { row: abts, edit, dialog } = openEdit("ABT teams");
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(within(dialog).getByText(/The teams this ladder pages/)).toHaveTextContent(
      /Separate with commas, up to 30\./,
    );
    const input = within(dialog).getByLabelText(/New value/);
    expect(input).toHaveFocus();
    // It starts from the saved value.
    expect(input).toHaveValue("atlas, castor");
    fireEvent.change(input, { target: { value: "atlas, castor , draco," } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(edit).toHaveFocus();
    expect(within(abts).queryByRole("button", { name: "Remove" })).not.toBeInTheDocument();
    // The section it was changed in stays open, and the change waits for Submit.
    expect(abts).toBeInTheDocument();
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: CRE ABT teams. Submit to save them.",
    );
    await submit();
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({
      ladder: "CRE",
      values: { "teams.abts": ["atlas", "castor", "draco"] },
    });
    expect(screen.getByText(/^✓ Saved/)).toHaveTextContent(
      "✓ Saved 1 change. The alert service uses it after its next restart.",
    );
  });

  it("closes on Escape and gives focus back to Edit", () => {
    render(<CaseAlertSettings chain="CRE" />);
    const { edit, dialog } = openEdit("Priorities");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(edit).toHaveFocus();
    expect(screen.getByRole("region", { name: "CRE changes to submit" })).toHaveTextContent("No changes to submit.");
  });

  it("saves a head as a name and an email", async () => {
    render(<CaseAlertSettings chain="CRE" />);
    const { dialog } = openEdit("Head of CS");
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: " Kamala Silva " } });
    fireEvent.change(within(dialog).getByLabelText("Email"), { target: { value: "kamala@example.test" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    await submit();
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({
      ladder: "CRE",
      values: { "heads.cs": { name: "Kamala Silva", email: "kamala@example.test" } },
    });
  });

  it("shows the server's reason when Submit is refused, and keeps the change", async () => {
    saveValues.mutateAsync.mockRejectedValueOnce(
      new Error("teams.teamLeadsToCall: must be a whole number from 0 to 20"),
    );
    render(<CaseAlertSettings chain="CRE" />);
    const { dialog } = openEdit("Team leads to call");
    const input = within(dialog).getByLabelText(/New value/);
    expect(input).toHaveValue(3);
    expect(within(dialog).getByText(/0 to 20\./)).toBeInTheDocument();
    fireEvent.change(input, { target: { value: "99" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    fireEvent.click(screen.getByRole("button", { name: "Submit CRE changes" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "teams.teamLeadsToCall: must be a whole number from 0 to 20",
    );
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({ ladder: "CRE", values: { "teams.teamLeadsToCall": 99 } });
    expect(screen.getByRole("region", { name: "CRE changes to submit" })).toHaveTextContent("CRE Team leads to call");
  });

  it("saves a pick from a few and a yes or no", async () => {
    render(<CaseAlertSettings chain="CRE" />);
    let { dialog } = openEdit("People on leave");
    fireEvent.change(within(dialog).getByLabelText("New value"), { target: { value: "call" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));

    ({ dialog } = openEdit("Needs both to stop"));
    const box = within(dialog).getByRole("checkbox", { name: "On" });
    expect(box).toBeChecked();
    fireEvent.click(box);
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    await submit();
    expect(saveValues.mutateAsync).toHaveBeenNthCalledWith(1, { ladder: "CRE", values: { onLeave: "call" } });
    expect(saveValues.mutateAsync).toHaveBeenNthCalledWith(2, {
      ladder: "CRE",
      values: { "acknowledgement.requireBoth": false },
    });
  });

  it("goes back to the file's value from the dialog, after asking", async () => {
    render(<CaseAlertSettings chain="CRE" />);
    const { dialog } = openEdit("ABT teams");
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    expect(within(dialog).getByText(/The saved value is removed and the file's is used\./)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: CRE ABT teams uses the file's value again.",
    );
    await submit();
    expect(deleteValue.mutateAsync).toHaveBeenCalledWith(ABTS_ID);
  });

  it("offers no way back to the file when nothing is saved", () => {
    render(<CaseAlertSettings chain="CRE" />);
    const { dialog } = openEdit("Priorities");
    expect(within(dialog).queryByRole("button", { name: "Use the file's value" })).not.toBeInTheDocument();
  });

  it("shows one SRE ladder on both SRE chains, with a note", () => {
    for (const chain of ["SRE_SAAS", "SRE_IAAS"] as const) {
      const { unmount } = render(<CaseAlertSettings chain={chain} />);
      expect(screen.getByText("One SRE ladder: these apply to both the SaaS and IaaS chains.")).toBeInTheDocument();
      // Saved and in use alike: one value, no "Saved:" line.
      expect(within(row("Time between steps")).getByText("5m")).toBeInTheDocument();
      expect(within(row("Time between steps")).queryByText(/^Saved:/)).not.toBeInTheDocument();
      expect(screen.getByRole("region", { name: "Chat messages" })).toBeInTheDocument();
      expect(screen.queryByText("Head of CRE")).not.toBeInTheDocument();
      unmount();
    }
  });
  it("shows SME's own ladder settings: timing, teams, on leave, no heads", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["SME"], routeLadders: [] },
    });
    render(<CaseAlertSettings chain="SME" />);
    expect(screen.getByText(/SME ladder settings/)).toBeInTheDocument();
    for (const label of ["Time between steps", "Special Ops teams", "People on leave", "Say when nobody answered"]) {
      expect(row(label)).toBeInTheDocument();
    }
    expect(screen.queryByText("Head of CRE")).not.toBeInTheDocument();
    expect(screen.queryByText("Call L4")).not.toBeInTheDocument();
  });


  it("shows the CRE settings only the file can set, read-only", () => {
    render(<CaseAlertSettings chain="CRE" />);
    const group = screen.getByRole("region", { name: /: set in the alert service's file$/ });
    expect(within(group).getByText(/They change only with a reviewed change to the file/)).toBeInTheDocument();
    const value = (label: string) => fileValue(group, label);
    expect(value("Alerts on")).toBe("Yes");
    expect(value("Channel")).toBe("Calls and chat");
    expect(value("Max calls per ladder")).toBe("32");
    expect(value("Highest step")).toBe("L3");
    expect(value("Allowed numbers")).toBe("only 2 listed numbers");
    expect(value("Call before every lead has a number")).toBe("No");
    expect(value("Call the heads when nobody below can be called")).toBe("Yes");
    expect(value("Phone numbers from")).toBe("their profile");
    expect(within(group).queryByRole("button")).not.toBeInTheDocument();
    // A count, never the numbers themselves.
    expect(group.textContent).not.toMatch(/\+\d/);
  });
  it("shows the SRE settings only the file can set, without the heads", () => {
    render(<CaseAlertSettings chain="SRE_IAAS" />);
    const group = screen.getByRole("region", { name: /: set in the alert service's file$/ });
    const value = (label: string) => fileValue(group, label);
    expect(value("Alerts on")).toBe("No");
    expect(value("Channel")).toBe("Log only (no calls or chat)");
    expect(value("Max calls per ladder")).toBe("no cap");
    expect(value("Highest step")).toBe("no cap");
    expect(value("Allowed numbers")).toBe("any number");
    expect(value("Call before every lead has a number")).toBe("Yes");
    expect(value("Phone numbers from")).toBe("only numbers in the file");
    expect(within(group).queryByText(/Call the heads/)).not.toBeInTheDocument();
    expect(within(group).queryByRole("button")).not.toBeInTheDocument();
  });

  it("says the file-only settings are not reported yet", () => {
    const notYet = /Not reported yet: the alert service reports these when it runs in shadow or db mode\./;
    settings.data = make({ running: undefined });
    const { unmount } = render(<CaseAlertSettings chain="CRE" />);
    expect(screen.getByText(notYet)).toBeInTheDocument();
    unmount();
    settings.data = make({ running: { reportedAt: AT, mode: "shadow", ladders: {} } });
    render(<CaseAlertSettings chain="CRE" />);
    const group = screen.getByRole("region", { name: /: set in the alert service's file$/ });
    expect(within(group).getByText(notYet)).toBeInTheDocument();
    expect(within(group).queryByRole("table")).not.toBeInTheDocument();
  });

  it("still shows a saved key it doesn't know, never a link in it, and can go back to the file's", async () => {
    settings.data = make({
      values: [
        { id: ABTS_ID, ladder: "CRE", key: "teams.someday", value: HOOK, updatedBy: "a@b", updatedOn: AT },
      ],
    });
    render(<CaseAlertSettings chain="CRE" />);
    const { dialog } = openEdit("teams.someday");
    expect(within(dialog).getByText("Saved here: a link (not shown)")).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: "Add to changes" })).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    await submit();
    expect(deleteValue.mutateAsync).toHaveBeenCalledWith(ABTS_ID);
    expect(document.body.textContent).not.toContain("token=");
  });

  it("starts with everything but Team chat routing closed, each summary saying what's inside", () => {
    window.localStorage.clear();
    render(<CaseAlertSettings chain="CRE" />);
    // Team chat routing is not a section: it is always shown.
    expect(screen.getByRole("heading", { name: "Team chat routing" })).toBeInTheDocument();
    const summaries = [...document.querySelectorAll("summary")].map((x) => x.textContent);
    expect(summaries).toEqual([
      "▸Send a case's alerts to another CRE team · none",
      "▸CRE ladder settings · 2 saved here",
      "▸CRE: set in the alert service's file · read-only",
    ]);
    for (const d of document.querySelectorAll("details")) expect(d).not.toHaveAttribute("open");
    // Closed sections draw nothing inside.
    expect(screen.queryByText("ABT teams")).not.toBeInTheDocument();
    // The setup step stays at the top, outside them.
    const setup = screen.getByRole("button", { name: "Save them here" });
    expect(setup.closest("details")).toBeNull();
  });

  it("says each ladder group's count once the ladder section is opened, and they start closed too", () => {
    window.localStorage.clear();
    render(<CaseAlertSettings chain="CRE" />);
    fireEvent.click(screen.getByText("CRE ladder settings · 2 saved here"));
    const groups = within(screen.getByRole("region", { name: "CRE ladder settings" }))
      .getAllByRole("region")
      .map((r) => r.querySelector("summary")?.textContent);
    expect(groups).toEqual([
      "▸Who gets alerts · 1 saved here",
      "▸When a ladder starts · all from the file",
      "▸How it climbs · all from the file",
      "▸Heads · 1 saved here",
    ]);
    expect(screen.queryByText("ABT teams")).not.toBeInTheDocument();
  });

  it("counts the redirects in their summary", () => {
    window.localStorage.clear();
    settings.data = make({
      routes: [
        { id: "r1", ladder: "CRE", groupName: "Phoenix", teamKey: "atlas", enabled: true, updatedBy: "x", updatedOn: AT },
        { id: "r2", ladder: "CRE", groupName: "Sirius", teamKey: "castor", enabled: true, updatedBy: "x", updatedOn: AT },
      ],
    });
    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.getByText("Send a case's alerts to another CRE team · 2 redirects")).toBeInTheDocument();
  });

  it("opens a section from its summary and remembers it", () => {
    window.localStorage.clear();
    const { unmount } = render(<CaseAlertSettings chain="CRE" />);
    const summary = screen.getByText("CRE ladder settings · 2 saved here").closest("summary") as HTMLElement;
    summary.focus();
    expect(summary).toHaveFocus();
    // Enter or Space on a <summary> is a click.
    fireEvent.click(summary);
    expect(summary.closest("details")).toHaveAttribute("open");
    expect(summary).toHaveTextContent("▾CRE ladder settings");
    fireEvent.click(screen.getByText("Who gets alerts · 1 saved here"));
    expect(screen.getByText("ABT teams")).toBeInTheDocument();
    unmount();

    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.getByText("ABT teams")).toBeInTheDocument();
    // Closing is remembered as well.
    fireEvent.click(screen.getByText("CRE ladder settings · 2 saved here"));
    expect(screen.queryByText("ABT teams")).not.toBeInTheDocument();
    expect(JSON.parse(window.localStorage.getItem(OPEN_KEY) ?? "[]")).toEqual(["CRE.ladder.Who gets alerts"]);
  });

  it("still opens and closes when the browser's storage throws", () => {
    window.localStorage.clear();
    const get = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    const set = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    try {
      render(<CaseAlertSettings chain="CRE" />);
      expect(screen.queryByText("ABT teams")).not.toBeInTheDocument();
      fireEvent.click(screen.getByText("CRE ladder settings · 2 saved here"));
      fireEvent.click(screen.getByText("Who gets alerts · 1 saved here"));
      expect(screen.getByText("ABT teams")).toBeInTheDocument();
    } finally {
      get.mockRestore();
      set.mockRestore();
    }
  });
});
