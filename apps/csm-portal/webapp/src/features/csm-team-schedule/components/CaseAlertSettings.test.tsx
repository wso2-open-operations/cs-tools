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
const saveRoute = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const deleteRoute = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const saveValues = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };
const deleteValue = { mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false };

/** Presses the family's Submit and waits until it has saved everything. */
async function submit(family = "SRE"): Promise<void> {
  fireEvent.click(screen.getByRole("button", { name: `Submit ${family} changes` }));
  await waitFor(() =>
    expect(screen.getByRole("region", { name: `${family} changes to submit` })).toHaveTextContent(
      "No changes to submit.",
    ),
  );
}

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
  useSaveCaseAlertRoute: () => saveRoute,
  useDeleteCaseAlertRoute: () => deleteRoute,
  useSaveCaseAlertValues: () => saveValues,
  useDeleteCaseAlertValue: () => deleteValue,
}));

const APOLLO_ID = "22222222-2222-4222-8222-222222222222";
const HINT = "spaces/AAAAtest01 · …wxyz";

const apolloRoom: CaseAlertChatRoom = {
  id: APOLLO_ID,
  ladder: "SRE",
  teamKey: "apollo",
  label: "Apollo SRE",
  enabled: true,
  teamless: true,
  hasWebhook: true,
  webhookHint: HINT,
  webhookSetBy: "sre.rota@example.test",
  lastTestStatus: "delivered",
  updatedBy: "sre.rota@example.test",
  updatedOn: "2026-10-13T10:00:00Z",
};

function make(over: Partial<Settings> = {}): Settings {
  return {
    canManage: { view: true, roomLadders: ["SRE"], routeLadders: ["SRE"] },
    environment: "staging",
    webhooksEnabled: true,
    revision: { saved: 4 },
    rooms: [apolloRoom],
    routeOptions: {
      groups: [
        { key: "vega", name: "Vega", family: "CRE" },
        { key: "apollo", name: "Apollo", family: "SRE" },
        { key: "asgardeo", name: "Asgardeo", family: "SME" },
      ],
      teams: [
        { key: "vega", name: "Vega", family: "CRE" },
        { key: "apollo", name: "Apollo", family: "SRE" },
      ],
      rotas: [{ code: "SRE_SAAS", label: "SaaS SRE" }],
    },
    routes: [
      {
        id: "55555555-5555-4555-8555-555555555555",
        ladder: "SRE",
        groupName: "default sre group",
        rotaCode: "SRE_SAAS",
        enabled: true,
        updatedBy: "x",
        updatedOn: "2026-10-13T10:00:00Z",
      },
    ],
    values: [],
    ...over,
  };
}

/** What the alert service reported, with the SRE ladder on channel. */
function running(channel: string): Settings["running"] {
  return {
    reportedAt: "2026-10-13T10:00:00Z",
    mode: "shadow",
    ladders: {},
    fileOnly: { SRE: { channel } },
  };
}

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
  // "Where alerts go" starts closed; these tests work inside it, so it opens
  // as for a person who opened it before.
  window.localStorage.setItem("csm-ts.alert-settings.open", JSON.stringify(["SRE.where", "CRE.where", "SRE.advanced"]));
  settings.data = make();
});

