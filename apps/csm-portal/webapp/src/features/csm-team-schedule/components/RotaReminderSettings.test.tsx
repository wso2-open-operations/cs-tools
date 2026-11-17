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

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { RotaReminderSettings as Settings, SaveRotaReminderSettings } from "../types";
// vi.mock below is hoisted above this import, so the card gets the mocks.
import RotaReminderSettings from "./RotaReminderSettings";

const settings: { data: Settings | null | undefined; isLoading: boolean; isError: boolean } = {
  data: null,
  isLoading: false,
  isError: false,
};
const saving = { pending: false };
const mutate = vi.fn();

vi.mock("../api/useRotaReminderSettings", () => ({
  useRotaReminderSettings: () => ({ data: settings.data, isLoading: settings.isLoading, isError: settings.isError }),
  useSaveRotaReminderSettings: () => ({ mutate, isPending: saving.pending }),
}));

// The Send now section has its own tests; here only the settings form.
vi.mock("./RotaReminderSendNow", () => ({ default: () => null }));

// The people the Cc pickers offer: CRE team members and heads, CRE rota
// admins (one on no team), and an SRE member and an inactive grant that must
// not be offered.
vi.mock("../api/useTeamSchedule", () => ({
  useScheduleCatalogue: () => ({
    data: {
      teams: [
        {
          key: "vega", name: "Vega", family: "CRE", sortOrder: 1,
          members: [
            { userId: "1", name: "Ana Lead", email: "a@x.test", isLead: true, role: "lead" },
            { userId: "2", name: "Bo Engineer", email: "B@x.test", isLead: false, role: "engineer" },
          ],
          heads: [{ userId: "3", name: "Cy Head", email: "head@x.test", isLead: false, role: "americas_team_lead" }],
        },
        {
          key: "apollo", name: "Apollo", family: "SRE", sortOrder: 2,
          members: [{ userId: "4", name: "Sre Person", email: "sre@x.test", isLead: false, role: "engineer" }],
        },
      ],
    },
  }),
}));
vi.mock("../api/useRotaAccess", () => ({
  useRotaAccess: () => ({
    data: {
      rotaAdmins: [
        { userId: "5", name: "Dee Admin", email: "admin@x.test", family: "CRE", active: true, grantedBy: "", grantedOn: "" },
        { userId: "6", name: "Old Admin", email: "old@x.test", family: "CRE", active: false, grantedBy: "", grantedOn: "" },
        { userId: "7", name: "Sre Admin", email: "sreadmin@x.test", family: "SRE", active: true, grantedBy: "", grantedOn: "" },
      ],
    },
  }),
}));

type Callbacks ={ onSuccess?: (s: Settings) => void; onError?: (e: Error) => void };

const LA = { start: "08:00", end: "18:00", timeZone: "America/Los_Angeles" };
const ALL_INVITES_ON = { CRE_MORNING: true, CRE_EVENING: true, CRE_WEEKEND: true, CRE_WEEKEND_NIGHT: true };
const invitesFor = () => screen.getByRole("group", { name: "Calendar invitations for" });

const LOADED: Settings = {
  family: "CRE",
  weekdayEnabled: true,
  weekendEnabled: false,
  invitesEnabled: true,
  weekendRequired: { CRE_WEEKEND: 3, CRE_WEEKEND_NIGHT: 1, OTHER_CODE: 2 },
  windows: {
    CRE_WEEKEND_NIGHT: LA,
    CRE_WEEKEND_NIGHT_OC: LA,
    OTHER_WINDOW: { start: "06:00", end: "07:00", timeZone: "UTC" },
  },
  copyTo: [],
  updatedBy: "ana@example.test",
  updatedOn: "2026-10-01T09:30:00Z",
};

