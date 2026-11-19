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

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const getMock = vi.fn();
const postMock = vi.fn();
const accessMock = vi.fn();

vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => accessMock(),
}));

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, post: postMock }),
}));

import { ProductIssueDetailDialog } from "@features/csm-cases/components/ProductIssueDetailDialog";

function cmd(over: Record<string, unknown> = {}) {
  return {
    id: "cmd-1",
    kind: "SET_BOARD_STATUS",
    status: "PENDING",
    attempts: 0,
    requestedBy: "u",
    createdOn: "2026-01-02T00:00:00Z",
    ...over,
  };
}

function detail(over: Record<string, unknown> = {}) {
  return {
    id: "wi-1",
    number: "PI0000001",
    title: "Login fails",
    github: { owner: "wso2", repository: "product-is", issueNumber: 12, state: "OPEN" },
    boardStatus: "WAITING_ON_CS",
    linkState: "ACTIVE",
    conversation: [],
    statusHistory: [],
    commands: [],
    ...over,
  };
}

function renderDialog(queryClient?: QueryClient) {
  const qc = queryClient ?? new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return {
    qc,
    ...render(
      <QueryClientProvider client={qc}>
        <ProductIssueDetailDialog workItemId="wi-1" onClose={() => {}} />
      </QueryClientProvider>,
    ),
  };
}

async function loaded() {
  await screen.findByText("Login fails");
}

function keyOf(call: unknown[]): string | undefined {
  return (call[2] as { headers?: Record<string, string> } | undefined)?.headers?.["Idempotency-Key"];
}

