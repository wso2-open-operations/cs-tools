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

import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type {
  CaseAlertChatRoom,
  CaseAlertSettings as Settings,
} from "../types";
// vi.mock below is hoisted above this import, so the section gets the mocks.
import CaseAlertSettings from "./CaseAlertSettings";

const settings: { data?: Settings } = {};
// A password box has no role; its label ends in the required asterisk when
// the space is new.
const WEBHOOK = /^Google Chat webhook URL/;
const saveRoom = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const setWebhook = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const testRoom = { mutate: vi.fn(), isPending: false };
const deleteRoom = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const noop = { mutate: vi.fn(), isPending: false };

vi.mock("../api/useCaseAlertSettings", () => ({
  useCaseAlertSettings: () => ({
    data: settings.data,
    isLoading: false,
    isError: false,
  }),
  useSaveCaseAlertRoom: () => saveRoom,
  useSetCaseAlertWebhook: () => setWebhook,
  useTestCaseAlertRoom: () => testRoom,
  useDeleteCaseAlertRoom: () => deleteRoom,
  useSaveCaseAlertRoute: () => noop,
  useDeleteCaseAlertRoute: () => noop,
  useSaveCaseAlertValues: () => noop,
  useDeleteCaseAlertValue: () => noop,
}));

const AT = "2026-10-13T10:00:00Z";
const HINT = "spaces/AAAAtest01 · …wxyz";
const HOOK =
  "https://chat.googleapis.com/v1/spaces/AAAAtest01/messages?key=k&token=t";
const NOT_DB =
  "The alert service only compares it with its file (shadow mode); it uses it once PAGING_CONFIG_SOURCE is db or unset.";
const DB = "The alert service uses it after its next restart.";

function room(over: Partial<CaseAlertChatRoom>): CaseAlertChatRoom {
  return {
    id: "22222222-2222-4222-8222-222222222222",
    ladder: "SRE",
    label: "SRE Case Alerts",
    enabled: true,
    teamless: false,
    hasWebhook: false,
    updatedBy: "x",
    updatedOn: AT,
    ...over,
  };
}

const apollo = room({
  id: "apollo-room",
  teamKey: "apollo",
  label: "Apollo SRE",
  teamless: true,
  hasWebhook: true,
  webhookHint: HINT,
  lastTestStatus: "delivered",
  lastTestAt: AT,
});
const iaas = room({
  id: "iaas-room",
  teamKey: "iaas",
  label: "IaaS SRE",
  channel: "call",
});
const shared = room({ id: "shared-room" });

function make(over: Partial<Settings> = {}): Settings {
  return {
    canManage: {
      view: true,
      roomLadders: ["CRE", "SRE"],
      routeLadders: ["CRE", "SRE"],
    },
    environment: "staging",
    webhooksEnabled: true,
    revision: { saved: 4 },
    rooms: [apollo, iaas],
    routeOptions: {
      groups: [],
      teams: [
        { key: "apollo", name: "Apollo", family: "SRE" },
        { key: "artemis", name: "Artemis", family: "SRE" },
        { key: "iaas", name: "IaaS", family: "SRE" },
        { key: "zeus", name: "Zeus", family: "SRE" },
        { key: "vega", name: "Vega", family: "CRE" },
      ],
      rotas: [],
    },
    routes: [],
    values: [],
    running: {
      reportedAt: AT,
      mode: "shadow",
      ladders: {},
      fileOnly: { SRE: { channel: "chat" } },
    },
    ...over,
  };
}

/** A row of the chat space table, by its team's name. */
function chatRow(name: string): HTMLElement {
  const table = screen.getByRole("table", { name: "Chat space for each team" });
  const tr = within(table).getByText(name).closest("tr");
  if (!tr) throw new Error(`no row for ${name}`);
  return tr;
}

function optionNames(select: HTMLElement): string[] {
  return within(select)
    .getAllByRole("option")
    .map((o) => o.textContent ?? "");
}

function openAdd(): { button: HTMLElement; dialog: HTMLElement } {
  const button = screen.getByRole("button", {
    name: "+ Add chat space",
  });
  button.focus();
  fireEvent.click(button);
  return {
    button,
    dialog: screen.getByRole("dialog", { name: "Add a team's chat space" }),
  };
}