// The required fields' labels end in an aria-hidden asterisk: role queries
// (which use the accessible name) are unaffected, and a time input, which has
// no role, is found by its label from the start.
const LK = "People needed on LK Weekend each day";
const AMERICAS = "People needed on Americas Weekend each day";
const lkField = () => screen.getByRole("spinbutton", { name: LK });
const americasField = () => screen.getByRole("spinbutton", { name: AMERICAS });
const startField = () => screen.getByLabelText(/^Start\b/);
const endField = () => screen.getByLabelText(/^End\b/);
const zoneField = () => screen.getByRole("combobox", { name: "Time zone" });
const copyField = () => screen.getByLabelText("Cc on reminder emails");

/** Picks a member in a Cc picker: type part of the name, choose the match. */
function pick(label: string, typed: string, name: RegExp | string): void {
  const input = screen.getByLabelText(label);
  // A real focus, so the picker focused before it loses focus and closes.
  act(() => input.focus());
  fireEvent.change(input, { target: { value: typed } });
  fireEvent.mouseDown(within(screen.getByRole("listbox")).getByRole("option", { name }));
}
const saveButton = () => screen.getByRole("button", { name: /^(Save reminder settings|Saving…)$/ });
const discardButton = () => screen.getByRole("button", { name: "Discard changes" });

function lastBody(): SaveRotaReminderSettings {
  return mutate.mock.calls.at(-1)?.[0] as SaveRotaReminderSettings;
}

beforeEach(() => {
  mutate.mockReset();
  saving.pending = false;
  settings.data = null;
  settings.isLoading = false;
  settings.isError = false;
});