describe("product issue write-back", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
    accessMock.mockReset();
    accessMock.mockReturnValue({ canWrite: true });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  describe("button visibility", () => {
    it("shows both buttons for an open WOC issue with an active link", async () => {
      getMock.mockResolvedValue(detail());
      renderDialog();
      await loaded();
      expect(screen.getByRole("button", { name: "Back to product team" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Close issue" })).toBeInTheDocument();
    });

    it("hides Back to product team unless the board status is WAITING_ON_CS", async () => {
      getMock.mockResolvedValue(detail({ boardStatus: "IN_PROGRESS" }));
      renderDialog();
      await loaded();
      expect(screen.queryByRole("button", { name: "Back to product team" })).toBeNull();
      expect(screen.getByRole("button", { name: "Close issue" })).toBeInTheDocument();
    });

    it("hides both buttons and the comment box when the link is not active", async () => {
      getMock.mockResolvedValue(detail({ linkState: "INACCESSIBLE" }));
      renderDialog();
      await loaded();
      expect(screen.queryByRole("button", { name: "Back to product team" })).toBeNull();
      expect(screen.queryByRole("button", { name: "Close issue" })).toBeNull();
      expect(screen.queryByLabelText("Comment")).toBeNull();
    });

    it("hides Close issue when the GitHub issue is already closed", async () => {
      getMock.mockResolvedValue(
        detail({ github: { owner: "wso2", repository: "product-is", issueNumber: 12, state: "CLOSED" } }),
      );
      renderDialog();
      await loaded();
      expect(screen.queryByRole("button", { name: "Close issue" })).toBeNull();
    });

    it("replaces Back to product team with 'Move to WOW pending' while a move is pending", async () => {
      getMock.mockResolvedValue(detail({ commands: [cmd()] }));
      renderDialog();
      await loaded();
      expect(screen.getByText("Move to WOW pending")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Back to product team" })).toBeNull();
      expect(screen.getByRole("button", { name: "Close issue" })).toBeInTheDocument();
    });

    it("replaces Close issue with 'Close pending' while a close is pending", async () => {
      getMock.mockResolvedValue(detail({ commands: [cmd({ id: "c2", kind: "SET_STATE" })] }));
      renderDialog();
      await loaded();
      expect(screen.getByText("Close pending")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Close issue" })).toBeNull();
      expect(screen.getByRole("button", { name: "Back to product team" })).toBeInTheDocument();
    });
  });

  describe("permission", () => {
    it("hides the comment box, both buttons and every Retry without write access", async () => {
      accessMock.mockReturnValue({ canWrite: false });
      getMock.mockResolvedValue(
        detail({
          conversation: [
            { id: "cm-1", author: "Me", body: "b", createdOn: "2026-01-02T00:00:00Z", delivery: "FAILED", commandId: "cmd-c" },
          ],
          commands: [
            cmd({ id: "cmd-c", kind: "ADD_COMMENT", status: "FAILED", lastError: "e1" }),
            cmd({ id: "c4", status: "FAILED", lastError: "e2" }),
          ],
        }),
      );
      renderDialog();
      await loaded();
      expect(screen.getByText("Failed")).toBeInTheDocument();
      expect(screen.queryByLabelText("Comment")).toBeNull();
      expect(screen.queryByRole("button", { name: "Back to product team" })).toBeNull();
      expect(screen.queryByRole("button", { name: "Close issue" })).toBeNull();
      expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    });
  });

  describe("awaiting GitHub after delivery", () => {
    it("hides the move button and shows a waiting note for 2 minutes after delivery, then restores it", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const deliveredOn = new Date(Date.now() - 60_000).toISOString();
      getMock.mockResolvedValue(detail({ commands: [cmd({ status: "DELIVERED", deliveredOn })] }));
      renderDialog();
      await loaded();
      expect(screen.getByText("Waiting for GitHub to confirm")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Back to product team" })).toBeNull();
      const n = getMock.mock.calls.length;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5100);
      });
      expect(getMock.mock.calls.length).toBeGreaterThan(n);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(61_000);
      });
      expect(screen.queryByText("Waiting for GitHub to confirm")).toBeNull();
      expect(screen.getByRole("button", { name: "Back to product team" })).toBeInTheDocument();
      const settled = getMock.mock.calls.length;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(20_000);
      });
      expect(getMock.mock.calls.length).toBe(settled);
    });

    it("does not wait for a delivered command older than the window", async () => {
      const deliveredOn = new Date(Date.now() - 10 * 60_000).toISOString();
      getMock.mockResolvedValue(detail({ commands: [cmd({ status: "DELIVERED", deliveredOn })] }));
      renderDialog();
      await loaded();
      expect(screen.queryByText("Waiting for GitHub to confirm")).toBeNull();
      expect(screen.getByRole("button", { name: "Back to product team" })).toBeInTheDocument();
    });

    it("hides Close issue while a delivered close awaits confirmation", async () => {
      const deliveredOn = new Date(Date.now() - 5_000).toISOString();
      getMock.mockResolvedValue(
        detail({ commands: [cmd({ id: "c5", kind: "SET_STATE", status: "DELIVERED", deliveredOn })] }),
      );
      renderDialog();
      await loaded();
      expect(screen.getByText("Waiting for GitHub to confirm")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Close issue" })).toBeNull();
    });
  });

  describe("action errors", () => {
    it("shows the comment-row Retry error", async () => {
      getMock.mockResolvedValue(
        detail({
          conversation: [
            { id: "cm-1", author: "Me", body: "b", createdOn: "2026-01-02T00:00:00Z", delivery: "FAILED", commandId: "cmd-c" },
          ],
          commands: [cmd({ id: "cmd-c", kind: "ADD_COMMENT", status: "FAILED", lastError: "x" })],
        }),
      );
      postMock.mockRejectedValue(new Error("Retry refused."));
      renderDialog();
      await loaded();
      fireEvent.click(screen.getByRole("button", { name: "Retry" }));
      expect(await screen.findByRole("alert")).toHaveTextContent("Retry refused.");
    });

    it("keeps one error slot: a new action replaces the previous error", async () => {
      getMock.mockResolvedValue(detail());
      postMock
        .mockRejectedValueOnce(new Error("Move refused."))
        .mockRejectedValueOnce(new Error("Comment refused."));
      renderDialog();
      await loaded();
      fireEvent.click(screen.getByRole("button", { name: "Back to product team" }));
      expect(await screen.findByText("Move refused.")).toBeInTheDocument();
      fireEvent.change(screen.getByLabelText("Comment"), { target: { value: "hi" } });
      fireEvent.click(screen.getByRole("button", { name: "Send" }));
      expect(await screen.findByText("Comment refused.")).toBeInTheDocument();
      expect(screen.queryByText("Move refused.")).toBeNull();
    });
  });

  describe("idempotency keys for move and close", () => {
    const answered = (message: string) => Object.assign(new Error(message), { status: 409 });

    it("drops the key after a server answer and keeps it after a network failure", async () => {
      getMock.mockResolvedValue(detail());
      postMock
        .mockRejectedValueOnce(new Error("Failed to fetch"))
        .mockRejectedValueOnce(answered("nope"))
        .mockRejectedValueOnce(answered("nope again"));
      renderDialog();
      await loaded();
      const click = async (n: number) => {
        fireEvent.click(screen.getByRole("button", { name: "Back to product team" }));
        await waitFor(() => expect(postMock).toHaveBeenCalledTimes(n));
        await waitFor(() =>
          expect(screen.getByRole("button", { name: "Back to product team" })).not.toBeDisabled(),
        );
      };
      await click(1);
      await click(2);
      await click(3);
      const [k1, k2, k3] = postMock.mock.calls.map(keyOf);
      expect(k2).toBe(k1);
      expect(k3).not.toBe(k2);
    });
  });

  describe("comment box and confirm dialog", () => {
    it("disables the comment field while sending", async () => {
      getMock.mockResolvedValue(detail());
      postMock.mockReturnValue(new Promise(() => {}));
      renderDialog();
      await loaded();
      fireEvent.change(screen.getByLabelText("Comment"), { target: { value: "x" } });
      fireEvent.click(screen.getByRole("button", { name: "Send" }));
      await waitFor(() => expect(screen.getByLabelText("Comment")).toBeDisabled());
    });

    it("labels the confirm dialog, focuses Cancel, and closes it when Close stops being available", async () => {
      getMock.mockResolvedValue(detail());
      const { qc } = renderDialog();
      await loaded();
      fireEvent.click(screen.getByRole("button", { name: "Close issue" }));
      const dlg = await screen.findByRole("dialog", { name: "Close issue" });
      expect(within(dlg).getByRole("button", { name: "Cancel" })).toHaveFocus();
      await act(async () => {
        qc.setQueryData(["csm-product-work-item", "wi-1"], detail({ commands: [cmd({ id: "c6", kind: "SET_STATE" })] }));
      });
      await waitFor(() => expect(screen.queryByText(/on GitHub\?/)).toBeNull());
    });
  });

  describe("cancelled comments", () => {
    const comment = (over: Record<string, unknown> = {}) => ({
      id: "cm-1",
      author: "Me",
      body: "never sent",
      createdOn: "2026-01-02T00:00:00Z",
      origin: "PORTAL",
      commandId: "cmd-c",
      ...over,
    });
    const cancelled = cmd({
      id: "cmd-c",
      kind: "ADD_COMMENT",
      status: "CANCELLED",
      lastError: "the GitHub issue is no longer linked",
    });

    it("shows Not sent with the reason and no Retry", async () => {
      getMock.mockResolvedValue(
        detail({ conversation: [comment({ delivery: "CANCELLED" })], commands: [cancelled] }),
      );
      renderDialog();
      await loaded();
      expect(screen.getByText("Not sent")).toBeInTheDocument();
      expect(screen.getByText("the GitHub issue is no longer linked")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
      expect(screen.queryByText("Failed")).toBeNull();
    });

    it("derives Not sent from the command when delivery is absent", async () => {
      getMock.mockResolvedValue(
        detail({ conversation: [comment({ delivery: null })], commands: [cancelled] }),
      );
      renderDialog();
      await loaded();
      expect(screen.getByText("Not sent")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    });

    it("shows Not sent from the comment alone when its command is not listed", async () => {
      getMock.mockResolvedValue(detail({ conversation: [comment({ delivery: "CANCELLED" })] }));
      renderDialog();
      await loaded();
      expect(screen.getByText("Not sent")).toBeInTheDocument();
    });
  });

  describe("failed commands", () => {
    it("shows the error and retries the failed move", async () => {
      getMock.mockResolvedValue(
        detail({ commands: [cmd({ status: "FAILED", lastError: "GitHub said no" })] }),
      );
      postMock.mockResolvedValue({ command: cmd() });
      renderDialog();
      await loaded();
      expect(screen.getByText(/Move to WOW failed: GitHub said no/)).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Retry" }));
      await waitFor(() =>
        expect(postMock).toHaveBeenCalledWith(
          "/product-work-items/wi-1/commands/cmd-1/retry",
          {},
        ),
      );
    });

    it("shows a failed close with its error", async () => {
      getMock.mockResolvedValue(
        detail({ commands: [cmd({ id: "c3", kind: "SET_STATE", status: "FAILED", lastError: "boom" })] }),
      );
      renderDialog();
      await loaded();
      expect(screen.getByText(/Close failed: boom/)).toBeInTheDocument();
    });
  });

  describe("comments", () => {
    it("shows a sent comment at once with a Pending badge and clears the box", async () => {
      getMock.mockResolvedValue(detail());
      const comment = {
        id: "cm-1",
        author: "Me",
        body: "hello team",
        createdOn: "2026-01-02T00:00:00Z",
        origin: "PORTAL",
        delivery: "PENDING",
        commandId: "cmd-c",
      };
      const command = cmd({ id: "cmd-c", kind: "ADD_COMMENT", commentId: "cm-1" });
      postMock.mockResolvedValue({ comment, command });
      // The refetch after the action returns what the server now holds.
      getMock.mockResolvedValueOnce(detail()).mockResolvedValue(
        detail({ conversation: [comment], commands: [command] }),
      );
      renderDialog();
      await loaded();
      fireEvent.change(screen.getByLabelText("Comment"), { target: { value: "hello team" } });
      fireEvent.click(screen.getByRole("button", { name: "Send" }));
      expect(await screen.findByText("hello team", { selector: "p" })).toBeInTheDocument();
      expect(screen.getByText("Pending")).toBeInTheDocument();
      expect(postMock).toHaveBeenCalledWith(
        "/product-work-items/wi-1/comments",
        { body: "hello team" },
        expect.objectContaining({ headers: { "Idempotency-Key": expect.any(String) } }),
      );
      await waitFor(() => expect(screen.getByLabelText("Comment")).toHaveValue(""));
    });

    it("shows a failed comment with its error and retries its command", async () => {
      getMock.mockResolvedValue(
        detail({
          conversation: [
            {
              id: "cm-1",
              author: "Me",
              body: "lost words",
              createdOn: "2026-01-02T00:00:00Z",
              origin: "PORTAL",
              delivery: "FAILED",
              commandId: "cmd-c",
            },
          ],
          commands: [
            cmd({ id: "cmd-c", kind: "ADD_COMMENT", status: "FAILED", lastError: "rate limited" }),
          ],
        }),
      );
      postMock.mockResolvedValue({ command: cmd({ id: "cmd-c", kind: "ADD_COMMENT" }) });
      renderDialog();
      await loaded();
      expect(screen.getByText("Failed")).toBeInTheDocument();
      expect(screen.getByText("rate limited")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Retry" }));
      await waitFor(() =>
        expect(postMock).toHaveBeenCalledWith(
          "/product-work-items/wi-1/commands/cmd-c/retry",
          {},
        ),
      );
    });

    it("shows the server message on a 409 and keeps the text", async () => {
      getMock.mockResolvedValue(detail());
      postMock.mockRejectedValue(new Error("This issue is no longer linked."));
      renderDialog();
      await loaded();
      fireEvent.change(screen.getByLabelText("Comment"), { target: { value: "x" } });
      fireEvent.click(screen.getByRole("button", { name: "Send" }));
      expect(await screen.findByText("This issue is no longer linked.")).toBeInTheDocument();
      expect(screen.getByLabelText("Comment")).toHaveValue("x");
    });

    it("disables Send while the request is in flight (no double submit)", async () => {
      getMock.mockResolvedValue(detail());
      let resolve: (v: unknown) => void = () => {};
      postMock.mockReturnValue(new Promise((r) => (resolve = r)));
      renderDialog();
      await loaded();
      fireEvent.change(screen.getByLabelText("Comment"), { target: { value: "x" } });
      const send = screen.getByRole("button", { name: "Send" });
      fireEvent.click(send);
      await waitFor(() => expect(send).toBeDisabled());
      fireEvent.click(send);
      expect(postMock).toHaveBeenCalledTimes(1);
      await act(async () => {
        resolve({
          comment: { id: "cm-1", author: "Me", body: "x", createdOn: "2026-01-02T00:00:00Z", delivery: "PENDING" },
          command: cmd({ kind: "ADD_COMMENT" }),
        });
      });
    });

    it("reuses the key when the same comment is resent after a failure, and uses a new key for the next comment", async () => {
      getMock.mockResolvedValue(detail());
      const ok = {
        comment: { id: "cm-1", author: "Me", body: "same", createdOn: "2026-01-02T00:00:00Z", delivery: "PENDING" },
        command: cmd({ kind: "ADD_COMMENT" }),
      };
      postMock
        .mockRejectedValueOnce(new Error("network"))
        .mockResolvedValueOnce(ok)
        .mockResolvedValueOnce(ok);
      renderDialog();
      await loaded();
      const box = screen.getByLabelText("Comment");
      fireEvent.change(box, { target: { value: "same" } });
      fireEvent.click(screen.getByRole("button", { name: "Send" }));
      await screen.findByText("network");
      fireEvent.click(screen.getByRole("button", { name: "Send" }));
      await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));
      await waitFor(() => expect(screen.getByLabelText("Comment")).toHaveValue(""));
      // Next composed comment, identical text: still a new key.
      fireEvent.change(screen.getByLabelText("Comment"), { target: { value: "same" } });
      fireEvent.click(screen.getByRole("button", { name: "Send" }));
      await waitFor(() => expect(postMock).toHaveBeenCalledTimes(3));
      const [k1, k2, k3] = postMock.mock.calls.map(keyOf);
      expect(k1).toBeTruthy();
      expect(k2).toBe(k1);
      expect(k3).not.toBe(k1);
    });
  });

  describe("actions", () => {
    it("sends the move with the contract body and refetches the detail and case list", async () => {
      getMock.mockResolvedValue(detail());
      postMock.mockResolvedValue({ command: cmd() });
      const { qc } = renderDialog();
      const invalidate = vi.spyOn(qc, "invalidateQueries");
      await loaded();
      fireEvent.click(screen.getByRole("button", { name: "Back to product team" }));
      await waitFor(() =>
        expect(postMock).toHaveBeenCalledWith(
          "/product-work-items/wi-1/board-status",
          { status: "WAITING_ON_PRODUCT_TEAM" },
          expect.objectContaining({ headers: { "Idempotency-Key": expect.any(String) } }),
        ),
      );
      await waitFor(() => {
        const keys = invalidate.mock.calls.map((c) => (c[0] as { queryKey: unknown[] }).queryKey[0]);
        expect(keys).toContain("csm-product-work-item");
        expect(keys).toContain("csm-case-product-work-items");
      });
    });

    it("closes only after the confirmation dialog", async () => {
      getMock.mockResolvedValue(detail());
      postMock.mockResolvedValue({ command: cmd({ id: "c9", kind: "SET_STATE" }) });
      renderDialog();
      await loaded();
      fireEvent.click(screen.getByRole("button", { name: "Close issue" }));
      expect(postMock).not.toHaveBeenCalled();
      const confirm = await screen.findByText(/Close wso2\/product-is#12 on GitHub\?/);
      const dlg = confirm.closest('[role="dialog"]') as HTMLElement;
      fireEvent.click(within(dlg).getByRole("button", { name: "Close issue" }));
      await waitFor(() =>
        expect(postMock).toHaveBeenCalledWith(
          "/product-work-items/wi-1/close",
          {},
          expect.objectContaining({ headers: { "Idempotency-Key": expect.any(String) } }),
        ),
      );
    });

    it("cancelling the confirmation sends nothing", async () => {
      getMock.mockResolvedValue(detail());
      renderDialog();
      await loaded();
      fireEvent.click(screen.getByRole("button", { name: "Close issue" }));
      fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
      expect(postMock).not.toHaveBeenCalled();
    });

    it("shows the server's message when the move is refused with 409", async () => {
      getMock.mockResolvedValue(detail());
      postMock.mockRejectedValue(new Error("Issue is not waiting on CS."));
      renderDialog();
      await loaded();
      fireEvent.click(screen.getByRole("button", { name: "Back to product team" }));
      expect(await screen.findByText("Issue is not waiting on CS.")).toBeInTheDocument();
    });
  });

  describe("polling", () => {
    it("polls every 5 s while a command is pending and stops once none is", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      getMock.mockResolvedValue(detail({ commands: [cmd()] }));
      renderDialog();
      await loaded();
      const after = getMock.mock.calls.length;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5100);
      });
      expect(getMock.mock.calls.length).toBeGreaterThan(after);

      getMock.mockResolvedValue(detail({ commands: [cmd({ status: "DELIVERED" })] }));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5100);
      });
      const settled = getMock.mock.calls.length;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(20000);
      });
      expect(getMock.mock.calls.length).toBe(settled);
    });

    it("does not poll when no command is pending", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      getMock.mockResolvedValue(detail());
      renderDialog();
      await loaded();
      const n = getMock.mock.calls.length;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(20000);
      });
      expect(getMock.mock.calls.length).toBe(n);
    });
  });
});