beforeEach(() => {
  vi.resetAllMocks();
  window.localStorage.clear();
  // The chat spaces sit under "Where alerts go", which starts closed; these
  // tests work inside it, so it opens as for a person who opened it before.
  window.localStorage.setItem("csm-ts.alert-settings.open", JSON.stringify(["SRE.where", "CRE.where", "SRE.advanced"]));
  settings.data = make();
});

describe("Chat space for each team", () => {
  it("leaves when a change is used to the header's chip, not each dialog", () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    expect(within(dialog).queryByText(/restart/)).not.toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Add to changes" })).toBeInTheDocument();
  });

  it("lists only the routings saved here, with the hint and last test: the space alone", () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(
      screen.getByRole("heading", { name: "Chat space for each team" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Which Google Chat space each team's alerts go to.",
      ),
    ).toBeInTheDocument();
    // How a team is alerted stays the file's: no column for it.
    expect(
      screen.queryByRole("columnheader", { name: "Alerts by" }),
    ).not.toBeInTheDocument();
    const table = screen.getByRole("table", { name: "Chat space for each team" });
    // Artemis has none saved: not listed.
    expect(within(table).queryByText("Artemis")).not.toBeInTheDocument();
    expect(within(table).getAllByRole("row")).toHaveLength(3);

    const a = chatRow("Apollo");
    expect(
      within(a).getByText(/Apollo SRE · also gets incidents with no SRE team/),
    ).toBeInTheDocument();
    expect(within(a).getByText(HINT)).toBeInTheDocument();
    expect(within(a).getByText(/Last test:/)).toHaveTextContent(
      "Last test: delivered ·",
    );
    expect(within(chatRow("IaaS")).queryByText("Calls only")).toBeNull();
    expect(
      within(chatRow("IaaS")).getByText("No URL saved (uses the file's space)"),
    ).toBeInTheDocument();
    // Nothing in the rows saves on change.
    expect(within(table).queryByRole("combobox")).not.toBeInTheDocument();
    expect(within(table).queryByRole("checkbox")).not.toBeInTheDocument();
    expect(document.body.textContent).not.toContain("token=");
  });

  it("lists every SRE routing on both SRE chains, the shared space first", () => {
    settings.data = make({ rooms: [iaas, apollo, shared] });
    for (const chain of ["SRE_SAAS", "SRE_IAAS"] as const) {
      const { unmount } = render(<CaseAlertSettings layout="full" chain={chain} />);
      const rows = within(
        screen.getByRole("table", { name: "Chat space for each team" }),
      ).getAllByRole("row");
      expect(
        rows.map((r) => r.querySelector("b")?.textContent).slice(1),
      ).toEqual(["All SRE teams (shared space)", "Apollo", "IaaS"]);
      unmount();
    }
  });

  it("says when nothing is saved here", () => {
    settings.data = make({ rooms: [] });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(
      screen.getByText(
        "No chat space saved here yet. Alerts go to the chat spaces in the alert service's file.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("table", { name: "Chat space for each team" }),
    ).not.toBeInTheDocument();
  });

  it("shows the CRE shared space on the CRE chain, never an SRE team", () => {
    settings.data = make({
      rooms: [
        apollo,
        room({ id: "cre", ladder: "CRE", label: "CRE Case Alerts" }),
      ],
    });
    render(<CaseAlertSettings layout="full" chain="CRE" />);
    expect(chatRow("All CRE teams (shared space)")).toBeInTheDocument();
    expect(screen.queryByText("Apollo SRE")).not.toBeInTheDocument();
  });

  it("closes the More menu on Escape and on a click outside", () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const more = within(chatRow("Apollo")).getByRole("button", {
      name: "More",
    });
    expect(more).toHaveAttribute("aria-haspopup", "menu");
    fireEvent.click(more);
    expect(screen.getByRole("menuitem", { name: "Turn off" })).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("menuitem", { name: "Turn off" }), {
      key: "ArrowDown",
    });
    expect(screen.getByRole("menuitem", { name: "Remove" })).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("menuitem", { name: "Remove" }), {
      key: "Escape",
    });
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(more).toHaveFocus();
    fireEvent.click(more);
    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("shows nothing of a family whose settings the reader may not change", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["CRE"], routeLadders: ["CRE"] },
    });
    const { container } = render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("says when URLs can't be saved on this server, and can't add one", () => {
    settings.data = make({ webhooksEnabled: false });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(
      screen.getByText("Webhook URLs can't be saved on this server."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "+ Add chat space" }),
    ).toBeDisabled();
  });

});