describe("RotaReminderSettings", () => {
  it("shows the built-in settings with the automatic reminders off, and says nothing is saved yet, when nothing was saved", () => {
    render(<RotaReminderSettings />);
    expect(screen.getByRole("heading", { name: "CRE rota reminders" })).toBeInTheDocument();
    expect(screen.getByText("Built-in settings")).toBeInTheDocument();
    expect(screen.queryByText("Saved")).not.toBeInTheDocument();
    expect(screen.getByText(/^Not saved yet: no automatic reminder is sent until one is turned on here and saved/)).toBeInTheDocument();
    expect(screen.getByText(/^The only on\/off for the automatic reminders in this environment/)).toBeInTheDocument();
    expect(screen.queryByText(/^Last changed by/)).not.toBeInTheDocument();

    // Nothing saved means nothing scheduled: the card must not show them as on.
    expect(screen.getByRole("checkbox", { name: "Send weekday reminders" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Send weekend reminders" })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Attach calendar invitations" })).toBeChecked();
    expect(lkField()).toHaveValue(3);
    expect(americasField()).toHaveValue(1);
    expect(startField()).toHaveValue("08:00");
    expect(endField()).toHaveValue("18:00");
    expect(zoneField()).toHaveValue("America/Los_Angeles");
    expect(copyField()).toHaveValue("");

    expect(saveButton()).toBeDisabled();
    expect(discardButton()).toBeDisabled();
  });

  it("shows who last changed saved settings, and no built-in note", () => {
    settings.data = LOADED;
    render(<RotaReminderSettings />);
    expect(screen.getByText("Saved")).toBeInTheDocument();
    expect(screen.queryByText("Built-in settings")).not.toBeInTheDocument();
    expect(screen.getByText(/Last changed by ana@example\.test on/)).toBeInTheDocument();
    expect(screen.queryByText(/^Not saved yet/)).not.toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Send weekend reminders" })).not.toBeChecked();
  });

  it("says when each reminder goes out: 09:00 IST the day before, the weekend on Monday and Friday", () => {
    render(<RotaReminderSettings />);
    expect(screen.getByRole("checkbox", { name: "Send weekday reminders" })).toHaveAccessibleDescription(
      /one email at 09:00 IST the day before \(Sunday to Thursday; on Sunday for Monday\), with a calendar invitation unless invitations are turned off below/,
    );
    const weekend = screen.getByRole("checkbox", { name: "Send weekend reminders" });
    expect(weekend).toHaveAccessibleDescription(/Sent at 09:00 IST on Monday once the weekend is fully marked/);
    expect(weekend).toHaveAccessibleDescription(/On Friday at 09:00 IST a final reminder with any changes goes to everyone on the weekend, even if gaps remain/);
    expect(weekend).toHaveAccessibleDescription(/only the CRE rota admins hear about it: a 09:00 IST email each morning, Monday to Thursday/);
    expect(screen.getByText(/an email can arrive up to an hour later/)).toBeInTheDocument();
  });

  it("groups the settings under Settings, in Automatic reminders, Weekend rota and Cc", () => {
    render(<RotaReminderSettings />);
    expect(screen.getByRole("heading", { name: "Settings", level: 4 })).toBeInTheDocument();

    const automatic = within(screen.getByRole("group", { name: "Automatic reminders" }));
    expect(automatic.getByRole("checkbox", { name: "Send weekday reminders" })).toBeInTheDocument();
    expect(automatic.getByRole("checkbox", { name: "Send weekend reminders" })).toBeInTheDocument();
    expect(automatic.getByRole("checkbox", { name: "Attach calendar invitations" })).toBeInTheDocument();

    const weekend = within(screen.getByRole("group", { name: "Weekend rota" }));
    expect(weekend.getByRole("spinbutton", { name: LK })).toBeInTheDocument();
    expect(weekend.getByRole("spinbutton", { name: AMERICAS })).toBeInTheDocument();
    const hours = within(weekend.getByRole("group", { name: "Americas Weekend and Americas On-call hours" }));
    expect(hours.getByLabelText(/^Start\b/)).toBeInTheDocument();
    expect(hours.getByLabelText(/^End\b/)).toBeInTheDocument();
    expect(hours.getByRole("combobox", { name: "Time zone" })).toBeInTheDocument();

    const copies = within(screen.getByRole("group", { name: "Cc" }));
    expect(copies.getByLabelText("Cc on reminder emails")).toHaveAttribute("role", "combobox");
    expect(copies.getByLabelText("Cc on calendar invitations")).toHaveAttribute("role", "combobox");
    expect(copies.getByLabelText("Cc on weekend reminders only")).toHaveAttribute("role", "combobox");
  });

  it("marks the head counts, the hours and the time zone required, and the copies optional", () => {
    render(<RotaReminderSettings />);
    for (const field of [lkField(), americasField(), startField(), endField(), zoneField()]) {
      expect(field).toBeRequired();
      const label = document.querySelector(`label[for="${field.id}"]`);
      expect(label?.querySelector(".MuiFormLabel-asterisk")).toHaveAttribute("aria-hidden", "true");
    }
    const copy = copyField();
    expect(copy).not.toBeRequired();
    expect(document.querySelector(`label[for="${copy.id}"] .MuiFormLabel-asterisk`)).toBeNull();
  });

  it("saves the whole body: both windows set, other saved codes kept, addresses trimmed and deduped", () => {
    settings.data = LOADED;
    render(<RotaReminderSettings />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Send weekend reminders" }));
    fireEvent.change(lkField(), { target: { value: "4" } });
    fireEvent.change(americasField(), { target: { value: "2" } });
    fireEvent.change(startField(), { target: { value: "09:00" } });
    fireEvent.change(endField(), { target: { value: "17:30" } });
    fireEvent.change(zoneField(), { target: { value: "Asia/Colombo" } });
    pick("Cc on reminder emails", "ana", /Ana Lead/);
    pick("Cc on reminder emails", "bo", /Bo Engineer/);
    expect(saveButton()).toBeEnabled();
    fireEvent.click(saveButton());

    const hours = { start: "09:00", end: "17:30", timeZone: "Asia/Colombo" };
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(lastBody()).toEqual({
      weekdayEnabled: true,
      weekendEnabled: true,
      invitesEnabled: true,
      weekendRequired: { CRE_WEEKEND: 4, CRE_WEEKEND_NIGHT: 2, OTHER_CODE: 2 },
      windows: {
        CRE_WEEKEND_NIGHT: hours,
        CRE_WEEKEND_NIGHT_OC: hours,
        OTHER_WINDOW: { start: "06:00", end: "07:00", timeZone: "UTC" },
      },
      inviteRotations: ALL_INVITES_ON,
      copyTo: ["a@x.test", "B@x.test"],
      weekendCopyTo: [],
      inviteCopyTo: [],
    });
  });

  it("a first save keeps the deployment's minimums and hours when they were not changed", () => {
    render(<RotaReminderSettings />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Attach calendar invitations" }));
    fireEvent.click(saveButton());
    expect(lastBody()).toEqual({
      weekdayEnabled: false,
      weekendEnabled: false,
      invitesEnabled: false,
      weekendRequired: {},
      windows: {},
      inviteRotations: ALL_INVITES_ON,
      copyTo: [],
      weekendCopyTo: [],
      inviteCopyTo: [],
    });
  });

  it("saves the three Cc lists apart: reminder emails, calendar invitations, weekend summary", () => {
    settings.data = { ...LOADED, copyTo: ["a@x.test"] };
    render(<RotaReminderSettings />);
    pick("Cc on calendar invitations", "dee", /Dee Admin/);
    pick("Cc on weekend reminders only", "head", /Cy Head/);
    fireEvent.click(saveButton());
    expect(lastBody()).toMatchObject({
      copyTo: ["a@x.test"],
      inviteCopyTo: ["admin@x.test"],
      weekendCopyTo: ["head@x.test"],
    });
  });

  it("offers only active CRE members, team members, heads and rota admins, by name or email", () => {
    render(<RotaReminderSettings />);
    const input = copyField();
    act(() => input.focus());
    const offered = within(screen.getByRole("listbox"))
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(offered).toEqual([
      "Ana Leada@x.test",
      "Bo EngineerB@x.test",
      "Cy Headhead@x.test",
      "Dee Adminadmin@x.test",
    ]);
    fireEvent.change(input, { target: { value: "admin@" } });
    expect(within(screen.getByRole("listbox")).getAllByRole("option")).toHaveLength(1);
    fireEvent.change(input, { target: { value: "sre" } });
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("No CRE member matches");
  });

  it("does not offer somebody already picked, and Enter picks the highlighted match without saving", () => {
    settings.data = { ...LOADED, copyTo: ["a@x.test"] };
    render(<RotaReminderSettings />);
    const input = copyField();
    act(() => input.focus());
    expect(within(screen.getByRole("listbox")).queryByRole("option", { name: /Ana Lead/ })).not.toBeInTheDocument();
    fireEvent.change(input, { target: { value: "bo" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(mutate).not.toHaveBeenCalled();
    fireEvent.click(saveButton());
    expect(lastBody()).toMatchObject({ copyTo: ["a@x.test", "B@x.test"] });
  });

  it("closes the list after a pick, so it never covers Save, and opens it again on typing", () => {
    render(<RotaReminderSettings />);
    pick("Cc on calendar invitations", "dee", /Dee Admin/);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Cc on calendar invitations"), { target: { value: "c" } });
    expect(screen.getByRole("listbox")).toBeInTheDocument();
  });

  it("shows a saved address that is not a CRE member as a marked chip that can be removed", () => {
    settings.data = { ...LOADED, weekendCopyTo: ["head@x.test", "outsider@y.test"] };
    render(<RotaReminderSettings />);
    expect(screen.getByText("Cy Head")).toBeInTheDocument();
    expect(screen.getByText("outsider@y.test")).toBeInTheDocument();
    expect(screen.getByText("Not a CRE member")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Remove outsider@y.test" }));
    fireEvent.click(saveButton());
    expect(lastBody()).toMatchObject({ weekendCopyTo: ["head@x.test"] });
  });

  it("shows the saved invitation Cc, and refuses a saved address that is not an email", () => {
    settings.data = { ...LOADED, inviteCopyTo: ["admin@x.test", "not an address"] };
    render(<RotaReminderSettings />);
    expect(screen.getByText("Dee Admin")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "Send weekend reminders" }));
    fireEvent.click(saveButton());
    expect(mutate).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent('"not an address" is not an email address.');
  });

  it("offers a calendar invitation switch per rotation, all on until one is turned off", () => {
    render(<RotaReminderSettings />);
    const group = invitesFor();
    for (const name of ["Morning", "Evening", "LK Weekend (IST)", "Americas Weekend (PT)"]) {
      expect(within(group).getByRole("checkbox", { name })).toBeChecked();
    }
  });

  it("names the Americas Weekend switch and explains daylight saving by the zone its hours are kept on", () => {
    render(<RotaReminderSettings />);
    const hours = () => screen.getByRole("group", { name: "Americas Weekend and Americas On-call hours" });
    expect(within(invitesFor()).getByRole("checkbox", { name: "Americas Weekend (PT)" })).toBeInTheDocument();
    expect(hours()).toHaveTextContent(/daylight saving/);

    fireEvent.change(zoneField(), { target: { value: "Asia/Colombo" } });
    expect(within(invitesFor()).getByRole("checkbox", { name: "Americas Weekend (IST)" })).toBeInTheDocument();
    expect(hours()).not.toHaveTextContent(/daylight saving/);
    expect(hours()).not.toHaveAttribute("aria-describedby");
  });

  it("saves a rotation's invitations turned off, sending every switch and keeping other saved codes", () => {
    settings.data = { ...LOADED, inviteRotations: { OTHER_CODE: false } };
    render(<RotaReminderSettings />);
    fireEvent.click(within(invitesFor()).getByRole("checkbox", { name: "Americas Weekend (PT)" }));
    fireEvent.click(saveButton());
    expect(lastBody().inviteRotations).toEqual({ ...ALL_INVITES_ON, CRE_WEEKEND_NIGHT: false, OTHER_CODE: false });
  });

  it("shows a rotation saved with its invitations off as off", () => {
    settings.data = { ...LOADED, inviteRotations: { CRE_MORNING: false } };
    render(<RotaReminderSettings />);
    expect(within(invitesFor()).getByRole("checkbox", { name: "Morning" })).not.toBeChecked();
    expect(within(invitesFor()).getByRole("checkbox", { name: "Evening" })).toBeChecked();
  });

  it("disables the per-rotation switches while calendar invitations are off, and keeps their choice", () => {
    settings.data = { ...LOADED, inviteRotations: { CRE_EVENING: false } };
    render(<RotaReminderSettings />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Attach calendar invitations" }));
    expect(invitesFor()).toBeDisabled();
    expect(within(invitesFor()).getByRole("checkbox", { name: "Morning" })).not.toBeChecked();
    fireEvent.click(saveButton());
    expect(lastBody()).toMatchObject({
      invitesEnabled: false,
      inviteRotations: { ...ALL_INVITES_ON, CRE_EVENING: false },
    });
  });

  it("turning the weekday reminders on is a first save that sends weekday on and weekend off", () => {
    render(<RotaReminderSettings />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Send weekday reminders" }));
    fireEvent.click(saveButton());
    expect(lastBody()).toMatchObject({ weekdayEnabled: true, weekendEnabled: false });
  });

  it("a first save sends only the minimum that was changed", () => {
    render(<RotaReminderSettings />);
    fireEvent.change(lkField(), { target: { value: "4" } });
    fireEvent.click(saveButton());
    expect(lastBody()).toMatchObject({ weekendRequired: { CRE_WEEKEND: 4 }, windows: {} });
  });

  it("refuses an address that is not an email, without saving", () => {
    settings.data = { ...LOADED, copyTo: ["ana@example.test", "not-an-email"] };
    render(<RotaReminderSettings />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Send weekend reminders" }));
    fireEvent.click(saveButton());
    expect(screen.getByRole("alert")).toHaveTextContent("not-an-email");
    expect(mutate).not.toHaveBeenCalled();
  });

  it("refuses a window that starts when it ends, without saving", () => {
    render(<RotaReminderSettings />);
    fireEvent.change(endField(), { target: { value: "08:00" } });
    fireEvent.click(saveButton());
    expect(screen.getByRole("alert")).toHaveTextContent("must differ");
    expect(mutate).not.toHaveBeenCalled();
  });

  it("refuses a head count that is not a whole number from 0 to 20, without saving", () => {
    render(<RotaReminderSettings />);
    fireEvent.change(lkField(), { target: { value: "21" } });
    fireEvent.click(saveButton());
    expect(screen.getByRole("alert")).toHaveTextContent("People needed on LK Weekend each day must be a whole number from 0 to 20.");
    expect(mutate).not.toHaveBeenCalled();

    fireEvent.change(lkField(), { target: { value: "2.5" } });
    fireEvent.click(saveButton());
    expect(screen.getByRole("alert")).toHaveTextContent("whole number");
    expect(mutate).not.toHaveBeenCalled();
  });

  it("says why the server did not save", () => {
    mutate.mockImplementation((_body: SaveRotaReminderSettings, opts?: Callbacks) =>
      opts?.onError?.(new Error("only a CSM admin or the CRE rota admin can change the CRE rota reminder settings")),
    );
    render(<RotaReminderSettings />);
    fireEvent.change(lkField(), { target: { value: "4" } });
    fireEvent.click(saveButton());
    expect(screen.getByRole("alert")).toHaveTextContent(
      "only a CSM admin or the CRE rota admin can change the CRE rota reminder settings",
    );
    expect(screen.queryByText("Saved.")).not.toBeInTheDocument();
  });

  it("confirms a save, shows what was saved and has nothing left to save", () => {
    settings.data = LOADED;
    mutate.mockImplementation((body: SaveRotaReminderSettings, opts?: Callbacks) =>
      opts?.onSuccess?.({ ...LOADED, ...body, updatedBy: "me@example.test", updatedOn: "2026-10-02T09:30:00Z" }),
    );
    render(<RotaReminderSettings />);
    fireEvent.change(lkField(), { target: { value: "4" } });
    fireEvent.click(saveButton());

    expect(screen.getByText("Saved.")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(lkField()).toHaveValue(4);
    expect(screen.getByText(/Last changed by me@example\.test on/)).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
    expect(discardButton()).toBeDisabled();
  });

  it("puts changes back with Discard", () => {
    settings.data = LOADED;
    render(<RotaReminderSettings />);
    pick("Cc on reminder emails", "ana", /Ana Lead/);
    expect(within(screen.getByRole("list", { name: "Copied" })).getByText("Ana Lead")).toBeInTheDocument();
    expect(discardButton()).toBeEnabled();
    fireEvent.click(discardButton());
    expect(screen.queryByRole("list", { name: "Copied" })).not.toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
    expect(discardButton()).toBeDisabled();
  });

  it("is not dirty once a change is put back by hand", () => {
    render(<RotaReminderSettings />);
    fireEvent.change(lkField(), { target: { value: "5" } });
    expect(saveButton()).toBeEnabled();
    fireEvent.change(lkField(), { target: { value: "3" } });
    expect(saveButton()).toBeDisabled();
  });

  it("says Saving… and holds Save while a save is under way", () => {
    saving.pending = true;
    render(<RotaReminderSettings />);
    fireEvent.change(lkField(), { target: { value: "4" } });
    expect(saveButton()).toHaveTextContent("Saving…");
    expect(saveButton()).toBeDisabled();
  });

  it("says it is loading, and when it could not load", () => {
    settings.data = undefined;
    settings.isLoading = true;
    const { unmount } = render(<RotaReminderSettings />);
    expect(screen.getByText("Loading reminder settings…")).toBeInTheDocument();
    unmount();

    settings.isLoading = false;
    settings.isError = true;
    render(<RotaReminderSettings />);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load the reminder settings.");
  });
});
