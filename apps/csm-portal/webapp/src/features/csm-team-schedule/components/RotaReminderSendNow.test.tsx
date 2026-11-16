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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { RotaReminderRequest, ScheduleAssignment, ScheduleAssignmentsResponse } from "../types";
// vi.mock below is hoisted above this import, so the section gets the mocks.
import RotaReminderSendNow from "./RotaReminderSendNow";

const requests: { data: RotaReminderRequest[] | undefined } = { data: [] };
const roster: { data: ScheduleAssignmentsResponse | undefined; isLoading: boolean; isError: boolean } = {
  data: undefined,
  isLoading: false,
  isError: false,
};
const sending = { pending: false };
const mutate = vi.fn();
const previewArgs = vi.fn();

vi.mock("../api/useRotaReminderRequests", () => ({
  useRotaReminderRequests: () => ({ data: requests.data }),
  useSendRotaReminder: () => ({ mutate, isPending: sending.pending }),
  useReminderPreview: (from: string, to: string, enabled: boolean) => {
    previewArgs(from, to, enabled);
    return { data: roster.data, isLoading: roster.isLoading, isError: roster.isError };
  },
}));

type Callbacks = { onSuccess?: (r: RotaReminderRequest) => void; onError?: (e: Error) => void };

// Wednesday 21 October 2026, 13:30 in Colombo. The next working day is Thursday
// 22 October and the weekend is Saturday 24 and Sunday 25 October.
const NOW = new Date("2026-10-21T08:00:00Z");

function assignment(name: string, shiftCode: string, rotaDate: string): ScheduleAssignment {
  const email = `${name.toLowerCase()}@example.test`;
  return {
    id: `${name}-${shiftCode}-${rotaDate}`,
    engineer: { userId: `u-${name}`, name, email, isLead: false },
    teamKey: "cre",
    shiftCode,
    rotaDate,
    startsAt: `${rotaDate}T00:00:00Z`,
    endsAt: `${rotaDate}T08:00:00Z`,
    isOnCall: false,
    source: "manual",
  };
}

const SAT = "2026-10-24";
const SUN = "2026-10-25";
const THU = "2026-10-22";

const ROSTERED: ScheduleAssignment[] = [
  assignment("Ann", "CRE_WEEKEND", SAT),
  assignment("Ben", "CRE_WEEKEND", SAT),
  assignment("Cid", "CRE_WEEKEND", SAT),
  assignment("Dee", "CRE_WEEKEND_NIGHT", SAT),
  assignment("Ann", "CRE_WEEKEND", SUN),
  assignment("Ben", "CRE_WEEKEND", SUN),
  assignment("Dee", "CRE_WEEKEND_NIGHT", SUN),
  assignment("Eve", "CRE_WEEKEND_NIGHT_OC", SUN),
  // Not a rotation the weekend reminder is about, so not counted.
  assignment("Zed", "CRE_MORNING", SAT),
  assignment("Fay", "CRE_MORNING", THU),
  assignment("Gus", "CRE_EVENING", THU),
  assignment("Hal", "CRE_EVENING", THU),
];

function setRoster(assignments: ScheduleAssignment[]): void {
  roster.data = { assignments, count: assignments.length };
}

function request(over: Partial<RotaReminderRequest>): RotaReminderRequest {
  return {
    id: "r1",
    family: "CRE",
    kind: "weekend",
    test: false,
    status: "QUEUED",
    requestedBy: "ann@example.com",
    requestedOn: "2026-10-21T05:00:00Z",
    ...over,
  };
}

const when = (iso: string): string => new Date(iso).toLocaleString();

const sendButton = () => screen.getByRole("button", { name: "Send now" });
const testButton = () => screen.getByRole("button", { name: "Send a test to me" });
const pickWeekend = (): void => {
  fireEvent.click(screen.getByRole("radio", { name: "Weekend" }));
};

/** The body rows of a table, each as the text of its cells. */
function bodyRows(table: HTMLElement): string[][] {
  return within(table)
    .getAllByRole("row")
    .slice(1)
    .map((row) => within(row).getAllByRole("cell").map((cell) => cell.textContent ?? ""));
}