/** The SRE tab's Submit, waiting until everything is saved. */
async function submit(family = "SRE"): Promise<void> {
  fireEvent.click(screen.getByRole("button", { name: `Submit ${family} changes` }));
  await waitFor(() =>
    expect(screen.getByRole("region", { name: `${family} changes to submit` })).toHaveTextContent(
      "No changes to submit.",
    ),
  );
}

function pending(family = "SRE"): HTMLElement {
  return screen.getByRole("region", { name: `${family} changes to submit` });
}

const SME_TEAMS = [
  { key: "asgardeo", name: "Asgardeo", family: "SME" as const },
  { key: "u2", name: "U2", family: "SME" as const },
];
const asgardeo = room({
  id: "asg-room",
  ladder: "SME",
  teamKey: "asgardeo",
  label: "Asgardeo SME",
  hasWebhook: true,
  webhookHint: HINT,
});

describe("Chat space for each team: changes wait for Submit", () => {
  it("offers the SRE teams' spaces under Advanced, and the SME teams after escalation, never a move-only team", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["SRE", "SME"], routeLadders: ["SRE"] },
      rooms: [apollo, iaas, asgardeo],
    });
    render(<CaseAlertSettings layout="full" family="SRE" teams={SME_TEAMS} moveTargetKeys={["zeus"]} />);
    const { dialog } = openAdd();
    expect(optionNames(within(dialog).getByLabelText("Team"))).toEqual([
      "All SRE teams (shared space)",
      "Apollo · has a space",
      "Artemis",
      "IaaS · has a space",
    ]);
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    const after = screen.getByRole("table", { name: "Each SME sub team's chat space" });
    expect(within(after).getAllByRole("row").slice(1).map((r) => (r as HTMLTableRowElement).cells[0].textContent)).toEqual([
      "Asgardeo",
      "U2",
    ]);
    expect(within(after).getByText(HINT)).toBeInTheDocument();
    expect(within(after).getByText(/^Not set · uses the shared SME space/)).toBeInTheDocument();
  });

  it("shows each family's table only to those who may change it", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] },
      rooms: [apollo, asgardeo],
    });
    render(<CaseAlertSettings layout="full" family="SRE" teams={SME_TEAMS} />);
    expect(within(chatRow("Apollo")).getByRole("button", { name: "Edit" })).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "After escalation" })).not.toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Before escalation" })).toBeInTheDocument();
  });

  it("needs a Google Chat webhook URL for a new space", () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    const url = within(dialog).getByLabelText(WEBHOOK);
    expect(url).toHaveAttribute("type", "password");
    expect(url).toBeRequired();
    expect(url.closest("label")?.querySelector(".MuiFormLabel-asterisk")).toHaveAttribute("aria-hidden", "true");
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(within(dialog).getByRole("alert")).toHaveTextContent("Paste the space's Google Chat webhook URL.");
    fireEvent.change(within(dialog).getByLabelText(WEBHOOK), {
      target: { value: "https://example.com/hook" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(within(dialog).getByRole("alert")).toHaveTextContent("isn't a Google Chat webhook URL");
    expect(pending()).toHaveTextContent("No changes to submit.");
  });

  it("does not mark the webhook URL once URLs can't be saved on this server, which disables the box", () => {
    const { rerender } = render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    openAdd();
    // The server stops taking URLs while the dialog is open.
    settings.data = make({ webhooksEnabled: false });
    rerender(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const url = within(screen.getByRole("dialog", { name: "Add a team's chat space" })).getByLabelText(WEBHOOK);
    expect(url).toBeDisabled();
    expect(url).not.toBeRequired();
    expect(url.closest("label")?.querySelector(".MuiFormLabel-asterisk")).toBeNull();
  });

  it("adds a team: nothing is saved until Submit, then the room and its URL", async () => {
    saveRoom.mutateAsync.mockResolvedValue({ ...apollo, id: "new-room" });
    setWebhook.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const { button, dialog } = openAdd();
    fireEvent.change(within(dialog).getByLabelText("Team"), { target: { value: "SRE|artemis" } });
    // Artemis starts as the file has it: it also gets incidents with no SRE team.
    expect(within(dialog).getByRole("checkbox", { name: "Also gets incidents with no SRE team" })).toBeChecked();
    expect(within(dialog).queryByLabelText("Alerts by")).not.toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText(WEBHOOK), { target: { value: HOOK } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(button).toHaveFocus();
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: Artemis chat space. Submit to save them.",
    );
    expect(pending()).toHaveTextContent("1 change not saved yet");
    expect(saveRoom.mutateAsync).not.toHaveBeenCalled();

    await submit();
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith({
      ladder: "SRE",
      teamKey: "artemis",
      label: "Artemis SRE",
      enabled: true,
      teamless: true,
      channel: "",
    });
    expect(setWebhook.mutateAsync).toHaveBeenCalledWith({ roomId: "new-room", url: HOOK });
    // Not reported yet: it is used from the next restart.
    expect(screen.getByText(/^✓ Saved/)).toHaveTextContent(`✓ Saved 1 change. ${DB}`);
  });

  it("changes a team that has a space when picked again: its URL kept unless a new one is typed", async () => {
    saveRoom.mutateAsync.mockResolvedValue(apollo);
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    fireEvent.change(within(dialog).getByLabelText("Team"), { target: { value: "SRE|apollo" } });
    expect(within(dialog).getByText(`Leave empty to keep the saved URL (${HINT}).`)).toBeInTheDocument();
    // Its URL is saved, so the box is no longer required.
    const url = within(dialog).getByLabelText(WEBHOOK);
    expect(url).not.toBeRequired();
    expect(url.closest("label")?.querySelector(".MuiFormLabel-asterisk")).toBeNull();
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Also gets incidents with no SRE team" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    await submit();
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ teamKey: "apollo", label: "Apollo SRE", teamless: false }),
    );
    expect(setWebhook.mutateAsync).not.toHaveBeenCalled();
  });

  it("edits a row: keeps the saved URL when left empty, and sends every field", async () => {
    saveRoom.mutateAsync.mockResolvedValue(iaas);
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const edit = within(chatRow("IaaS")).getByRole("button", { name: "Edit" });
    edit.focus();
    fireEvent.click(edit);
    const dialog = screen.getByRole("dialog", { name: "Edit IaaS chat space" });
    expect(within(dialog).queryByLabelText("Team")).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText(WEBHOOK)).not.toBeRequired();
    fireEvent.click(within(dialog).getByRole("checkbox", { name: "Also gets incidents with no SRE team" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(edit).toHaveFocus();
    await submit();
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith({
      ladder: "SRE",
      teamKey: "iaas",
      label: "IaaS SRE",
      enabled: true,
      teamless: true,
      channel: "call",
    });
    expect(setWebhook.mutateAsync).not.toHaveBeenCalled();
  });

  it("turns off and removes from the row as changes; Test sends at once", async () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const a = chatRow("Apollo");
    expect(within(a).getAllByRole("button").map((b) => b.textContent)).toEqual(["Edit", "Test", "More ▾"]);
    fireEvent.click(within(a).getByRole("button", { name: "Test" }));
    expect(testRoom.mutate).toHaveBeenCalledWith("apollo-room", expect.anything());

    fireEvent.click(within(a).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn off" }));
    fireEvent.click(within(a).getByRole("button", { name: "Turn off" }));
    fireEvent.click(within(chatRow("IaaS")).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Remove" }));
    fireEvent.click(within(chatRow("IaaS")).getByRole("button", { name: "Remove" }));
    expect(pending()).toHaveTextContent("Apollo chat space turned off");
    expect(pending()).toHaveTextContent("IaaS chat space removed");
    expect(saveRoom.mutateAsync).not.toHaveBeenCalled();
    expect(deleteRoom.mutateAsync).not.toHaveBeenCalled();

    await submit();
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith({
      ladder: "SRE",
      teamKey: "apollo",
      label: "Apollo SRE",
      enabled: false,
      teamless: true,
      channel: "",
    });
    expect(deleteRoom.mutateAsync).toHaveBeenCalledWith("iaas-room");
  });

  it("undoes one change, or discards them all, before Submit", () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn off" }));
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "Turn off" }));
    fireEvent.click(within(pending()).getByRole("button", { name: "Undo: Apollo chat space turned off" }));
    expect(pending()).toHaveTextContent("No changes to submit.");

    fireEvent.click(within(chatRow("IaaS")).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn off" }));
    fireEvent.click(within(chatRow("IaaS")).getByRole("button", { name: "Turn off" }));
    expect(pending()).toHaveTextContent("1 change not saved yet");
    fireEvent.click(within(pending()).getByRole("button", { name: "Discard" }));
    expect(pending()).toHaveTextContent("No changes to submit.");
    expect(within(pending()).getByRole("button", { name: "Submit SRE changes" })).toBeDisabled();
  });

  it("keeps a change that fails on Submit, and says why", async () => {
    saveRoom.mutateAsync.mockRejectedValueOnce(new Error("That webhook URL was not accepted."));
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    fireEvent.change(within(dialog).getByLabelText(WEBHOOK), { target: { value: HOOK } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    fireEvent.click(screen.getByRole("button", { name: "Submit SRE changes" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("That webhook URL was not accepted.");
    expect(pending()).toHaveTextContent("1 change not saved yet");
  });

  it("words the saved line for db mode, and lets it be dismissed", async () => {
    settings.data = make({ revision: { saved: 5, applied: 4, appliedMode: "db" } });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn off" }));
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "Turn off" }));
    await submit();
    expect(screen.getByText(/^✓ Saved/)).toHaveTextContent(`✓ Saved 1 change. ${DB}`);
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(screen.queryByText(/^✓ Saved/)).not.toBeInTheDocument();
  });

  it("words the saved line for shadow mode: compared, not used yet", async () => {
    settings.data = make({ revision: { saved: 5, applied: 4, appliedMode: "shadow" } });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn off" }));
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "Turn off" }));
    await submit();
    expect(screen.getByText(/^✓ Saved/)).toHaveTextContent(`✓ Saved 1 change. ${NOT_DB}`);
  });

  it("adds an SME team's space from its row: ladder SME, nothing but its URL", async () => {
    settings.data = make({
      rooms: [asgardeo],
      canManage: { view: true, roomLadders: ["SME"], routeLadders: [] },
    });
    saveRoom.mutateAsync.mockResolvedValue({ ...asgardeo, id: "u2-room" });
    setWebhook.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings layout="full" family="SRE" teams={SME_TEAMS} />);
    expect(screen.queryByRole("group", { name: "Before escalation" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Add a chat space URL for U2" }));
    const field = screen.getByLabelText("Google Chat webhook URL for U2");
    expect(field).toHaveFocus();
    // The only field of its form, so it is required (its label is for screen readers only).
    expect(field).toBeRequired();
    fireEvent.change(field, { target: { value: "https://example.com/hook" } });
    fireEvent.click(screen.getByRole("button", { name: "Add to changes" }));
    expect(screen.getByRole("alert")).toHaveTextContent("isn't a Google Chat webhook URL");
    fireEvent.change(field, { target: { value: HOOK } });
    fireEvent.click(screen.getByRole("button", { name: "Add to changes" }));
    expect(pending()).toHaveTextContent("U2 chat space added");
    expect(screen.getByText("New URL waits for Submit")).toBeInTheDocument();
    expect(saveRoom.mutateAsync).not.toHaveBeenCalled();
    await submit();
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith({ ladder: "SME", teamKey: "u2", label: "U2 SME" });
    expect(setWebhook.mutateAsync).toHaveBeenCalledWith({ roomId: "u2-room", url: HOOK });
  });
});