describe("CaseAlertSettings", () => {
  it("shows nothing to somebody who may not see the settings", () => {
    settings.data = make({
      canManage: { view: false, roomLadders: [], routeLadders: [] },
    });
    const { container } = render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows SRE and SME together on the SRE tab: chat spaces, SRE redirects, both ladders' settings", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["SRE", "SME"], routeLadders: ["SRE"] },
    });
    render(<CaseAlertSettings layout="full" family="SRE" />);
    expect(screen.getByRole("heading", { name: "SRE settings" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Chat space for each team" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Assignment groups" })).toBeInTheDocument();
    // Both ladders' switches, open from the start.
    const switches = screen.getByRole("region", { name: "On/off and channel" });
    expect(within(switches).getByRole("heading", { name: "SRE ladder" })).toBeInTheDocument();
    expect(within(switches).getByRole("heading", { name: "SME ladder" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Advanced" })).toBeInTheDocument();
  });

  it("shows the SME rota admin SME's part of the SRE tab only: no redirects, no SRE settings", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["SME"], routeLadders: [] },
    });
    render(<CaseAlertSettings layout="full" family="SRE" />);
    expect(screen.getByRole("heading", { name: "SRE settings" })).toBeInTheDocument();
    expect(screen.queryByText(/redirect/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Assignment groups" })).not.toBeInTheDocument();
    // SME's switches alone: no SRE ladder, so no heading to tell two apart.
    const switches = screen.getByRole("region", { name: "On/off and channel" });
    expect(within(switches).queryByRole("heading", { name: "SRE ladder" })).not.toBeInTheDocument();
    expect(within(switches).getByText("Alerts on")).toBeInTheDocument();
  });

  it("shows nothing of a family whose settings the reader may not change", () => {
    for (const [family, ladders] of [
      ["CRE", ["SRE", "SME"]],
      ["SRE", ["CRE"]],
    ] as const) {
      settings.data = make({
        canManage: { view: true, roomLadders: [...ladders], routeLadders: [] },
      });
      const { container, unmount } = render(<CaseAlertSettings layout="full" family={family} />);
      expect(container).toBeEmptyDOMElement();
      unmount();
    }
  });

  it("says when saved changes wait for a restart, and when they are in use", () => {
    settings.data = make({
      revision: { saved: 5, applied: 4, appliedMode: "db" },
    });
    const { unmount } = render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(screen.getByText("Saved · applies after a restart")).toHaveAccessibleName(
      /^Status: Saved\. Changes apply the next time the alert service restarts/,
    );
    unmount();
    settings.data = make({
      revision: {
        saved: 5,
        applied: 5,
        appliedMode: "db",
        appliedAt: "2026-10-13T10:00:00Z",
      },
    });
    const { unmount: unmount2 } = render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(screen.getByText("In use", { selector: ".cas-state" })).toHaveAccessibleName(
      /^Status: In use by the alert service \(as the alert service reported at/,
    );
    // A chip alone, no line: nothing out of the ordinary.
    expect(screen.queryByText(/^In use by the alert service/)).not.toBeInTheDocument();
    unmount2();
    // Shadow mode is out of the ordinary: said in a line of its own too.
    settings.data = make({ revision: { saved: 5, applied: 5, appliedMode: "shadow" } });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(screen.getByText("Not in use: shadow mode")).toBeInTheDocument();
    expect(screen.getByText(/^Saved, not in use yet\./)).toBeInTheDocument();
  });

  it("adds a redirect and removes one as changes, saved together on Submit", async () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    fireEvent.change(screen.getByRole("combobox", { name: "When an incident is assigned to" }), {
      target: { value: "Asgardeo" },
    });
    fireEvent.change(screen.getByRole("combobox", { name: "send its alerts to" }), {
      target: { value: "team:apollo" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: redirect from Asgardeo to Apollo. Submit to save them.",
    );
    const redirect = screen
      .getByText("Every team on the SaaS SRE rota")
      .closest("tr") as HTMLElement;
    fireEvent.click(within(redirect).getByRole("button", { name: "Remove the redirect from default sre group" }));
    fireEvent.click(within(redirect).getByRole("button", { name: "Remove" }));
    const pending = screen.getByRole("region", { name: "SRE changes to submit" });
    expect(pending).toHaveTextContent("2 changes not saved yet");
    expect(saveRoute.mutateAsync).not.toHaveBeenCalled();
    await submit();
    expect(saveRoute.mutateAsync).toHaveBeenCalledWith({ ladder: "SRE", groupName: "Asgardeo", teamKey: "apollo" });
    expect(deleteRoute.mutateAsync).toHaveBeenCalledWith("55555555-5555-4555-8555-555555555555");
    expect(screen.getByText(/^✓ Saved/)).toHaveTextContent("✓ Saved 2 changes.");
  });

  it("redirects an assignment group picked from the list, by its id", async () => {
    const IAAS = "77777777-7777-4777-8777-000000000001";
    const base = make();
    settings.data = make({
      routeOptions: {
        ...base.routeOptions!,
        assignmentGroups: [
          { id: IAAS, name: "WSO2 IaaS Team" },
          // Named like a team above: offered there, not twice.
          { id: "77777777-7777-4777-8777-000000000002", name: "Apollo" },
        ],
      },
    });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(screen.getAllByRole("option", { name: "Apollo" })).toHaveLength(2); // the group list and the target list
    fireEvent.change(screen.getByRole("combobox", { name: "When an incident is assigned to" }), {
      target: { value: `\u0000ag:${IAAS}` },
    });
    fireEvent.change(screen.getByRole("combobox", { name: "send its alerts to" }), {
      target: { value: "team:apollo" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: redirect from WSO2 IaaS Team to Apollo. Submit to save them.",
    );
    await submit();
    expect(saveRoute.mutateAsync).toHaveBeenCalledWith({
      ladder: "SRE",
      groupName: "WSO2 IaaS Team",
      groupId: IAAS,
      teamKey: "apollo",
    });
  });

  it("gives a typed name the id of the one assignment group that has it", async () => {
    const IAAS = "77777777-7777-4777-8777-000000000001";
    const base = make();
    settings.data = make({
      routeOptions: { ...base.routeOptions!, assignmentGroups: [{ id: IAAS, name: "WSO2 IaaS Team" }] },
    });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    fireEvent.change(screen.getByRole("combobox", { name: "When an incident is assigned to" }), {
      target: { value: "\u0000other" },
    });
    fireEvent.change(screen.getByRole("textbox", { name: "Assignment group name" }), {
      target: { value: "wso2 iaas team" },
    });
    fireEvent.change(screen.getByRole("combobox", { name: "send its alerts to" }), {
      target: { value: "team:apollo" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    await submit();
    expect(saveRoute.mutateAsync).toHaveBeenCalledWith({
      ladder: "SRE",
      groupName: "wso2 iaas team",
      groupId: IAAS,
      teamKey: "apollo",
    });
  });

  it("warns about SRE assignment groups no redirect covers", () => {
    settings.data = make({
      unmappedGroups: [
        { id: "a1", name: "WSO2 IaaS Team" },
        { id: "a2", name: "U2 SRE Group" },
      ],
    });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(screen.getByRole("status")).toHaveTextContent(
      "2 assignment groups have no SRE team: their incident cards go to the shared SRE room and nobody is called. Redirect them below: WSO2 IaaS Team, U2 SRE Group.",
    );
    // The redirects sit under Advanced now, and its summary counts them.
    expect(summaryWith(/^Advanced · .* · 2 unmapped/)).toBeInTheDocument();
  });

  it("redirects an SRE assignment group that is not a CSM Portal team, typed by name", async () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    fireEvent.change(screen.getByRole("combobox", { name: "When an incident is assigned to" }), {
      target: { value: "\u0000other" },
    });
    fireEvent.change(screen.getByRole("combobox", { name: "send its alerts to" }), {
      target: { value: "team:apollo" },
    });
    // No name typed yet: nothing to add.
    expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();
    fireEvent.change(screen.getByRole("textbox", { name: "Assignment group name" }), {
      target: { value: "  WSO2 IaaS Team " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(screen.getByText(/^Added to this tab's changes:/)).toHaveTextContent(
      "Added to this tab's changes: redirect from WSO2 IaaS Team to Apollo. Submit to save them.",
    );
    expect(screen.queryByRole("textbox", { name: "Assignment group name" })).not.toBeInTheDocument();
    await submit();
    expect(saveRoute.mutateAsync).toHaveBeenCalledWith({ ladder: "SRE", groupName: "WSO2 IaaS Team", teamKey: "apollo" });
  });

  it("marks the redirect's group, a typed group name and where it goes required", () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const from = screen.getByRole("combobox", { name: "When an incident is assigned to" });
    const to = screen.getByRole("combobox", { name: "send its alerts to" });
    for (const field of [from, to]) {
      expect(field).toBeRequired();
      expect(field.closest("label")?.querySelector(".MuiFormLabel-asterisk")).toHaveAttribute("aria-hidden", "true");
    }
    fireEvent.change(from, { target: { value: "\u0000other" } });
    const typed = screen.getByRole("textbox", { name: "Assignment group name" });
    expect(typed).toBeRequired();
    expect(typed.closest("label")?.querySelector(".MuiFormLabel-asterisk")).toBeInTheDocument();
  });

  it("sends an SME team's incidents to a whole rota, picked from lists", async () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(
      screen.getByText("Every team on the SaaS SRE rota"),
    ).toBeInTheDocument();
    const from = screen.getByRole("combobox", { name: "When an incident is assigned to" });
    // SRE and SME teams for the SRE ladder; never a CRE team.
    expect(
      within(from).getByRole("option", { name: "Apollo" }),
    ).toBeInTheDocument();
    expect(
      within(from).getByRole("option", { name: "Asgardeo" }),
    ).toBeInTheDocument();
    expect(
      within(from).queryByRole("option", { name: "Vega" }),
    ).not.toBeInTheDocument();
    fireEvent.change(from, { target: { value: "Asgardeo" } });
    fireEvent.change(screen.getByRole("combobox", { name: "send its alerts to" }), {
      target: { value: "rota:SRE_SAAS" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    await submit();
    expect(saveRoute.mutateAsync).toHaveBeenCalledWith({ ladder: "SRE", groupName: "Asgardeo", rotaCode: "SRE_SAAS" });
  });

  it("offers the ladder's own teams, then whole rotas, as where alerts go", async () => {
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const to = screen.getByRole("combobox", { name: "send its alerts to" });
    expect(
      within(to).getByRole("option", { name: "Apollo" }),
    ).toBeInTheDocument();
    expect(
      within(to).getByRole("option", { name: "Every team on SaaS SRE" }),
    ).toBeInTheDocument();
    expect(
      within(to).queryByRole("option", { name: "Vega" }),
    ).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole("combobox", { name: "When an incident is assigned to" }), {
      target: { value: "Asgardeo" },
    });
    fireEvent.change(to, { target: { value: "team:apollo" } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    await submit();
    expect(saveRoute.mutateAsync).toHaveBeenCalledWith({ ladder: "SRE", groupName: "Asgardeo", teamKey: "apollo" });
  });

  it("says where a redirected incident's alerts end up: the team's space and channel", () => {
    settings.data = make({
      running: running("both"),
      routes: [
        {
          id: "77777777-7777-4777-8777-777777777777",
          ladder: "SRE",
          groupName: "asgardeo",
          teamKey: "apollo",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const redirect = screen
      .getByRole("cell", { name: "Asgardeo" })
      .closest("tr") as HTMLElement;
    expect(within(redirect).getByText("Apollo")).toBeInTheDocument();
    expect(within(redirect).getByText("Redirect saved here")).toBeInTheDocument();
    expect(
      within(redirect).getByText("Calls and chat · in the Apollo SRE space"),
    ).toBeInTheDocument();
  });

  it("keeps a redirect from a team whose name is not its key: the alert service needs it", () => {
    settings.data = make({
      routes: [
        {
          id: "77777777-7777-4777-8777-777777777777",
          ladder: "SRE",
          groupName: "apollo sre group",
          teamKey: "apollo",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(
      screen.queryByText(
        "Same team, not needed: it is alerted as itself anyway",
      ),
    ).not.toBeInTheDocument();
  });

  it("marks a redirect from a team to itself as not needed", () => {
    settings.data = make({
      routes: [
        {
          id: "77777777-7777-4777-8777-777777777777",
          ladder: "SRE",
          groupName: "apollo",
          teamKey: "apollo",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(
      screen.getByText("Same team, not needed: it is alerted as itself anyway"),
    ).toBeInTheDocument();
  });

  it("names only the channel for a team that gets no chat cards", () => {
    settings.data = make({
      running: running("both"),
      rooms: [{ ...apolloRoom, channel: "call" }],
      routes: [
        {
          id: "77777777-7777-4777-8777-777777777777",
          ladder: "SRE",
          groupName: "asgardeo",
          teamKey: "apollo",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const redirect = screen
      .getByRole("cell", { name: "Asgardeo" })
      .closest("tr") as HTMLElement;
    expect(within(redirect).getByText("Calls only")).toBeInTheDocument();
  });

  it("offers CRE teams only on the CRE chain, and no rotas", () => {
    settings.data = make({
      canManage: { view: true, roomLadders: ["CRE"], routeLadders: ["CRE"] },
    });
    render(<CaseAlertSettings layout="full" chain="CRE" />);
    const from = screen.getByRole("combobox", { name: "When a case is assigned to" });
    expect(
      within(from).getByRole("option", { name: "Vega" }),
    ).toBeInTheDocument();
    expect(
      within(from).queryByRole("option", { name: "Asgardeo" }),
    ).not.toBeInTheDocument();
    expect(
      within(screen.getByRole("combobox", { name: "send its alerts to" })).queryByRole("group", {
        name: "A whole rota",
      }),
    ).not.toBeInTheDocument();
  });

  it("names the parts: each team's chat space, and where an incident's or a case's alerts go", () => {
    const { unmount } = render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(
      screen.getByRole("heading", { name: "Chat space for each team" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Assignment groups" })).toBeInTheDocument();
    // How a team is alerted stays in the file: no column for it.
    expect(
      screen.queryByRole("columnheader", { name: "Alerts by" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Assignment group" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "Alerts go to" }),
    ).toBeInTheDocument();
    unmount();
    settings.data = make({
      canManage: { view: true, roomLadders: ["CRE"], routeLadders: ["CRE"] },
      routes: [],
    });
    render(<CaseAlertSettings layout="full" chain="CRE" />);
    expect(
      screen.getByRole("group", {
        name: "Send a case's alerts to another CRE team",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "No redirects. Each case is alerted to the CRE team it's assigned to.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Other teams")).not.toBeInTheDocument();
    // A CRE case's group is always a CSM Portal team: no typed name.
    expect(
      screen.queryByRole("option", { name: "Another assignment group…" }),
    ).not.toBeInTheDocument();
  });

});

describe("Where each assignment group's alerts go", () => {
  const IAAS = "77777777-7777-4777-8777-000000000001";
  const APOLLO_GROUP = "77777777-7777-4777-8777-000000000002";
  const DEFAULT = "77777777-7777-4777-8777-000000000003";
  const U2 = "77777777-7777-4777-8777-000000000004";
  const ARTEMIS = "77777777-7777-4777-8777-000000000005";

  function withGroups(over: Partial<Settings> = {}): Settings {
    const base = make({ routes: [] });
    return make({
      routes: [],
      routeOptions: {
        ...base.routeOptions!,
        teams: [...base.routeOptions!.teams, { key: "iaas", name: "IaaS", family: "SRE" }],
        assignmentGroups: [
          { id: IAAS, name: "WSO2 IaaS Team" },
          { id: APOLLO_GROUP, name: "Apollo" },
          { id: DEFAULT, name: "Default SRE Group" },
          { id: U2, name: "U2 SRE Group" },
          { id: ARTEMIS, name: "Artemis SRE Group" },
        ],
      },
      running: {
        reportedAt: "2026-10-13T10:00:00Z",
        mode: "db",
        ladders: {},
        fileOnly: { SRE: { channel: "both" } },
        groups: {
          SRE: {
            aliases: { "wso2 iaas team": "iaas", "artemis sre group": "artemis" },
            defaultGroups: ["default sre group"],
          },
        },
      },
      ...over,
    });
  }

  function groupRow(name: string): HTMLElement {
    const table = screen.getByRole("table", { name: "Where each assignment group's alerts go" });
    return within(table).getByRole("cell", { name }).closest("tr") as HTMLElement;
  }

  it("says for each group where its alerts go and why: the file, a team's name, or nowhere", () => {
    settings.data = withGroups({ unmappedGroups: [{ id: U2, name: "U2 SRE Group" }] });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(within(groupRow("WSO2 IaaS Team")).getByText("IaaS")).toBeInTheDocument();
    expect(within(groupRow("WSO2 IaaS Team")).getByText("From the alert service's file")).toBeInTheDocument();
    // A team key the page has no name for is shown as the key.
    expect(within(groupRow("Artemis SRE Group")).getByText("artemis")).toBeInTheDocument();
    expect(within(groupRow("Default SRE Group")).getByText("The default SRE rota")).toBeInTheDocument();
    expect(within(groupRow("Apollo")).getByText("A team has this name")).toBeInTheDocument();
    expect(within(groupRow("Apollo")).getByText("Calls and chat · in the Apollo SRE space")).toBeInTheDocument();
    const u2 = groupRow("U2 SRE Group");
    expect(within(u2).getByText("The shared SRE room")).toBeInTheDocument();
    expect(within(u2).getByText("Not mapped: nobody is called")).toBeInTheDocument();
    // The shared room's groups come first.
    const names = within(screen.getByRole("table", { name: "Where each assignment group's alerts go" }))
      .getAllByRole("row")
      .slice(1)
      .map((r) => r.querySelector("td")?.textContent);
    expect(names[0]).toBe("U2 SRE Group");
  });

  it("puts a redirect saved here above the file", () => {
    settings.data = withGroups({
      routes: [
        {
          id: "88888888-8888-4888-8888-888888888888",
          ladder: "SRE",
          groupName: "wso2 iaas team",
          groupId: IAAS,
          rotaCode: "SRE_SAAS",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const iaas = groupRow("WSO2 IaaS Team");
    expect(within(iaas).getByText("Every team on the SaaS SRE rota")).toBeInTheDocument();
    expect(within(iaas).getByText("Redirect saved here")).toBeInTheDocument();
    expect(within(iaas).getByRole("button", { name: "Change WSO2 IaaS Team" })).toBeInTheDocument();
    expect(within(iaas).getByRole("button", { name: "Remove the redirect from WSO2 IaaS Team" })).toBeInTheDocument();
    // A group with no redirect offers one, and nothing to remove.
    expect(within(groupRow("U2 SRE Group")).getByRole("button", { name: "Redirect U2 SRE Group" })).toBeInTheDocument();
    expect(within(groupRow("U2 SRE Group")).queryByRole("button", { name: /^Remove/ })).not.toBeInTheDocument();
  });

  it("fills the form from a row: Redirect picks the group, Change also where it goes now", async () => {
    settings.data = withGroups({
      routes: [
        {
          id: "88888888-8888-4888-8888-888888888888",
          ladder: "SRE",
          groupName: "wso2 iaas team",
          groupId: IAAS,
          rotaCode: "SRE_SAAS",
          enabled: true,
          updatedBy: "x",
          updatedOn: "2026-10-13T10:00:00Z",
        },
      ],
    });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    const from = screen.getByRole("combobox", { name: "When an incident is assigned to" });
    const to = screen.getByRole("combobox", { name: "send its alerts to" });
    fireEvent.click(within(groupRow("U2 SRE Group")).getByRole("button", { name: "Redirect U2 SRE Group" }));
    expect(from).toHaveValue(`\u0000ag:${U2}`);
    expect(to).toHaveValue("");
    expect(to).toHaveFocus();
    fireEvent.change(to, { target: { value: "team:apollo" } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));

    fireEvent.click(within(groupRow("WSO2 IaaS Team")).getByRole("button", { name: "Change WSO2 IaaS Team" }));
    expect(from).toHaveValue(`\u0000ag:${IAAS}`);
    expect(to).toHaveValue("rota:SRE_SAAS");
    fireEvent.change(to, { target: { value: "team:iaas" } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    await submit();
    expect(saveRoute.mutateAsync).toHaveBeenNthCalledWith(1, {
      ladder: "SRE",
      groupName: "U2 SRE Group",
      groupId: U2,
      teamKey: "apollo",
    });
    // The same group again: the server replaces its redirect.
    expect(saveRoute.mutateAsync).toHaveBeenNthCalledWith(2, {
      ladder: "SRE",
      groupName: "WSO2 IaaS Team",
      groupId: IAAS,
      teamKey: "iaas",
    });
  });

  it("names a group the file doesn't report as not mapped, and says nothing about the file", () => {
    settings.data = withGroups({ running: undefined });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(within(groupRow("WSO2 IaaS Team")).getByText("The shared SRE room")).toBeInTheDocument();
    expect(screen.queryByText("From the alert service's file")).not.toBeInTheDocument();
  });

  it("offers no change to a reader who may not change SRE's redirects", () => {
    settings.data = withGroups({ canManage: { view: true, roomLadders: ["SRE"], routeLadders: [] } });
    render(<CaseAlertSettings layout="full" chain="SRE_SAAS" />);
    expect(within(groupRow("U2 SRE Group")).queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "When an incident is assigned to" })).not.toBeInTheDocument();
  });
});
