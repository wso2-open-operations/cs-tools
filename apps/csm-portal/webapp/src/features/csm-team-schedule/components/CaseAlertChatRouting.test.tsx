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

/** A row of the chat routing table, by its team's name. */
function chatRow(name: string): HTMLElement {
  const table = screen.getByRole("table", { name: "Team chat routing" });
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
    name: "+ Add team chat routing",
  });
  button.focus();
  fireEvent.click(button);
  return {
    button,
    dialog: screen.getByRole("dialog", { name: "Add team chat routing" }),
  };
}

beforeEach(() => {
  vi.resetAllMocks();
  settings.data = make();
});

describe("Team chat routing", () => {
  it("says in the add dialog that a routing takes effect after the alert service restarts", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    expect(
      within(dialog).getByText(
        "Saved with this tab's Submit. The alert service uses it after its next restart.",
      ),
    ).toBeInTheDocument();
  });

  it("lists only the routings saved here, with the hint and last test: the space alone", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByRole("heading", { name: "Team chat routing" }),
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
    const table = screen.getByRole("table", { name: "Team chat routing" });
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
      const { unmount } = render(<CaseAlertSettings chain={chain} />);
      const rows = within(
        screen.getByRole("table", { name: "Team chat routing" }),
      ).getAllByRole("row");
      expect(
        rows.map((r) => r.querySelector("b")?.textContent).slice(1),
      ).toEqual(["All SRE teams (shared space)", "Apollo", "IaaS"]);
      unmount();
    }
  });

  it("says when nothing is saved here", () => {
    settings.data = make({ rooms: [] });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByText(
        "No team chat routing saved here yet. Alerts go to the chat spaces in the alert service's file.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("table", { name: "Team chat routing" }),
    ).not.toBeInTheDocument();
  });

  it("shows the CRE shared space on the CRE chain, never an SRE team", () => {
    settings.data = make({
      rooms: [
        apollo,
        room({ id: "cre", ladder: "CRE", label: "CRE Case Alerts" }),
      ],
    });
    render(<CaseAlertSettings chain="CRE" />);
    expect(chatRow("All CRE teams (shared space)")).toBeInTheDocument();
    expect(screen.queryByText("Apollo SRE")).not.toBeInTheDocument();
  });

  it("closes the More menu on Escape and on a click outside", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
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
    const { container } = render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("says when URLs can't be saved on this server, and can't add one", () => {
    settings.data = make({ webhooksEnabled: false });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    expect(
      screen.getByText("Webhook URLs can't be saved on this server."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "+ Add team chat routing" }),
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

describe("Team chat routing: changes wait for Submit", () => {
  it("offers every team of the reader's ladders, marking those with a space, and hides teams people are only moved onto", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["SRE", "SME"], routeLadders: ["SRE"] },
      rooms: [apollo, iaas, asgardeo],
    });
    render(<CaseAlertSettings family="SRE" teams={SME_TEAMS} moveTargetKeys={["zeus"]} />);
    const { dialog } = openAdd();
    const team = within(dialog).getByLabelText("Team");
    expect(optionNames(team)).toEqual([
      "All SRE teams (shared space)",
      "Apollo · has a space",
      "Artemis",
      "IaaS · has a space",
      "Asgardeo · has a space",
      "U2",
    ]);
    expect(within(team).getAllByRole("group").map((g) => g.getAttribute("label"))).toEqual(["SRE teams", "SME teams"]);
  });

  it("lists SRE and SME rows together, each changeable only by its own admin", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] },
      rooms: [apollo, asgardeo],
    });
    render(<CaseAlertSettings family="SRE" teams={SME_TEAMS} />);
    expect(within(chatRow("Apollo")).getByRole("button", { name: "Edit" })).toBeInTheDocument();
    expect(within(chatRow("Asgardeo")).queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
    const { dialog } = openAdd();
    expect(optionNames(within(dialog).getByLabelText("Team"))).not.toContain("U2");
  });

  it("needs a Google Chat webhook URL for a new space", () => {
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    expect(within(dialog).getByLabelText("Google Chat webhook URL")).toHaveAttribute("type", "password");
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(within(dialog).getByRole("alert")).toHaveTextContent("Paste the space's Google Chat webhook URL.");
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), {
      target: { value: "https://example.com/hook" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    expect(within(dialog).getByRole("alert")).toHaveTextContent("isn't a Google Chat webhook URL");
    expect(pending()).toHaveTextContent("No changes to submit.");
  });

  it("adds a team: nothing is saved until Submit, then the room and its URL", async () => {
    saveRoom.mutateAsync.mockResolvedValue({ ...apollo, id: "new-room" });
    setWebhook.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const { button, dialog } = openAdd();
    fireEvent.change(within(dialog).getByLabelText("Team"), { target: { value: "SRE|artemis" } });
    // Artemis starts as the file has it: it also gets incidents with no SRE team.
    expect(within(dialog).getByRole("checkbox", { name: "Also gets incidents with no SRE team" })).toBeChecked();
    expect(within(dialog).queryByLabelText("Alerts by")).not.toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), { target: { value: HOOK } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(button).toHaveFocus();
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: Artemis chat routing. Submit to save them.",
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
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    fireEvent.change(within(dialog).getByLabelText("Team"), { target: { value: "SRE|apollo" } });
    expect(within(dialog).getByText(`Leave empty to keep the saved URL (${HINT}).`)).toBeInTheDocument();
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
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const edit = within(chatRow("IaaS")).getByRole("button", { name: "Edit" });
    edit.focus();
    fireEvent.click(edit);
    const dialog = screen.getByRole("dialog", { name: "Edit IaaS chat routing" });
    expect(within(dialog).queryByLabelText("Team")).not.toBeInTheDocument();
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
    render(<CaseAlertSettings chain="SRE_SAAS" />);
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
    expect(pending()).toHaveTextContent("Apollo chat routing turned off");
    expect(pending()).toHaveTextContent("IaaS chat routing removed");
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
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn off" }));
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "Turn off" }));
    fireEvent.click(within(pending()).getByRole("button", { name: "Undo: Apollo chat routing turned off" }));
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
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    const { dialog } = openAdd();
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), { target: { value: HOOK } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    fireEvent.click(screen.getByRole("button", { name: "Submit SRE changes" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("That webhook URL was not accepted.");
    expect(pending()).toHaveTextContent("1 change not saved yet");
  });

  it("words the saved line for db mode, and lets it be dismissed", async () => {
    settings.data = make({ revision: { saved: 5, applied: 4, appliedMode: "db" } });
    render(<CaseAlertSettings chain="SRE_SAAS" />);
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
    render(<CaseAlertSettings chain="SRE_SAAS" />);
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "More" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Turn off" }));
    fireEvent.click(within(chatRow("Apollo")).getByRole("button", { name: "Turn off" }));
    await submit();
    expect(screen.getByText(/^✓ Saved/)).toHaveTextContent(`✓ Saved 1 change. ${NOT_DB}`);
  });

  it("adds an SME team's space with ladder SME and no SRE box", async () => {
    settings.data = make({
      rooms: [asgardeo],
      canManage: { view: true, roomLadders: ["SME"], routeLadders: [] },
    });
    saveRoom.mutateAsync.mockResolvedValue({ ...asgardeo, id: "u2-room" });
    setWebhook.mutateAsync.mockResolvedValue({});
    render(<CaseAlertSettings family="SRE" teams={SME_TEAMS} />);
    const { dialog } = openAdd();
    expect(optionNames(within(dialog).getByLabelText("Team"))).toEqual(["Asgardeo · has a space", "U2"]);
    fireEvent.change(within(dialog).getByLabelText("Team"), { target: { value: "SME|u2" } });
    expect(within(dialog).queryByRole("checkbox")).not.toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText("Google Chat webhook URL"), { target: { value: HOOK } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Add to changes" }));
    await submit();
    expect(saveRoom.mutateAsync).toHaveBeenCalledWith({
      ladder: "SME",
      teamKey: "u2",
      label: "U2 SME",
      enabled: true,
      teamless: false,
      channel: "",
    });
    expect(setWebhook.mutateAsync).toHaveBeenCalledWith({ roomId: "u2-room", url: HOOK });
  });
});