const IAAS_GROUP = { id: "77777777-7777-4777-8777-000000000001", name: "WSO2 IaaS Team", services: ["WSO2 IaaS"] };
const APOLLO_GROUP = {
  id: "77777777-7777-4777-8777-000000000002",
  name: "Apollo SRE Group",
  services: ["Asgardeo", "Moesif", "client-a", "client-b", "client-c"],
};
const ARTEMIS_GROUP = { id: "77777777-7777-4777-8777-000000000003", name: "Artemis SRE Group", services: ["Choreo"] };
const DEFAULT_GROUP = { id: "77777777-7777-4777-8777-000000000004", name: "Default SRE Group", services: [] };

/** The SRE ladder's shared space, with a URL saved here. */
const sharedSpace = room({ id: "shared-room", hasWebhook: true, webhookHint: "spaces/SHARED01 · …abcd" });

/** A row of the before-escalation table, by its group's name. */
function groupRow(name: string): HTMLTableRowElement {
  const table = screen.getByRole("table", { name: "Each assignment group's chat space" });
  const tr = within(table).getByText(name).closest("tr");
  if (!tr) throw new Error(`no row for ${name}`);
  return tr;
}

describe("Each assignment group's chat space", () => {
  const groupSettings = (over: Partial<Settings> = {}) =>
    make({
      rooms: [apollo, sharedSpace, iaas],
      routeOptions: {
        groups: [],
        teams: [
          { key: "apollo", name: "Apollo", family: "SRE" },
          { key: "artemis", name: "Artemis", family: "SRE" },
          { key: "iaas", name: "IaaS", family: "SRE" },
        ],
        rotas: [],
        assignmentGroups: [IAAS_GROUP, APOLLO_GROUP, ARTEMIS_GROUP, DEFAULT_GROUP],
      },
      running: {
        reportedAt: AT,
        mode: "db",
        ladders: {},
        groups: {
          SRE: {
            aliases: { "apollo sre group": "apollo", "artemis sre group": "artemis", "wso2 iaas team": "iaas" },
            defaultGroups: ["default sre group"],
            teamlessRooms: ["apollo", "artemis"],
          },
        },
      },
      ...over,
    });

  it("lists every assignment group with its SRE sub team, its services and where its alerts go", () => {
    settings.data = groupSettings();
    render(<CaseAlertSettings layout="full" family="SRE" />);
    const table = screen.getByRole("table", { name: "Each assignment group's chat space" });
    expect(within(table).getAllByRole("row").slice(1).map((r) => (r as HTMLTableRowElement).cells[0].textContent)).toEqual([
      "Apollo SRE Group",
      "Artemis SRE Group",
      "Default SRE Group",
      "WSO2 IaaS Team",
      "Any other group",
    ]);
    expect(groupRow("Apollo SRE Group").cells[1]).toHaveTextContent(/^Apollo$/);
    expect(groupRow("Apollo SRE Group").cells[2]).toHaveTextContent("Asgardeo, Moesif, client-a + 2 more");
    expect(groupRow("Default SRE Group").cells[1]).toHaveTextContent("Default SRE rota");
    // No space of its own: where its cards go now, from what is saved and reported.
    expect(groupRow("Apollo SRE Group")).toHaveTextContent("Not set · uses the Apollo SRE space");
    expect(groupRow("Artemis SRE Group")).toHaveTextContent("Not set · uses the shared SRE space (SRE Case Alerts)");
    expect(groupRow("Default SRE Group")).toHaveTextContent("Not set · uses the Apollo SRE space");
    expect(groupRow("Any other group")).toHaveTextContent("Uses the shared SRE space (SRE Case Alerts)");
  });

  it("shows a group's own space, and says when two groups share one", () => {
    const apolloSpace = room({
      id: "apollo-group-room",
      groupId: APOLLO_GROUP.id,
      groupName: APOLLO_GROUP.name,
      label: APOLLO_GROUP.name,
      hasWebhook: true,
      webhookHint: HINT,
      lastTestStatus: "delivered",
      lastTestAt: AT,
    });
    const artemisSpace = room({
      ...apolloSpace,
      id: "artemis-group-room",
      groupId: ARTEMIS_GROUP.id,
      groupName: ARTEMIS_GROUP.name,
      label: ARTEMIS_GROUP.name,
    });
    settings.data = groupSettings({ rooms: [apolloSpace, artemisSpace, sharedSpace] });
    render(<CaseAlertSettings layout="full" family="SRE" />);
    expect(groupRow("Apollo SRE Group")).toHaveTextContent("same space as Artemis SRE Group");
    expect(groupRow("Artemis SRE Group")).toHaveTextContent("same space as Apollo SRE Group");
    expect(within(groupRow("Apollo SRE Group")).getByRole("button", { name: /Change the chat space URL of/ })).toBeInTheDocument();
    fireEvent.click(within(groupRow("Apollo SRE Group")).getByRole("button", { name: /Send a test card/ }));
    expect(testRoom.mutate).toHaveBeenCalledWith("apollo-group-room", expect.anything());
  });

  it("adds a group's URL as a change: the room for the group, then its URL, on Submit", async () => {
    settings.data = groupSettings();
    saveRoom.mutateAsync.mockResolvedValue(room({ id: "iaas-group-room", groupId: IAAS_GROUP.id }));
    setWebhook.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings layout="full" family="SRE" />);
    fireEvent.click(screen.getByRole("button", { name: "Add a chat space URL for WSO2 IaaS Team" }));
    fireEvent.change(screen.getByLabelText("Google Chat webhook URL for WSO2 IaaS Team"), { target: { value: HOOK } });
    fireEvent.click(screen.getByRole("button", { name: "Add to changes" }));
    expect(pending()).toHaveTextContent("WSO2 IaaS Team chat space added");
    // Undo takes it back before anything is saved.
    fireEvent.click(within(groupRow("WSO2 IaaS Team")).getByRole("button", { name: "Undo" }));
    expect(pending()).toHaveTextContent("No changes to submit.");
    fireEvent.click(screen.getByRole("button", { name: "Add a chat space URL for WSO2 IaaS Team" }));
    fireEvent.change(screen.getByLabelText("Google Chat webhook URL for WSO2 IaaS Team"), { target: { value: HOOK } });
    fireEvent.click(screen.getByRole("button", { name: "Add to changes" }));
    await submit();
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith({ ladder: "SRE", groupId: IAAS_GROUP.id, label: "WSO2 IaaS Team" });
    expect(setWebhook.mutateAsync).toHaveBeenCalledWith({ roomId: "iaas-group-room", url: HOOK });
  });

  it("removes a group's space as a change, after asking", async () => {
    const iaasSpace = room({
      id: "iaas-group-room",
      groupId: IAAS_GROUP.id,
      groupName: IAAS_GROUP.name,
      label: IAAS_GROUP.name,
      hasWebhook: true,
      webhookHint: HINT,
    });
    settings.data = groupSettings({ rooms: [iaasSpace, iaas, sharedSpace] });
    deleteRoom.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings layout="full" family="SRE" />);
    fireEvent.click(within(groupRow("WSO2 IaaS Team")).getByRole("button", { name: /Remove WSO2 IaaS Team/ }));
    expect(groupRow("WSO2 IaaS Team")).toHaveTextContent("Remove? It then uses the shared SRE space (SRE Case Alerts).");
    fireEvent.click(within(groupRow("WSO2 IaaS Team")).getByRole("button", { name: "Remove" }));
    expect(groupRow("WSO2 IaaS Team")).toHaveTextContent("Removed when you Submit");
    await submit();
    expect(deleteRoom.mutateAsync).toHaveBeenCalledWith("iaas-group-room");
  });

  it("reads a redirect to the default rota as a default group, and says when no card is posted", () => {
    settings.data = groupSettings({
      rooms: [apollo],
      routes: [
        {
          id: "r-default",
          ladder: "SRE",
          groupName: "default sre group",
          groupId: DEFAULT_GROUP.id,
          rotaCode: "SRE_SAAS",
          enabled: true,
          updatedBy: "x",
          updatedOn: AT,
        },
        {
          id: "r-apollo",
          ladder: "SRE",
          groupName: "apollo sre group",
          groupId: APOLLO_GROUP.id,
          teamKey: "artemis",
          enabled: true,
          updatedBy: "x",
          updatedOn: AT,
        },
      ],
      running: {
        reportedAt: AT,
        mode: "db",
        ladders: {},
        groups: { SRE: { aliases: {}, defaultGroups: [], teamlessRooms: ["apollo"], sharedRoom: false } },
      },
    });
    render(<CaseAlertSettings layout="full" family="SRE" />);
    // A redirect shows where the group is called, wherever the file pointed it.
    expect(groupRow("Apollo SRE Group").cells[1]).toHaveTextContent(/^Artemis$/);
    // Redirected to the default rota: its cards go to the teamless spaces.
    expect(groupRow("Default SRE Group")).toHaveTextContent("Not set · uses the Apollo SRE space");
    // Artemis has no space and the ladder no shared one: nothing is posted.
    expect(groupRow("Apollo SRE Group")).toHaveTextContent("Not set · gets no card: the SRE ladder has no shared space");
    expect(groupRow("Any other group")).toHaveTextContent("Gets no card: the SRE ladder has no shared space");
  });

  it("offers no change to a reader who may not change SRE", () => {
    settings.data = groupSettings({ canManage: { view: true, roomLadders: ["SRE", "SME"], routeLadders: ["SRE"] } });
    render(<CaseAlertSettings layout="full" family="SRE" teams={SME_TEAMS} />);
    expect(screen.getAllByRole("button", { name: /Add a chat space URL for/ }).length).toBeGreaterThan(0);
  });
});
