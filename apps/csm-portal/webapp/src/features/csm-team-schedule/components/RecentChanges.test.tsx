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
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ScheduleActivity } from "../types";
import RecentChanges from "./RecentChanges";

const change = (id: string, userId: string, createdOn: string): ScheduleActivity => ({
  id,
  assignmentId: `a-${id}`,
  userId,
  teamKey: "atlas",
  rotaDate: "2026-11-02",
  subject: "rota",
  shiftCode: "CRE_MORNING",
  action: "CREATED",
  actorEmail: "lead@example.com",
  createdOn,
});

const names: Record<string, string> = { u1: "Jane Doe", u2: "John Roe" };

const renderPanel = (activity: ScheduleActivity[]) =>
  render(
    <RecentChanges
      activity={activity}
      isLoading={false}
      nameOf={(id) => names[id]}
      shiftLabel={() => "Morning"}
      kindLabel={(c) => c}
      onClose={() => {}}
    />,
  );

describe("RecentChanges", () => {
  it("shows each change's exact time in IST", () => {
    // 03:57 UTC is 09:27 IST.
    renderPanel([change("1", "u1", "2026-11-03T03:57:00Z")]);
    const time = screen.getByText("3 Nov, 09:27 IST");
    expect(time.tagName).toBe("TIME");
    expect(time).toHaveAttribute("dateTime", "2026-11-03T03:57:00.000Z");
  });

  it("filters the list to one person, and back to everyone", () => {
    renderPanel([
      change("1", "u1", "2026-11-03T04:00:00Z"),
      change("2", "u2", "2026-11-03T03:00:00Z"),
      change("3", "u1", "2026-11-02T10:00:00Z"),
    ]);
    const list = (): HTMLElement => screen.getByRole("list");
    expect(within(list()).getAllByRole("listitem")).toHaveLength(3);

    const select = screen.getByRole("combobox", { name: "Person" });
    expect(within(select).getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Everyone",
      "Jane Doe",
      "John Roe",
    ]);

    fireEvent.change(select, { target: { value: "u1" } });
    const items = within(list()).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    for (const item of items) expect(item).toHaveTextContent("Jane Doe");
    expect(screen.getByText("2", { selector: ".count" })).toBeInTheDocument();

    fireEvent.change(select, { target: { value: "" } });
    expect(within(list()).getAllByRole("listitem")).toHaveLength(3);
  });

  it("offers no person filter when every change is about one person", () => {
    renderPanel([change("1", "u1", "2026-11-03T04:00:00Z"), change("2", "u1", "2026-11-03T03:00:00Z")]);
    expect(screen.queryByRole("combobox", { name: "Person" })).not.toBeInTheDocument();
  });

  it("lists only people it can name, never a row of \"someone\"", () => {
    renderPanel([
      change("1", "u1", "2026-11-03T04:00:00Z"),
      change("2", "u2", "2026-11-03T03:00:00Z"),
      change("3", "u9", "2026-11-03T02:00:00Z"),
      change("4", "u8", "2026-11-03T01:00:00Z"),
    ]);
    const select = screen.getByRole("combobox", { name: "Person" });
    expect(within(select).getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Everyone",
      "Jane Doe",
      "John Roe",
    ]);
  });
});
