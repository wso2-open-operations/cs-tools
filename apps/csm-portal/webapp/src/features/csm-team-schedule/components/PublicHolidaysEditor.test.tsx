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
import type { PublicHolidaysList } from "../types";
// vi.mock below is hoisted above this import, so the editor gets the mocks.
import PublicHolidaysEditor from "./PublicHolidaysEditor";

const list: { data: PublicHolidaysList | undefined; isError: boolean } = { data: undefined, isError: false };
const refetch = vi.fn();
const addMutate = vi.fn();
const removeMutate = vi.fn();

vi.mock("../api/usePublicHolidayEditor", () => ({
  usePublicHolidayList: () => ({ data: list.data, isLoading: !list.data && !list.isError, isError: list.isError, refetch }),
  useAddPublicHolidays: () => ({ mutate: addMutate, isPending: false }),
  useRemovePublicHoliday: () => ({ mutate: removeMutate, isPending: false }),
}));

type Callbacks = { onSuccess?: (r?: { stored: number }) => void; onError?: (e: Error) => void };

const editable = (holidays: PublicHolidaysList["holidays"] = []): PublicHolidaysList => ({
  holidays,
  countries: ["India", "Sri Lanka"],
  canEdit: true,
});

describe("PublicHolidaysEditor", () => {
  beforeEach(() => {
    list.data = undefined;
    list.isError = false;
    refetch.mockReset();
    addMutate.mockReset();
    removeMutate.mockReset();
  });

  it("is not shown to a reader who may not edit", () => {
    list.data = { holidays: [], countries: ["Sri Lanka"], canEdit: false };
    const { container } = render(<PublicHolidaysEditor />);
    expect(container).toBeEmptyDOMElement();
  });

  it("lists the chosen country's stored days, with where each came from", () => {
    list.data = editable([
      { country: "Sri Lanka", date: "2027-02-04", title: "Independence Day", source: "MANUAL", updatedBy: "admin@x.test" },
      { country: "India", date: "2027-01-26", title: "Republic Day", source: "CALENDAR" },
    ]);
    render(<PublicHolidaysEditor />);
    const table = screen.getByRole("table", { name: /Sri Lanka public holidays/ });
    expect(within(table).getByText("Independence Day")).toBeInTheDocument();
    expect(within(table).getByText("By hand (admin@x.test)")).toBeInTheDocument();
    expect(within(table).queryByText("Republic Day")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Country"), { target: { value: "India" } });
    const india = screen.getByRole("table", { name: /India public holidays/ });
    expect(within(india).getByText("Republic Day")).toBeInTheDocument();
    expect(within(india).getByText("From the calendar")).toBeInTheDocument();
  });

  it("adds the pasted days for the country and says how many", () => {
    list.data = editable();
    render(<PublicHolidaysEditor />);
    const add = screen.getByRole("button", { name: "Add holidays" });
    expect(add).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Holidays to add, one per line"), {
      target: { value: "2027-01-14, Tamil Thai Pongal Day\n2027-02-04\tIndependence Day" },
    });
    expect(screen.getByText(/2 holidays ready/)).toBeInTheDocument();
    fireEvent.click(add);
    expect(addMutate).toHaveBeenCalledWith(
      {
        country: "Sri Lanka",
        holidays: [
          { date: "2027-01-14", title: "Tamil Thai Pongal Day" },
          { date: "2027-02-04", title: "Independence Day" },
        ],
      },
      expect.anything(),
    );
    act(() => (addMutate.mock.calls[0][1] as Callbacks).onSuccess?.({ stored: 2 }));
    expect(screen.getByRole("status")).toHaveTextContent("Added 2 holidays for Sri Lanka.");
    expect(screen.getByLabelText("Holidays to add, one per line")).toHaveValue("");
  });

  it("holds back a paste with a bad line and names it", () => {
    list.data = editable();
    render(<PublicHolidaysEditor />);
    fireEvent.change(screen.getByLabelText("Holidays to add, one per line"), {
      target: { value: "2027-01-14, Thai Pongal\n14/02/2027, Valentine" },
    });
    expect(screen.getByRole("list", { name: "Lines to fix" })).toHaveTextContent("Line 2:");
    expect(screen.getByRole("button", { name: "Add holidays" })).toBeDisabled();
  });

  it("adds for a typed country and shows the server's refusal", () => {
    list.data = editable();
    render(<PublicHolidaysEditor />);
    fireEvent.change(screen.getByLabelText("Country"), { target: { value: "__other__" } });
    fireEvent.change(screen.getByLabelText("Country name"), { target: { value: " Australia " } });
    fireEvent.change(screen.getByLabelText("Holidays to add, one per line"), { target: { value: "2027-01-26, Australia Day" } });
    fireEvent.click(screen.getByRole("button", { name: "Add holidays" }));
    expect(addMutate.mock.calls[0][0]).toEqual({ country: "Australia", holidays: [{ date: "2027-01-26", title: "Australia Day" }] });
    act(() =>
      (addMutate.mock.calls[0][1] as Callbacks).onError?.(
        new Error("Only a portal admin or a rota admin can change the public holidays."),
      ),
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Only a portal admin or a rota admin");
  });

  it("is not shown while nothing says the reader may edit, even when the list fails", () => {
    list.isError = true;
    const { container } = render(<PublicHolidaysEditor />);
    expect(container).toBeEmptyDOMElement();
  });

  it("keeps the section, year picker included, while another year loads or fails", () => {
    list.data = editable();
    render(<PublicHolidaysEditor />);

    list.data = undefined;
    fireEvent.change(screen.getByLabelText("Year"), { target: { value: String(new Date().getFullYear() + 1) } });
    expect(screen.getByRole("status")).toHaveTextContent(/Loading the holidays/);
    expect(screen.getByLabelText("Year")).toBeInTheDocument();

    list.isError = true;
    fireEvent.change(screen.getByLabelText("Year"), { target: { value: String(new Date().getFullYear() + 2) } });
    expect(screen.getByRole("alert")).toHaveTextContent(/could not be loaded/);
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(refetch).toHaveBeenCalled();

    // A later answer that the reader may not edit hides it.
    list.isError = false;
    list.data = { holidays: [], countries: [], canEdit: false };
    fireEvent.change(screen.getByLabelText("Year"), { target: { value: String(new Date().getFullYear()) } });
    expect(screen.queryByRole("heading", { name: "Public holidays" })).not.toBeInTheDocument();
  });

  it("keeps a paste typed while the previous one was being added", () => {
    list.data = editable();
    render(<PublicHolidaysEditor />);
    const box = screen.getByLabelText("Holidays to add, one per line");
    fireEvent.change(box, { target: { value: "2027-01-14, Tamil Thai Pongal Day" } });
    fireEvent.click(screen.getByRole("button", { name: "Add holidays" }));
    fireEvent.change(box, { target: { value: "2027-02-04, Independence Day" } });
    act(() => (addMutate.mock.calls[0][1] as Callbacks).onSuccess?.({ stored: 1 }));
    expect(box).toHaveValue("2027-02-04, Independence Day");
  });

  it("imports a calendar file's days for the chosen year into the box, after what is there", async () => {
    list.data = editable();
    render(<PublicHolidaysEditor />);
    const y = new Date().getFullYear();
    const box = screen.getByLabelText("Holidays to add, one per line");
    fireEvent.change(box, { target: { value: `${y}-12-31, New Year's Eve` } });
    const ics = [
      "BEGIN:VCALENDAR",
      "BEGIN:VEVENT",
      `DTSTART;VALUE=DATE:${y}0115`,
      "SUMMARY:Tamil Thai Pongal Day",
      "END:VEVENT",
      "BEGIN:VEVENT",
      `DTSTART;VALUE=DATE:${y + 1}0115`,
      "SUMMARY:Next year",
      "END:VEVENT",
      "END:VCALENDAR",
    ].join("\r\n");
    fireEvent.change(screen.getByLabelText("Holiday file to import"), {
      target: { files: [new File([ics], "lk.ics", { type: "text/calendar" })] },
    });
    expect(await screen.findByRole("status")).toHaveTextContent(`1 holiday in ${y} read from the calendar. Check the lines`);
    expect(box).toHaveValue(`${y}-12-31, New Year's Eve\n${y}-01-15, Tamil Thai Pongal Day`);
    expect(screen.getByText(/2 holidays ready/)).toBeInTheDocument();
    expect(addMutate).not.toHaveBeenCalled();
  });

  it("says so when a file holds nothing to add, or is too large", async () => {
    list.data = editable();
    render(<PublicHolidaysEditor />);
    const input = screen.getByLabelText("Holiday file to import");
    fireEvent.change(input, { target: { files: [new File(["BEGIN:VCALENDAR\r\nEND:VCALENDAR"], "empty.ics")] } });
    expect(await screen.findByRole("alert")).toHaveTextContent(/No holidays found in empty.ics for \d{4}/);

    const big = new File(["x"], "big.txt");
    Object.defineProperty(big, "size", { value: 2 * 1024 * 1024 });
    fireEvent.change(input, { target: { files: [big] } });
    expect(await screen.findByRole("alert")).toHaveTextContent("big.txt is larger than 1 MB");
    expect(screen.getByLabelText("Holidays to add, one per line")).toHaveValue("");
  });

  it("removes one day", () => {
    list.data = editable([{ country: "Sri Lanka", date: "2027-02-04", title: "Independence Day", source: "MANUAL" }]);
    render(<PublicHolidaysEditor />);
    fireEvent.click(screen.getByRole("button", { name: "Remove Independence Day on 2027-02-04" }));
    expect(removeMutate).toHaveBeenCalledWith({ country: "Sri Lanka", date: "2027-02-04" }, expect.anything());
    act(() => (removeMutate.mock.calls[0][1] as Callbacks).onSuccess?.());
    expect(screen.getByRole("status")).toHaveTextContent("Removed 2027-02-04 for Sri Lanka.");
  });
});
