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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeCallRequestView } from "@api/backend/types";

// The widget's data hooks are the seam: what the list shows, and what a change sends.
const hooks = vi.hoisted(() => ({
  requests: [] as BeCallRequestView[],
  listArgs: vi.fn(),
  patch: vi.fn(),
  patchPending: false,
}));

vi.mock("@features/csm-cases/api/useCsmCaseCallRequests", () => ({
  useGetCsmCaseCallRequests: (caseId: string, states?: string[]) => {
    hooks.listArgs(caseId, states);
    return {
    data: hooks.requests,
    isLoading: false,
    isError: false,
    isFetching: false,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
    };
  },
  usePostCsmCaseCallRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  usePatchCsmCaseCallRequest: () => ({ mutateAsync: hooks.patch, isPending: hooks.patchPending }),
}));

import { CallRequestsWidget } from "@features/csm-cases/components/CallRequestsWidget";

const call = (id: string, stateId: number, label: string): BeCallRequestView => ({
  id,
  reason: `reason ${id}`,
  preferredTimes: ["2026-10-08T10:00:00Z"],
  durationMin: 30,
  state: { id: stateId, label },
});

const complete = (): HTMLElement => screen.getByRole("button", { name: /mark as completed/i });
const queryComplete = (): HTMLElement | null =>
  screen.queryByRole("button", { name: /mark as completed/i });

function renderWidget(props: { isClosed?: boolean; readOnly?: boolean } = {}) {
  return render(<CallRequestsWidget caseId="case-1" caseState="work_in_progress" {...props} />);
}

describe("CallRequestsWidget: Mark as completed", () => {
  beforeEach(() => {
    hooks.patch.mockReset().mockResolvedValue({});
    hooks.patchPending = false;
    hooks.requests = [call("cr-1", 3, "Scheduled")];
  });

  it("concludes the call in one click: state only, no notes, no dialog", async () => {
    renderWidget();
    fireEvent.click(complete());

    await waitFor(() => expect(hooks.patch).toHaveBeenCalledTimes(1));
    expect(hooks.patch).toHaveBeenCalledWith({
      caseId: "case-1",
      callRequestId: "cr-1",
      patch: { state: "concluded" },
    });
    expect(Object.keys(hooks.patch.mock.calls[0][0].patch)).toEqual(["state"]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("is offered on scheduled and notes-pending calls only", () => {
    hooks.requests = [
      call("a", 2, "Pending on WSO2"),
      call("b", 3, "Scheduled"),
      call("c", 7, "Notes Pending"),
      call("d", 6, "Canceled"),
      call("e", 8, "Concluded"),
      call("f", 5, "WSO2 Rejected"),
    ];
    renderWidget();
    expect(screen.getAllByRole("button", { name: /mark as completed/i })).toHaveLength(2);
    // The existing notes action is still there for the notes-pending call.
    expect(screen.getAllByRole("button", { name: /send call notes/i })).toHaveLength(1);
  });

  it("sends one request for a fast double click", async () => {
    let finish: (v: unknown) => void = () => {};
    hooks.patch.mockImplementation(() => new Promise((resolve) => (finish = resolve)));
    renderWidget();

    fireEvent.click(complete());
    fireEvent.click(complete());
    fireEvent.click(complete());
    expect(hooks.patch).toHaveBeenCalledTimes(1);

    finish({});
    await waitFor(() => expect(hooks.patch).toHaveBeenCalledTimes(1));
  });

  it("shows the backend's reason when the call has changed meanwhile, and lets the engineer retry", async () => {
    const reason =
      "a call request can only be marked completed while it is scheduled or notes pending (this one is currently Canceled)";
    hooks.patch.mockRejectedValueOnce(new Error(reason)).mockResolvedValueOnce({});
    renderWidget();

    fireEvent.click(complete());
    expect(await screen.findByText(reason)).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    // Not stuck: the button works again, and a new attempt clears the old message.
    fireEvent.click(complete());
    await waitFor(() => expect(hooks.patch).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.queryByText(reason)).not.toBeInTheDocument());
  });

  it("falls back to a generic message when the error carries none", async () => {
    hooks.patch.mockRejectedValueOnce(new Error(""));
    renderWidget();
    fireEvent.click(complete());
    expect(await screen.findByText(/could not mark the call request as completed/i)).toBeInTheDocument();
  });

  it("does nothing on a closed case or without write access", () => {
    for (const props of [{ isClosed: true }, { readOnly: true }]) {
      const { unmount } = renderWidget(props);
      expect(complete()).toBeDisabled();
      fireEvent.click(complete());
      expect(hooks.patch).not.toHaveBeenCalled();
      unmount();
    }
  });

  it("is disabled while another change to a call request is in flight", () => {
    hooks.patchPending = true;
    renderWidget();
    expect(complete()).toBeDisabled();
    expect(queryComplete()).toBeInTheDocument();
  });
});

describe("CallRequestsWidget: state filter", () => {
  beforeEach(() => {
    hooks.listArgs.mockReset();
    hooks.patch.mockReset().mockResolvedValue({});
    hooks.patchPending = false;
    hooks.requests = [call("cr-1", 3, "Scheduled")];
  });

  const lastStates = (): unknown => hooks.listArgs.mock.calls.at(-1)?.[1];
  const pick = (name: RegExp): void => {
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name }));
  };

  it("asks only for the calls that can still move on, so finished ones leave the list", () => {
    renderWidget();
    expect(lastStates()).toEqual(["pending_on_customer", "pending_on_wso2", "scheduled", "notes_pending"]);
    expect(screen.getByText("1 open")).toBeInTheDocument();
  });

  it("brings every call back on All states", () => {
    renderWidget();
    pick(/all states/i);
    expect(lastStates()).toBeUndefined();
    expect(screen.getByText("1 total")).toBeInTheDocument();
  });

  it("filters to a single state, finished ones included", () => {
    renderWidget();
    pick(/^concluded$/i);
    expect(lastStates()).toEqual(["concluded"]);
    expect(screen.getByText("1 matching")).toBeInTheDocument();
  });

  it("explains where finished calls went when nothing is open", () => {
    hooks.requests = [];
    renderWidget();
    expect(screen.getByText(/no open call requests/i)).toBeInTheDocument();
    expect(screen.getByText(/all states/i)).toBeInTheDocument();
  });
});