beforeEach(() => {
  // Only the clock is fake, so nothing else waits on a timer.
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(NOW);
  mutate.mockReset();
  previewArgs.mockReset();
  sending.pending = false;
  requests.data = [];
  roster.isLoading = false;
  roster.isError = false;
  setRoster(ROSTERED);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("RotaReminderSendNow", () => {
  it("offers the Morning, the Evening and the weekend as one choice, each with its dates", () => {
    render(<RotaReminderSendNow />);
    expect(screen.getByRole("heading", { name: "Send now", level: 4 })).toBeInTheDocument();
    expect(
      screen.getByText(
        "Sends the reminders and calendar invitations now, whatever the on/off switches in Settings say. They go out at the scheduled task's next run, within the hour.",
      ),
    ).toBeInTheDocument();

    expect(screen.getByRole("group", { name: "What to send" })).toBeInTheDocument();
    const morning = screen.getByRole("radio", { name: "Morning" });
    const evening = screen.getByRole("radio", { name: "Evening" });
    const weekend = screen.getByRole("radio", { name: "Weekend" });
    expect(morning).toBeChecked();
    expect(evening).not.toBeChecked();
    expect(weekend).not.toBeChecked();
    expect(morning).toHaveAccessibleDescription("Thursday, 22 October");
    expect(screen.queryByRole("radio", { name: "Next working day" })).not.toBeInTheDocument();
    // Said once under the choice (the table lists each day on its own rows).
    expect(screen.getByText("Thursday, 22 October", { selector: "p" })).toBeInTheDocument();

    pickWeekend();
    expect(weekend).toBeChecked();
    expect(morning).not.toBeChecked();
    expect(weekend).toHaveAccessibleDescription("Saturday, 24 and Sunday, 25 October");
    expect(screen.getByText("Saturday, 24 and Sunday, 25 October", { selector: "p" })).toBeInTheDocument();
  });

  it("previews the next working day's Morning as a table of day, rotation and people", () => {
    render(<RotaReminderSendNow />);
    expect(previewArgs).toHaveBeenLastCalledWith(THU, THU, true);

    const table = screen.getByRole("table", { name: "Who gets it" });
    expect(within(table).getAllByRole("columnheader").map((h) => h.textContent)).toEqual([
      "Day",
      "Rotation",
      "People",
    ]);
    expect(bodyRows(table)).toEqual([
      ["Thursday, 22 October", "Morning", "1 · Fay"],
      ["Thursday, 22 October", "Morning on-call", "nobody"],
    ]);
    expect(screen.getByText("People on leave are left out.")).toBeInTheDocument();
    expect(
      screen.queryByText(/^Sent even if the weekend is not fully marked/),
    ).not.toBeInTheDocument();
  });

  it("previews the Evening on its own, for the same day", () => {
    render(<RotaReminderSendNow />);
    fireEvent.click(screen.getByRole("radio", { name: "Evening" }));
    expect(previewArgs).toHaveBeenLastCalledWith(THU, THU, true);
    expect(bodyRows(screen.getByRole("table", { name: "Who gets it" }))).toEqual([
      ["Thursday, 22 October", "Evening", "2 · Gus, Hal"],
    ]);
  });

  it("previews the weekend by day and rotation, and counts each person once", () => {
    render(<RotaReminderSendNow />);
    pickWeekend();
    expect(previewArgs).toHaveBeenLastCalledWith(SAT, SUN, true);

    expect(bodyRows(screen.getByRole("table", { name: "Who gets it" }))).toEqual([
      ["Saturday, 24 October", "LK Weekend", "3 · Ann, Ben, Cid"],
      ["Saturday, 24 October", "Americas Weekend", "1 · Dee"],
      ["Saturday, 24 October", "Americas On-call", "nobody"],
      ["Sunday, 25 October", "LK Weekend", "2 · Ann, Ben"],
      ["Sunday, 25 October", "Americas Weekend", "1 · Dee"],
      ["Sunday, 25 October", "Americas On-call", "1 · Eve"],
    ]);

    expect(
      screen.getByText(
        "Sent even if the weekend is not fully marked: what is missing is listed and the CRE rota admins are told. Sending again updates the same calendar events.",
      ),
    ).toBeInTheDocument();

    // Ann, Ben, Cid, Dee and Eve: five people, though some are listed twice.
    fireEvent.click(sendButton());
    expect(screen.getByText("Email 5 people now?")).toBeInTheDocument();
  });

  it("says nobody is rostered, and does not let anything be sent", () => {
    setRoster([]);
    render(<RotaReminderSendNow />);
    expect(screen.getByText("Nobody is on these rotations, so nothing would be sent.")).toBeInTheDocument();
    expect(sendButton()).toBeDisabled();
  });

  it("says it is loading the rota, and when it could not", () => {
    roster.data = undefined;
    roster.isLoading = true;
    const { unmount } = render(<RotaReminderSendNow />);
    expect(screen.getByText("Loading who is on the rota…")).toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "Who gets it" })).not.toBeInTheDocument();
    expect(sendButton()).toBeDisabled();
    unmount();

    roster.isLoading = false;
    roster.isError = true;
    render(<RotaReminderSendNow />);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load the rota for the preview.");
    expect(sendButton()).toBeDisabled();
  });

  it("asks twice: nothing is sent until Yes, send", () => {
    render(<RotaReminderSendNow />);
    pickWeekend();
    fireEvent.click(sendButton());
    expect(mutate).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Yes, send" })).toBeEnabled();
    expect(screen.queryByRole("button", { name: "Send now" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send a test to me" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Yes, send" }));
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate.mock.calls[0]?.[0]).toEqual({ kind: "weekend", test: false });
  });

  it("sends the Morning and the Evening apart, each to its own people", () => {
    render(<RotaReminderSendNow />);
    fireEvent.click(sendButton());
    expect(screen.getByText("Email 1 person now?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Yes, send" }));
    expect(mutate.mock.calls[0]?.[0]).toEqual({ kind: "morning", test: false });

    fireEvent.click(screen.getByRole("radio", { name: "Evening" }));
    fireEvent.click(sendButton());
    expect(screen.getByText("Email 2 people now?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Yes, send" }));
    expect(mutate.mock.calls[1]?.[0]).toEqual({ kind: "evening", test: false });
  });

  it("does not send on Cancel, and offers Send now again", () => {
    render(<RotaReminderSendNow />);
    pickWeekend();
    fireEvent.click(sendButton());
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(mutate).not.toHaveBeenCalled();
    expect(screen.queryByText(/^Email \d+ (people|person) now\?$/)).not.toBeInTheDocument();
    expect(sendButton()).toBeEnabled();
    expect(testButton()).toBeEnabled();
  });

  it("says it is queued once the ask is taken", () => {
    mutate.mockImplementation((_body: unknown, opts?: Callbacks) => opts?.onSuccess?.(request({})));
    render(<RotaReminderSendNow />);
    pickWeekend();
    fireEvent.click(sendButton());
    fireEvent.click(screen.getByRole("button", { name: "Yes, send" }));

    expect(screen.getByText("Queued. It goes out at the scheduled task's next run.")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByText(/^Email \d+ (people|person) now\?$/)).not.toBeInTheDocument();
  });

  it("shows the server's words when it refuses", () => {
    mutate.mockImplementation((_body: unknown, opts?: Callbacks) =>
      opts?.onError?.(new Error("a weekend reminder was asked for less than 10 minutes ago")),
    );
    render(<RotaReminderSendNow />);
    pickWeekend();
    fireEvent.click(sendButton());
    fireEvent.click(screen.getByRole("button", { name: "Yes, send" }));

    expect(screen.getByRole("alert")).toHaveTextContent("a weekend reminder was asked for less than 10 minutes ago");
    expect(screen.queryByText(/^Queued\./)).not.toBeInTheDocument();
    expect(sendButton()).toBeEnabled();
  });

  it("holds Send now, and says why, while a reminder of that kind is waiting", () => {
    requests.data = [request({ kind: "weekend", status: "QUEUED" })];
    render(<RotaReminderSendNow />);

    // The Morning reminder is not the one waiting.
    expect(sendButton()).toBeEnabled();
    expect(screen.queryByText(/is already waiting to be sent\.$/)).not.toBeInTheDocument();

    pickWeekend();
    expect(sendButton()).toBeDisabled();
    expect(screen.getByText("A weekend reminder is already waiting to be sent.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: "Morning" }));
    expect(sendButton()).toBeEnabled();
  });

  it("holds the Morning but not the Evening while a Morning reminder waits", () => {
    requests.data = [request({ kind: "morning", status: "QUEUED" })];
    render(<RotaReminderSendNow />);
    expect(sendButton()).toBeDisabled();
    expect(screen.getByText("A morning reminder is already waiting to be sent.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: "Evening" }));
    expect(sendButton()).toBeEnabled();
  });

  it("holds the Morning and the Evening while an older weekday request, which sends both, is being sent", () => {
    requests.data = [request({ kind: "weekday", status: "SENDING" })];
    render(<RotaReminderSendNow />);
    expect(sendButton()).toBeDisabled();
    expect(screen.getByText("A morning reminder is already waiting to be sent.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: "Evening" }));
    expect(sendButton()).toBeDisabled();
    pickWeekend();
    expect(sendButton()).toBeEnabled();
  });

  it("holds Send now while the ask is on its way", () => {
    sending.pending = true;
    render(<RotaReminderSendNow />);
    expect(sendButton()).toBeDisabled();
  });

  describe("Send a test to me", () => {
    it("asks for a test of the chosen kind at once, with no confirmation", () => {
      render(<RotaReminderSendNow />);
      pickWeekend();
      fireEvent.click(testButton());

      expect(mutate).toHaveBeenCalledTimes(1);
      expect(mutate.mock.calls[0]?.[0]).toEqual({ kind: "weekend", test: true });
      expect(screen.queryByRole("button", { name: "Yes, send" })).not.toBeInTheDocument();
      expect(screen.queryByText(/^Email \d+ (people|person) now\?$/)).not.toBeInTheDocument();
    });

    it("asks for the Morning or the Evening test as chosen", () => {
      render(<RotaReminderSendNow />);
      fireEvent.click(testButton());
      expect(mutate.mock.calls[0]?.[0]).toEqual({ kind: "morning", test: true });
      fireEvent.click(screen.getByRole("radio", { name: "Evening" }));
      fireEvent.click(testButton());
      expect(mutate.mock.calls[1]?.[0]).toEqual({ kind: "evening", test: true });
    });

    it("says the test is queued, and goes only to the person asking", () => {
      mutate.mockImplementation((_body: unknown, opts?: Callbacks) =>
        opts?.onSuccess?.(request({ test: true })),
      );
      render(<RotaReminderSendNow />);
      fireEvent.click(testButton());

      expect(
        screen.getByText("Test queued: it goes only to you, at the scheduled task's next run."),
      ).toBeInTheDocument();
      expect(screen.queryByText(/^Queued\./)).not.toBeInTheDocument();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("shows the server's words when it refuses the test", () => {
      mutate.mockImplementation((_body: unknown, opts?: Callbacks) =>
        opts?.onError?.(new Error("a test was asked for less than 2 minutes ago")),
      );
      render(<RotaReminderSendNow />);
      fireEvent.click(testButton());

      expect(screen.getByRole("alert")).toHaveTextContent("a test was asked for less than 2 minutes ago");
      expect(screen.queryByText(/^Test queued/)).not.toBeInTheDocument();
    });

    it("stays available while a real reminder of that kind is waiting", () => {
      requests.data = [request({ kind: "weekend", status: "QUEUED" })];
      render(<RotaReminderSendNow />);
      pickWeekend();
      expect(sendButton()).toBeDisabled();
      expect(testButton()).toBeEnabled();

      fireEvent.click(testButton());
      expect(mutate.mock.calls[0]?.[0]).toEqual({ kind: "weekend", test: true });
    });

    it("does not need anybody to be on the rota", () => {
      setRoster([]);
      render(<RotaReminderSendNow />);
      expect(sendButton()).toBeDisabled();
      expect(testButton()).toBeEnabled();
    });

    it("is held while an ask is on its way", () => {
      sending.pending = true;
      render(<RotaReminderSendNow />);
      expect(testButton()).toBeDisabled();
    });

    it("is not held back by a waiting test, and a waiting test does not hold Send now", () => {
      requests.data = [request({ kind: "weekday", test: true, status: "QUEUED" })];
      render(<RotaReminderSendNow />);
      expect(sendButton()).toBeEnabled();
      expect(screen.queryByText(/is already waiting to be sent\.$/)).not.toBeInTheDocument();
    });
  });

  it("lists recent asks in a table with where each one is", () => {
    requests.data = [
      request({ id: "a", kind: "weekend", status: "QUEUED", requestedBy: "ann@example.com" }),
      request({ id: "b", kind: "weekday", status: "SENDING", requestedBy: "ben@example.com" }),
      request({
        id: "c",
        kind: "weekend",
        status: "SENT",
        requestedBy: "cid@example.com",
        handledOn: "2026-10-20T10:00:00Z",
        result: "12 emails sent",
      }),
      request({ id: "d", kind: "weekday", status: "SENT", requestedBy: "dee@example.com", handledOn: "2026-10-19T10:00:00Z" }),
      request({ id: "e", kind: "weekday", status: "FAILED", requestedBy: "eve@example.com", result: "mail server down" }),
      request({ id: "f", kind: "morning", status: "QUEUED", requestedBy: "fay@example.com" }),
      request({ id: "g", kind: "evening", status: "QUEUED", requestedBy: "gus@example.com" }),
    ];
    render(<RotaReminderSendNow />);

    expect(screen.getByRole("heading", { name: "Recent" })).toBeInTheDocument();
    const table = screen.getByRole("table", { name: "Recent" });
    expect(within(table).getAllByRole("columnheader").map((h) => h.textContent)).toEqual([
      "When",
      "What",
      "By",
      "Status",
    ]);
    const asked = when("2026-10-21T05:00:00Z");
    expect(bodyRows(table)).toEqual([
      [asked, "Weekend", "ann@example.com", "Waiting to be sent"],
      [asked, "Weekday", "ben@example.com", "Sending…"],
      [asked, "Weekend", "cid@example.com", `Sent on ${when("2026-10-20T10:00:00Z")}: 12 emails sent`],
      [asked, "Weekday", "dee@example.com", `Sent on ${when("2026-10-19T10:00:00Z")}`],
      [asked, "Weekday", "eve@example.com", "Failed: mail server down"],
      [asked, "Morning", "fay@example.com", "Waiting to be sent"],
      [asked, "Evening", "gus@example.com", "Waiting to be sent"],
    ]);
  });

  it("marks a test request in the recent table", () => {
    requests.data = [
      request({ id: "t", kind: "weekend", test: true, status: "SENT", handledOn: "2026-10-21T06:00:00Z" }),
      request({ id: "r", kind: "weekday", test: false, status: "QUEUED" }),
    ];
    render(<RotaReminderSendNow />);

    const table = within(screen.getByRole("table", { name: "Recent" }));
    expect(table.getByRole("cell", { name: "Test · Weekend" })).toBeInTheDocument();
    expect(table.getByRole("cell", { name: "Weekday" })).toBeInTheDocument();
    expect(table.queryByRole("cell", { name: "Test · Weekday" })).not.toBeInTheDocument();
  });

  it("shows no Recent table when nothing was asked for", () => {
    render(<RotaReminderSendNow />);
    expect(screen.queryByRole("heading", { name: "Recent" })).not.toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "Recent" })).not.toBeInTheDocument();
  });
});
