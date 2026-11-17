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

import { fireEvent, render, screen } from "@testing-library/react";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { describe, expect, it, vi } from "vitest";
import CallRequestCard from "../CallRequestCard";

describe("CallRequestCard", () => {
  it("offers approve, not reschedule, while the call is pending on the customer", () => {
    const onApproveClick = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <CallRequestCard
          call={{ id: "1", state: { label: "Pending on Customer" }, preferredTimes: [] } as never}
          onApproveClick={onApproveClick}
        />
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    expect(onApproveClick).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: /reschedule/i })).not.toBeInTheDocument();
  });

  it("shows Approve from the state id when the source label names the other party", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CallRequestCard
          call={
            {
              id: "1",
              state: { id: "pending_on_customer", label: "Pending on WSO2" },
              preferredTimes: [],
            } as never
          }
        />
      </ThemeProvider>,
    );
    expect(screen.getByText("Pending on Customer")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /approve/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /reschedule/i })).not.toBeInTheDocument();
  });

  it("shows a request pending on WSO2 as In Progress, keeping reschedule and cancel", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CallRequestCard
          call={
            {
              id: "1",
              state: { id: "pending_on_wso2", label: "Pending on WSO2" },
              preferredTimes: [],
            } as never
          }
        />
      </ThemeProvider>,
    );
    expect(screen.getByText("In Progress")).toBeInTheDocument();
    expect(screen.queryByText("Pending on WSO2")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /reschedule/i })).toBeEnabled();
    expect(screen.queryByRole("button", { name: /approve/i })).not.toBeInTheDocument();
  });

  it("shows In Progress from the label when the state has no id", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CallRequestCard
          call={{ id: "1", state: { label: "Pending On WSO2" }, preferredTimes: [] } as never}
        />
      </ThemeProvider>,
    );
    expect(screen.getByText("In Progress")).toBeInTheDocument();
  });

  it("keeps the label of a state whose id is not pending on WSO2", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CallRequestCard
          call={
            {
              id: "1",
              state: { id: "scheduled", label: "Scheduled" },
              preferredTimes: [],
            } as never
          }
        />
      </ThemeProvider>,
    );
    expect(screen.getByText("Scheduled")).toBeInTheDocument();
    expect(screen.queryByText("In Progress")).not.toBeInTheDocument();
  });

  it("shows preferred times and the scheduled time once an agent has accepted", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CallRequestCard
          call={
            {
              id: "1",
              state: { label: "Scheduled" },
              preferredTimes: ["2026-12-31T09:00:00Z"],
              scheduleTime: "2026-12-31T10:30:00Z",
              durationMin: 30,
            } as never
          }
          userTimeZone="UTC"
        />
      </ThemeProvider>,
    );

    expect(screen.getByText("Preferred Times")).toBeInTheDocument();
    expect(screen.getByText("Scheduled Time")).toBeInTheDocument();
    expect(screen.getByText(/9:00/)).toBeInTheDocument();
    expect(screen.getByText(/10:30/)).toBeInTheDocument();
  });

  it("hides scheduled time until an agent has set one", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CallRequestCard
          call={
            {
              id: "1",
              state: { label: "Pending on WSO2" },
              preferredTimes: ["2026-12-31T09:00:00Z"],
              scheduleTime: null,
            } as never
          }
          userTimeZone="UTC"
        />
      </ThemeProvider>,
    );

    expect(screen.getByText("Preferred Times")).toBeInTheDocument();
    expect(screen.queryByText("Scheduled Time")).not.toBeInTheDocument();
  });
});
