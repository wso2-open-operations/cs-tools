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
import CaseAlertDraftsProvider from "./CaseAlertDraftsProvider";
import { LadderAdvanced } from "./CaseAlertLadderSettings";

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
const SRE_ABTS_ID = "55555555-5555-4555-8555-555555555555";
const HEAD_ID = "44444444-4444-4444-8444-444444444444";
const HOOK = "https://chat.googleapis.com/v1/spaces/AAAAtest01/messages?key=k&token=t";
const AT = "2026-10-13T10:00:00Z";

/** The CRE settings the page shows, in order. */
const CRE_ROWS = [
  "Alerts on",
  "Channel",
  "Priorities",
  "Max calls per ladder",
  "Call before every lead has a number",
  "Email per step",
  "ABT team lists (Cc on L1 and L2)",
  "CRE team list (Cc on L3)",
  "CS team list (Cc on L4)",
];

function make(over: Partial<Settings> = {}): Settings {
  return {
    canManage: { view: true, roomLadders: ["CRE", "SRE"], routeLadders: ["CRE", "SRE"] },
    environment: "staging",
    webhooksEnabled: true,
    revision: { saved: 4 },
    rooms: [],
    routes: [],
    values: [
      // A CRE setting the page no longer shows: still saved, still in use.
      { id: ABTS_ID, ladder: "CRE", key: "teams.abts", value: ["atlas", "castor"], updatedBy: "a@b", updatedOn: AT },
      {
        id: HEAD_ID,
        ladder: "CRE",
        key: "heads.cre",
        value: { name: "Nimal Perera", email: "nimal@example.test" },
        updatedBy: "a@b",
        updatedOn: AT,
      },
      { id: SRE_ABTS_ID, ladder: "SRE", key: "teams.abts", value: ["apollo", "artemis"], updatedBy: "a@b", updatedOn: AT },
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
        SRE: {
          "timing.interval": { value: "5m", source: "saved" },
          "teams.abts": { value: ["apollo"], source: "file" },
          "trigger.priorities": { value: ["S0", "S1"], source: "file" },
          onLeave: { value: "skip", source: "file" },
        },
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

/** The CRE switches as the alert service reports them on the tab. */
function withCreSwitches(s: Settings): Settings {
  s.running!.ladders.CRE = {
    ...s.running!.ladders.CRE,
    enabled: { value: true, source: "file" },
    channel: { value: "both", source: "file" },
    "safety.maxCallsPerLadder": { value: 32, source: "file" },
    "safety.callWithoutVerifiedLeads": { value: false, source: "file" },
  };
  return s;
}

function row(name: string): HTMLElement {
  const cell = screen.getByText(name);
  const tr = cell.closest("tr");
  if (!tr) throw new Error(`no row for ${name}`);
  return tr;
}

/** The labels of a table's rows, in order. */
function rowLabels(table: HTMLElement): string[] {
  return within(table)
    .getAllByRole("row")
    .slice(1)
    .map((r) => r.querySelector("b")?.textContent ?? "");
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

/** The whole CRE ladder, as the code still holds it under Advanced: the page
 *  no longer shows it, so it is drawn here directly. */
function renderWholeCreLadder(): void {
  render(
    <CaseAlertDraftsProvider>
      <LadderAdvanced ladder="CRE" settings={settings.data!} onSaved={() => undefined} />
    </CaseAlertDraftsProvider>,
  );
}

const OPEN_KEY = "csm-ts.alert-settings.open";
/** Every section open, as for a person who opened them all before: most of
 *  these tests work inside them. The first section starts open anyway. */
function openEverything(): void {
  const ids = ["CRE", "SRE"].flatMap((f) => [`${f}.where`, `${f}.who`, `${f}.advanced`, `${f}.timing`]);
  window.localStorage.setItem(OPEN_KEY, JSON.stringify(ids));
}

const NOT_REPORTED_TIMING = "Not reported yet: the alert service reports it when it runs.";

/** A section's summary, by its whole text: the title and its details are
 *  separate elements, so getByText can't match the two together. */
function summaryWith(match: string | RegExp): HTMLElement {
  const found = [...document.querySelectorAll("summary")].find((x) => {
    const text = (x.textContent ?? "").replace(/^[▾▸]/, "");
    return typeof match === "string" ? text === match : match.test(text);
  });
  if (!found) throw new Error(`no section summary matching ${String(match)}`);
  return found as HTMLElement;
}

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
  openEverything();
  settings.data = make();
});

describe("CaseAlertLadderSettings: CRE", () => {
  it("shows the CRE switch and the settings people change, in one table, and nothing else of the ladder", () => {
    render(<CaseAlertSettings chain="CRE" />);
    const ladder = screen.getByRole("region", { name: "CRE ladder" });
    expect(rowLabels(within(ladder).getByRole("table"))).toEqual(CRE_ROWS);
    expect(within(row("Priorities")).getByText("S0, S1")).toBeInTheDocument();
    expect(within(row("Priorities")).getByText("from the file")).toBeInTheDocument();
    expect(within(ladder).getByText(/in shadow mode\. In shadow mode it still uses its file/)).toBeInTheDocument();

    // Everything else of the ladder stays in the code, off the page.
    for (const gone of ["Who gets alerts", "Advanced"]) {
      expect(screen.queryByRole("region", { name: gone })).not.toBeInTheDocument();
    }
    // The heads come from the leadership team when left empty, so they are off the tab too.
    for (const label of ["ABT teams", "Team leads to call", "People on leave", "Needs both to stop", "Head of CRE", "Head of CS"]) {
      expect(screen.queryByText(label)).not.toBeInTheDocument();
    }
    expect(screen.queryByRole("group", { name: /: set in the alert service's file$/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Copy them here" })).not.toBeInTheDocument();
    // A saved value it no longer shows is still counted, so nothing is in use unseen.
    expect(screen.getByText(/^The other CRE settings/)).toHaveTextContent(
      "The other CRE settings (who is in each team, the heads, where alerts go, how a chain climbs, the alert service's own) are not shown here and keep their values; 2 of them are saved here and still in use.",
    );

    expect(document.body.textContent).not.toContain("+9477");
    expect(document.body.textContent).not.toContain("token=");
    expect(document.body.textContent).not.toContain("chat.googleapis.com");
  });

  it("starts with the CRE ladder open and the rest closed, each summary saying what's inside", () => {
    window.localStorage.clear();
    render(<CaseAlertSettings chain="CRE" />);
    const summaries = [...document.querySelectorAll("summary")].map((x) => x.textContent);
    expect(summaries).toEqual([
      "▾CRE ladder · CRE on, calls and chat",
      "▸When each step is called · read-only",
    ]);
    expect(row("Alerts on")).toBeInTheDocument();
    expect(screen.queryByText(NOT_REPORTED_TIMING)).not.toBeInTheDocument();
  });

  it("says nothing more when no hidden CRE setting is saved", () => {
    settings.data = make({ values: [] });
    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.getByText(/^The other CRE settings/)).toHaveTextContent(
      "The other CRE settings (who is in each team, the heads, where alerts go, how a chain climbs, the alert service's own) are not shown here and keep their values.",
    );
  });

  it("counts a saved CRE key it doesn't know with the hidden ones, never showing a link in it", () => {
    settings.data = make({
      values: [{ id: ABTS_ID, ladder: "CRE", key: "teams.someday", value: HOOK, updatedBy: "a@b", updatedOn: AT }],
    });
    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.queryByText("teams.someday")).not.toBeInTheDocument();
    expect(screen.getByText(/^The other CRE settings/)).toHaveTextContent("1 of them is saved here and still in use.");
    expect(document.body.textContent).not.toContain("token=");
  });

  it("shows when each step is called, read-only, from the alert service's report", () => {
    settings.data!.running!.fileOnly!.CRE = {
      ...settings.data!.running!.fileOnly!.CRE,
      timing: { S0: [[0], [1, 2, 3], [4, 5, 6], [8, 9, 10], [12, 13, 14]] },
    };
    render(<CaseAlertSettings chain="CRE" />);
    const section = screen.getByRole("region", { name: "When each step is called" });
    const timing = within(section).getByRole("table", { name: "CRE: when each step is called" });
    const s0 = within(timing).getByText("S0").closest("tr")!;
    expect(Array.from(s0.querySelectorAll(".cas-val")).map((c) => c.textContent)).toEqual([
      "+0",
      "+1, +2, +3",
      "+4, +5, +6",
      "+8, +9, +10",
      "+12, +13, +14",
      "+14 min",
    ]);
    expect(within(section).queryByRole("button")).not.toBeInTheDocument();
  });

  it("says the timing is not reported yet", () => {
    render(<CaseAlertSettings chain="CRE" />);
    const section = screen.getByRole("region", { name: "When each step is called" });
    expect(within(section).getByText(NOT_REPORTED_TIMING)).toBeInTheDocument();
    expect(within(section).queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows nothing to somebody who may not change CRE's settings", () => {
    settings.data = make({ canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] } });
    const { container } = render(<CaseAlertSettings chain="CRE" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("closes on Escape and gives focus back to Edit", () => {
    render(<CaseAlertSettings chain="CRE" />);
    const { edit, dialog } = openEdit("Priorities");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(edit).toHaveFocus();
    expect(screen.getByRole("region", { name: "CRE changes to submit" })).toHaveTextContent("No changes to submit.");
  });

  it("edits the ABT team lists one team at a time, and marks a bad address on its own row", async () => {
    render(<CaseAlertSettings chain="CRE" />);
    const { dialog } = openEdit("ABT team lists (Cc on L1 and L2)");
    // One field per ABT team (the saved teams.abts), named, no key=value line.
    const atlas = within(dialog).getByLabelText("Atlas");
    const castor = within(dialog).getByLabelText("Castor");
    expect(atlas).toHaveAttribute("type", "email");
    // A team may be left empty, so no team's field is marked required.
    for (const field of [atlas, castor]) expect(field).not.toBeRequired();
    expect(dialog.querySelector(".MuiFormLabel-asterisk")).toBeNull();
    expect(within(dialog).getByText("Leave a team empty for no Cc.", { exact: false })).toBeInTheDocument();
    fireEvent.change(atlas, { target: { value: "atlas team" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(within(dialog).getByText("Enter one email address, like team-list@example.com.")).toBeInTheDocument();
    expect(atlas).toHaveAttribute("aria-invalid", "true");
    expect(castor).not.toHaveAttribute("aria-invalid");
    fireEvent.change(atlas, { target: { value: " atlas-team@example.test " } });
    expect(within(dialog).queryByText("Enter one email address, like team-list@example.com.")).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    // The row shows what waits for Submit, by team.
    const r = row("ABT team lists (Cc on L1 and L2)");
    expect(within(r).getByText("waits for Submit")).toBeInTheDocument();
    expect(within(r).getByText("atlas-team@example.test")).toBeInTheDocument();
    expect(within(r).getByText("1 of 2 teams has a list")).toBeInTheDocument();
    await submit();
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({
      ladder: "CRE",
      values: { "email.teamLists": { atlas: "atlas-team@example.test" } },
    });
  });

  it("lists a saved team list of a team that is no longer an ABT, marked, and leaves it out of the count", () => {
    const base = make();
    settings.data = {
      ...base,
      values: [
        ...base.values,
        {
          id: "77777777-7777-4777-8777-777777777777",
          ladder: "CRE",
          key: "email.teamLists",
          value: { atlas: "atlas@example.test", castor: "castor@example.test", rigel: "rigel@example.test" },
          updatedBy: "a@b",
          updatedOn: AT,
        },
      ],
    };
    render(<CaseAlertSettings chain="CRE" />);
    const r = row("ABT team lists (Cc on L1 and L2)");
    expect(within(r).getByText("rigel@example.test")).toBeInTheDocument();
    expect(within(r).getByText("(not used)", { exact: false })).toBeInTheDocument();
    expect(within(r).getByText("2 of 2 teams have a list")).toBeInTheDocument();
  });

  it("asks for one address for the CRE and CS lists", () => {
    render(<CaseAlertSettings chain="CRE" />);
    const { dialog } = openEdit("CRE team list (Cc on L3)");
    const input = within(dialog).getByLabelText("Mailing list");
    expect(input).toHaveAttribute("type", "email");
    // Empty means no Cc, so it is not marked required.
    expect(input).not.toBeRequired();
    expect(input.closest("label")?.querySelector(".MuiFormLabel-asterisk")).toBeNull();
    fireEvent.change(input, { target: { value: "a@example.test, b@example.test" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(within(dialog).getByRole("alert")).toHaveTextContent("enter one email address");
  });

  it("saves a head as a name and an email, in the full layout", async () => {
    render(<CaseAlertSettings chain="CRE" layout="full" />);
    const { dialog } = openEdit("Head of CS");
    // Both are needed, and both carry the required asterisk (outside the name).
    const name = within(dialog).getByRole("textbox", { name: "Name" });
    const email = within(dialog).getByRole("textbox", { name: "Email" });
    for (const field of [name, email]) {
      expect(field).toBeRequired();
      expect(field.closest("label")?.querySelector(".MuiFormLabel-asterisk")).toHaveAttribute("aria-hidden", "true");
    }
    fireEvent.change(name, { target: { value: " Kamala Silva " } });
    fireEvent.change(email, { target: { value: "kamala@example.test" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    await submit();
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({
      ladder: "CRE",
      values: { "heads.cs": { name: "Kamala Silva", email: "kamala@example.test" } },
    });
  });

  it("shows the server's reason when Submit is refused, and keeps the change", async () => {
    settings.data = withCreSwitches(make());
    saveValues.mutateAsync.mockRejectedValueOnce(
      new Error("safety.maxCallsPerLadder: must be a whole number from 1 to 50"),
    );
    render(<CaseAlertSettings chain="CRE" />);
    const { dialog } = openEdit("Max calls per ladder");
    const input = within(dialog).getByRole("spinbutton", { name: "New value" });
    // A number cannot be left empty.
    expect(input).toBeRequired();
    expect(input.closest("label")?.querySelector(".MuiFormLabel-asterisk")).toBeInTheDocument();
    expect(input).toHaveValue(32);
    expect(within(dialog).getByText(/1 to 50\./)).toBeInTheDocument();
    fireEvent.change(input, { target: { value: "99" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    fireEvent.click(screen.getByRole("button", { name: "Submit CRE changes" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "safety.maxCallsPerLadder: must be a whole number from 1 to 50",
    );
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({ ladder: "CRE", values: { "safety.maxCallsPerLadder": 99 } });
    expect(screen.getByRole("region", { name: "CRE changes to submit" })).toHaveTextContent("CRE Max calls per ladder");
  });

  it("changes a pick from a few and a yes or no right in their rows, saved on Submit", async () => {
    settings.data = withCreSwitches(make());
    render(<CaseAlertSettings chain="CRE" />);
    // No dialog for these: a dropdown and a switch in the row.
    expect(within(row("Channel")).queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
    const channel = within(row("Channel")).getByRole("combobox", { name: "Channel" });
    expect(channel).toHaveValue("both");
    fireEvent.change(channel, { target: { value: "chat" } });
    expect(channel).toHaveValue("chat");
    expect(within(row("Channel")).getByText("waits for Submit")).toBeInTheDocument();

    const box = within(row("Call before every lead has a number")).getByRole("switch", {
      name: "Call before every lead has a number",
    });
    expect(box).toHaveAttribute("aria-checked", "false");
    fireEvent.click(box);
    expect(box).toHaveAttribute("aria-checked", "true");
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: CRE Call before every lead has a number. Submit to save them.",
    );
    // Changed twice before Submit: one change, the latest.
    fireEvent.change(channel, { target: { value: "log" } });
    await submit();
    expect(saveValues.mutateAsync).toHaveBeenNthCalledWith(1, { ladder: "CRE", values: { channel: "log" } });
    expect(saveValues.mutateAsync).toHaveBeenNthCalledWith(2, {
      ladder: "CRE",
      values: { "safety.callWithoutVerifiedLeads": true },
    });
  });

  it("takes a switch back to the file's value from its row", async () => {
    settings.data = withCreSwitches(
      make({ values: [{ id: ABTS_ID, ladder: "CRE", key: "enabled", value: false, updatedBy: "a@b", updatedOn: AT }] }),
    );
    render(<CaseAlertSettings chain="CRE" />);
    const alertsOn = row("Alerts on");
    expect(within(alertsOn).getByRole("switch", { name: "Alerts on" })).toHaveAttribute("aria-checked", "false");
    // The switch shows the saved Off; the chip says what still runs.
    expect(within(alertsOn).getByText("In use: Yes")).toBeInTheDocument();
    fireEvent.click(within(alertsOn).getByRole("button", { name: "Alerts on: use the file's value" }));
    // Queued: the row says so and doesn't offer it twice.
    expect(within(alertsOn).getByText("file's value on Submit")).toBeInTheDocument();
    expect(within(alertsOn).queryByRole("button", { name: "Alerts on: use the file's value" })).not.toBeInTheDocument();
    await submit();
    expect(deleteValue.mutateAsync).toHaveBeenCalledWith(ABTS_ID);
  });

  it("goes back to the file's value from the dialog, after asking", async () => {
    render(<CaseAlertSettings chain="CRE" layout="full" />);
    const { dialog } = openEdit("Head of CRE");
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    expect(within(dialog).getByText(/The saved value is removed and the file's is used\./)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: CRE Head of CRE uses the file's value again.",
    );
    await submit();
    expect(deleteValue.mutateAsync).toHaveBeenCalledWith(HEAD_ID);
  });

  it("offers no way back to the file when nothing is saved", () => {
    render(<CaseAlertSettings chain="CRE" />);
    const { dialog } = openEdit("Priorities");
    expect(within(dialog).queryByRole("button", { name: "Use the file's value" })).not.toBeInTheDocument();
  });

  it("keeps CRE's chat spaces and redirects off the tab, and counts the saved ones", () => {
    settings.data = make({
      routes: [
        { id: "r1", ladder: "CRE", groupName: "Phoenix", teamKey: "atlas", enabled: true, updatedBy: "x", updatedOn: AT },
        { id: "r2", ladder: "CRE", groupName: "Sirius", teamKey: "castor", enabled: true, updatedBy: "x", updatedOn: AT },
      ],
    });
    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.queryByRole("region", { name: "Where alerts go" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "+ Add chat space" })).not.toBeInTheDocument();
    // The saved ABT teams, the head and the two redirects still apply, unseen.
    expect(screen.getByText(/^The other CRE settings/)).toHaveTextContent("4 of them are saved here and still in use.");
  });

  it("opens a section from its summary and remembers it", () => {
    window.localStorage.clear();
    const { unmount } = render(<CaseAlertSettings chain="CRE" />);
    const summary = summaryWith("When each step is called · read-only");
    summary.focus();
    expect(summary).toHaveFocus();
    // Enter or Space on a <summary> is a click.
    fireEvent.click(summary);
    expect(summary.closest("details")).toHaveAttribute("open");
    expect(screen.getByText(NOT_REPORTED_TIMING)).toBeInTheDocument();
    // The ladder starts open; closing it is remembered too.
    fireEvent.click(screen.getByText(/^CRE ladder/));
    expect(screen.queryByText("Alerts on")).not.toBeInTheDocument();
    unmount();

    render(<CaseAlertSettings chain="CRE" />);
    expect(screen.getByText(NOT_REPORTED_TIMING)).toBeInTheDocument();
    expect(screen.queryByText("Alerts on")).not.toBeInTheDocument();
    fireEvent.click(summaryWith("When each step is called · read-only"));
    expect(screen.queryByText(NOT_REPORTED_TIMING)).not.toBeInTheDocument();
    expect(JSON.parse(window.localStorage.getItem(OPEN_KEY) ?? "[]")).toEqual([]);
    expect(JSON.parse(window.localStorage.getItem("csm-ts.alert-settings.closed") ?? "[]")).toEqual(["CRE.ladder"]);
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
      expect(screen.queryByText(NOT_REPORTED_TIMING)).not.toBeInTheDocument();
      // The ladder still starts open.
      expect(row("Alerts on")).toBeInTheDocument();
      fireEvent.click(summaryWith("When each step is called · read-only"));
      expect(screen.getByText(NOT_REPORTED_TIMING)).toBeInTheDocument();
    } finally {
      get.mockRestore();
      set.mockRestore();
    }
  });
});

describe("CaseAlertLadderSettings: the whole CRE ladder, kept in the code", () => {
  it("still draws every CRE group and what only the file sets", () => {
    renderWholeCreLadder();
    const ladder = screen.getByRole("group", { name: "CRE ladder" });
    expect(within(ladder).getAllByRole("heading", { level: 5 }).map((h) => h.textContent)).toEqual([
      "Call limits",
      "When a ladder starts",
      "How it climbs",
      "Heads",
      "Case Alert email",
      "Set in the alert service's file (read-only)",
    ]);
    expect(within(row("Team leads to call")).getByText("3")).toBeInTheDocument();
    expect(within(row("Needs both to stop")).getByText("saved here")).toBeInTheDocument();
  });

  it("still words the CRE settings only the file can set, read-only", () => {
    const running = settings.data!.running!;
    running.fileOnly!.CRE = {
      ...running.fileOnly!.CRE,
      ackChat: true,
      "email.enabled": true,
      "email.teamLists": 7,
      "email.creTeamList": true,
      "email.csTeamList": true,
      "rules.custom": false,
      timing: { S0: [[0], [1, 2, 3], [4, 5, 6], [8, 9, 10], [12, 13, 14]] },
    };
    running.groups = { CRE: { aliases: { americas: "americas_cre_team" }, defaultGroups: [], teamlessRooms: [] } };
    renderWholeCreLadder();
    const group = screen.getByRole("group", { name: /: set in the alert service's file$/ });
    const value = (label: string) => fileValue(group, label);
    expect(value("Alerts on")).toBe("Yes");
    expect(value("Channel")).toBe("Calls and chat");
    expect(value("Highest step")).toBe("L3");
    expect(value("Allowed numbers")).toBe("only 2 listed numbers");
    expect(value("Call the heads when nobody below can be called")).toBe("Yes");
    expect(value("Reply in the thread when a chain stops")).toBe("Yes");
    expect(value("Email per step")).toBe("On (7 team lists, CRE list set, CS list set)");
    expect(value("Who each step calls")).toBe("Built-in rules (Case Paging Rules sheet)");
    expect(within(group).getByRole("table", { name: "CRE: when each step is called" })).toBeInTheDocument();
    const names = within(group).getByRole("table", { name: "CRE: team names mapped in the file" });
    expect(within(names).getByText("americas").closest("tr")).toHaveTextContent("americas_cre_team");
    expect(within(group).queryByRole("button")).not.toBeInTheDocument();
    // A count, never the numbers or the addresses themselves (the timing's
    // "+1" is minutes, not a phone number).
    expect(group.textContent).not.toMatch(/\+\d{6,}/);
    expect(group.textContent).not.toMatch(/@/);
  });
});

describe("CaseAlertLadderSettings: SRE, the full layout kept in the code", () => {
  it("shows one value per row: what's in use, where it came from, and a saved one not in use yet", () => {
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    expect(screen.getByRole("group", { name: "SRE ladder" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Who gets alerts" })).toBeInTheDocument();
    for (const group of ["When a ladder starts", "How it climbs", "Chat messages"]) {
      expect(screen.getByRole("heading", { name: group })).toBeInTheDocument();
    }
    expect(screen.queryByText("Head of CRE")).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Saved here" })).not.toBeInTheDocument();

    const abts = row("ABT teams");
    expect(within(abts).getByText("apollo")).toBeInTheDocument();
    expect(within(abts).getByText("from the file")).toBeInTheDocument();
    expect(within(abts).getByText(/^Saved:/)).toHaveTextContent("Saved: apollo, artemis · not in use yet");
    expect(within(row("Priorities")).getByText("S0, S1")).toBeInTheDocument();
    expect(within(row("Priorities")).queryByText(/^Saved:/)).not.toBeInTheDocument();
    expect(within(row("People on leave")).getByText("Skip them")).toBeInTheDocument();
  });

  it("says the alert service hasn't reported yet, with no setup step", () => {
    settings.data = make({ running: undefined });
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    expect(screen.getByText(/hasn't reported what it uses yet/)).toBeInTheDocument();
    expect(within(row("ABT teams")).getByText("not reported yet")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Copy them here" })).not.toBeInTheDocument();
  });

  it("offers the setup step: the file's values not saved yet, in one request, after asking", async () => {
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    expect(
      screen.getByText("2 SRE settings still come from the alert service's file. Copy them here to change them on this page."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Copy them here" }));
    expect(screen.getByText("Copy 2 settings here?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Copy 2 settings" }));
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: 2 SRE settings from the file. Submit to save them.",
    );
    expect(saveValues.mutateAsync).not.toHaveBeenCalled();
    expect(screen.queryByText(/still come/)).not.toBeInTheDocument();
    await submit("SRE");
    expect(saveValues.mutateAsync).toHaveBeenCalledTimes(1);
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({
      ladder: "SRE",
      values: { "trigger.priorities": ["S0", "S1"], onLeave: "skip" },
    });
    expect(screen.getByText(/^✓ Saved/)).toHaveTextContent("✓ Saved 1 change.");
  });

  it("hides the setup step once everything in use is saved", () => {
    settings.data = make({
      running: {
        reportedAt: AT,
        mode: "db",
        ladders: { SRE: { "teams.abts": { value: ["apollo"], source: "saved" } } },
      },
    });
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    expect(screen.queryByText(/still come/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Copy them here" })).not.toBeInTheDocument();
  });

  it("edits a list in a dialog, then gives focus back to Edit", async () => {
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    const { row: abts, edit, dialog } = openEdit("ABT teams");
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(within(dialog).getByText(/The teams this ladder pages/)).toHaveTextContent(/Separate with commas, up to 30\./);
    const input = within(dialog).getByLabelText(/New value/);
    expect(input).toHaveFocus();
    // A list may be saved empty, so it is not marked required.
    expect(input).not.toBeRequired();
    expect(input.closest("label")?.querySelector(".MuiFormLabel-asterisk")).toBeNull();
    // It starts from the saved value.
    expect(input).toHaveValue("apollo, artemis");
    fireEvent.change(input, { target: { value: "apollo, artemis , iaas," } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(edit).toHaveFocus();
    expect(abts).toBeInTheDocument();
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: SRE ABT teams. Submit to save them.",
    );
    await submit("SRE");
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({
      ladder: "SRE",
      values: { "teams.abts": ["apollo", "artemis", "iaas"] },
    });
    expect(screen.getByText(/^✓ Saved/)).toHaveTextContent(
      "✓ Saved 1 change. The alert service uses it after its next restart.",
    );
  });

  it("goes back to the file's value from the dialog, after asking", async () => {
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    const { dialog } = openEdit("ABT teams");
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: SRE ABT teams uses the file's value again.",
    );
    await submit("SRE");
    expect(deleteValue.mutateAsync).toHaveBeenCalledWith(SRE_ABTS_ID);
  });

  it("shows one SRE ladder on both SRE chains, with a note", () => {
    for (const chain of ["SRE_SAAS", "SRE_IAAS"] as const) {
      const { unmount } = render(<CaseAlertSettings chain={chain} layout="full" />);
      expect(screen.getByText("One SRE ladder: these apply to both the SaaS and IaaS chains.")).toBeInTheDocument();
      // Saved and in use alike: one value, no "Saved:" line.
      expect(within(row("Time between steps")).getByText("5m")).toBeInTheDocument();
      expect(within(row("Time between steps")).queryByText(/^Saved:/)).not.toBeInTheDocument();
      expect(screen.getByRole("heading", { name: "Chat messages" })).toBeInTheDocument();
      expect(screen.queryByText("Head of CRE")).not.toBeInTheDocument();
      unmount();
    }
  });

  it("shows SME's own ladder settings: timing, teams, on leave, no heads", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["SME"], routeLadders: [] },
    });
    render(<CaseAlertSettings chain="SME" layout="full" />);
    expect(screen.getByRole("group", { name: "SME ladder" })).toBeInTheDocument();
    for (const label of ["Time between steps", "Special Ops teams", "People on leave", "Say when nobody answered"]) {
      expect(row(label)).toBeInTheDocument();
    }
    expect(screen.queryByText("Head of CRE")).not.toBeInTheDocument();
    expect(screen.queryByText("Call L4")).not.toBeInTheDocument();
  });

  it("shows no CRE timing or team names on the SRE ladder", () => {
    settings.data!.running!.fileOnly!.SRE = { ...settings.data!.running!.fileOnly!.SRE, ackChat: false };
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    const group = screen.getByRole("group", { name: /: set in the alert service's file$/ });
    expect(fileValue(group, "Reply in the thread when a chain stops")).toBe("No");
    expect(within(group).queryByRole("table", { name: /when each step is called/ })).not.toBeInTheDocument();
    expect(within(group).queryByText("Email per step")).not.toBeInTheDocument();
  });

  it("shows the SRE settings only the file can set, without the heads", () => {
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    const group = screen.getByRole("group", { name: /: set in the alert service's file$/ });
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

  it("edits the switches on the tab once the alert service reports them there, and says who gets which channel", () => {
    window.localStorage.clear();
    const base = make();
    settings.data = make({
      running: {
        ...base.running!,
        ladders: {
          SRE: {
            ...base.running!.ladders.SRE,
            enabled: { value: true, source: "file" },
            channel: { value: "both", source: "saved" },
          },
        },
        channels: {
          SRE: { enabled: true, channel: "both", teams: [] },
        },
      },
    });
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    // Advanced starts closed.
    expect(screen.queryByRole("group", { name: /: set in the alert service's file$/ })).not.toBeInTheDocument();
    fireEvent.click(summaryWith(/^Advanced · /));
    // The file block no longer lists what the tab edits.
    const file = screen.getByRole("group", { name: /: set in the alert service's file$/ });
    expect(within(file).queryByText("Alerts on")).not.toBeInTheDocument();
    expect(within(file).queryByText("Channel")).not.toBeInTheDocument();
    const channels = screen.getByRole("group", { name: "SRE: who gets which channel" });
    expect(channels).toHaveTextContent("The SRE ladder is on: Calls and chat.");
    expect(summaryWith(/^On\/off and channel · SRE on, calls and chat/)).toBeInTheDocument();
  });

  it("says the file-only settings are not reported yet", () => {
    const notYet = /Not reported yet: the alert service reports these when it runs in shadow or db mode\./;
    settings.data = make({ running: undefined });
    const { unmount } = render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    expect(screen.getByText(notYet)).toBeInTheDocument();
    unmount();
    settings.data = make({ running: { reportedAt: AT, mode: "shadow", ladders: {} } });
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    const group = screen.getByRole("group", { name: /: set in the alert service's file$/ });
    expect(within(group).getByText(notYet)).toBeInTheDocument();
    expect(within(group).queryByRole("table")).not.toBeInTheDocument();
  });

  it("still shows a saved key it doesn't know, never a link in it, and can go back to the file's", async () => {
    settings.data = make({
      values: [{ id: SRE_ABTS_ID, ladder: "SRE", key: "teams.someday", value: HOOK, updatedBy: "a@b", updatedOn: AT }],
    });
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    const { dialog } = openEdit("teams.someday");
    expect(within(dialog).getByText("Saved here: a link (not shown)")).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: "Add to changes" })).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Use the file's value" }));
    await submit("SRE");
    expect(deleteValue.mutateAsync).toHaveBeenCalledWith(SRE_ABTS_ID);
    expect(document.body.textContent).not.toContain("token=");
  });

  it("puts the rest of the file under Advanced, a ladder's groups one after another", () => {
    window.localStorage.clear();
    render(<CaseAlertSettings chain="SRE_IAAS" layout="full" />);
    fireEvent.click(summaryWith(/^Advanced · /));
    const ladder = screen.getByRole("group", { name: "SRE ladder" });
    expect(within(ladder).getAllByRole("heading", { level: 5 }).map((h) => h.textContent)).toEqual([
      "Call limits",
      "When a ladder starts",
      "How it climbs",
      "Chat messages",
      "Set in the alert service's file (read-only)",
    ]);
    // The copy-from-the-file step sits with its ladder, above the groups.
    expect(within(ladder).getByRole("button", { name: "Copy them here" })).toBeInTheDocument();
  });
});

describe("CaseAlertLadderSettings: SRE, as the tab shows it", () => {
  it("shows each SRE-family ladder's switch and the settings people change, and nothing else", () => {
    settings.data = make({ canManage: { view: true, roomLadders: ["SRE", "SME"], routeLadders: ["SRE"] } });
    render(<CaseAlertSettings chain="SRE_IAAS" />);
    const ladders = screen.getByRole("region", { name: "SRE and SME ladders" });
    const [sre, sme] = within(ladders).getAllByRole("table");
    expect(rowLabels(sre)).toEqual([
      "Alerts on",
      "Channel",
      "Priorities",
      "Max calls per ladder",
      "Call before every lead has a number",
      "Time between steps",
    ]);
    expect(rowLabels(sme)).toEqual(["Alerts on", "Channel", "Time between steps"]);
    expect(within(ladders).getByText("One SRE ladder: these apply to both the SaaS and IaaS chains.")).toBeInTheDocument();
    expect(within(row("Priorities")).getByText("S0, S1")).toBeInTheDocument();

    for (const gone of ["Who gets alerts", "Advanced", "When each step is called"]) {
      expect(screen.queryByRole("region", { name: gone })).not.toBeInTheDocument();
    }
    for (const label of ["ABT teams", "People on leave", "Say when nobody answered"]) {
      expect(screen.queryByText(label)).not.toBeInTheDocument();
    }
    expect(screen.queryByRole("button", { name: "Copy them here" })).not.toBeInTheDocument();
    // The saved ABT teams are no longer on the tab, so they are counted.
    expect(screen.getByText(/^The other SRE settings/)).toHaveTextContent(
      "The other SRE settings (who is in each team, how a chain climbs, the alert service's own) are not shown here and keep their values; 1 of them is saved here and still in use.",
    );
  });

  it("starts with the ladders open and where alerts go closed", () => {
    window.localStorage.clear();
    render(<CaseAlertSettings chain="SRE_IAAS" />);
    const summaries = [...document.querySelectorAll("summary")].map((x) => x.textContent);
    expect(summaries).toEqual(["▾SRE ladder · SRE off · 1 saved here", "▸Where alerts go"]);
  });

  it("names the assignment groups that post to the shared room under where alerts go", () => {
    settings.data = make({ unmappedGroups: [{ id: "g1", name: "Lumen SRE Group" }] });
    render(<CaseAlertSettings chain="SRE_IAAS" />);
    const where = screen.getByRole("region", { name: "Where alerts go" });
    expect(where).toHaveTextContent("Lumen SRE Group");
  });

  it("saves a time between steps", async () => {
    render(<CaseAlertSettings chain="SRE_IAAS" />);
    const { dialog } = openEdit("Time between steps");
    const input = within(dialog).getByRole("textbox", { name: "New value" });
    // A duration cannot be left empty.
    expect(input).toBeRequired();
    fireEvent.change(input, { target: { value: "4m" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    await submit("SRE");
    expect(saveValues.mutateAsync).toHaveBeenCalledWith({ ladder: "SRE", values: { "timing.interval": "4m" } });
  });
});
