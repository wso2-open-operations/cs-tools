// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License
// at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  PENDING_INVITE_POLL_DELAYS_MS,
  resetPendingInvitesForTests,
  usePendingInvites,
} from "@features/settings/hooks/usePendingInvites";
import type { CreateProjectContactRequest, ProjectContact } from "@features/settings/types/users";

const request = (email: string): CreateProjectContactRequest => ({
  contactEmail: email,
  contactFirstName: "Shayan",
  contactLastName: "Test",
  isCsAdmin: false,
  isCsIntegrationUser: false,
  isLead: false,
  isPortalUser: true,
  isSecurityContact: true,
});

const contact = (email: string) => ({ id: email, email }) as ProjectContact;

function setup(overrides: Partial<Parameters<typeof usePendingInvites>[0]> = {}) {
  const options = {
    projectId: "p-1",
    send: vi.fn(),
    refetchContacts: vi.fn().mockResolvedValue([]),
    onInvited: vi.fn(),
    onFailed: vi.fn(),
    ...overrides,
  };
  const hook = renderHook(() => usePendingInvites(options));
  return { ...hook, options };
}

describe("usePendingInvites", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    resetPendingInvitesForTests();
  });
  afterEach(() => vi.useRealTimers());

  it("shows the invitation at once and drops it when it is created", async () => {
    let resolveSend: (v: "created") => void = () => {};
    const { result, options } = setup({
      send: vi.fn(() => new Promise<"created">((r) => (resolveSend = r))),
    });

    act(() => {
      result.current.invite(request("a@acme.com"));
    });
    expect(result.current.pending).toEqual([
      expect.objectContaining({ email: "a@acme.com", status: "inviting" }),
    ]);

    await act(async () => resolveSend("created"));
    expect(options.refetchContacts).toHaveBeenCalled();
    expect(options.onInvited).toHaveBeenCalledWith("a@acme.com", expect.objectContaining({ contactEmail: "a@acme.com" }));
    expect(result.current.pending).toEqual([]);
  });

  it("passes the request to onInvited so callers can tell system users apart", async () => {
    const { result, options } = setup({ send: vi.fn().mockResolvedValue("created") });
    const systemUser = { ...request("sys@acme.com"), isCsIntegrationUser: true, isPortalUser: false };

    await act(async () => {
      result.current.invite(systemUser);
    });

    expect(options.onInvited).toHaveBeenCalledWith("sys@acme.com", systemUser);
  });

  it("keeps a failed invitation with its reason and retries it", async () => {
    const send = vi
      .fn()
      .mockRejectedValueOnce(new Error("This address is already a contact on the project"))
      .mockResolvedValueOnce("created");
    const { result, options } = setup({ send });

    await act(async () => {
      result.current.invite(request("b@acme.com"));
    });
    expect(result.current.pending[0]).toMatchObject({
      status: "failed",
      error: "This address is already a contact on the project",
    });
    expect(options.onFailed).toHaveBeenCalledWith(
      "b@acme.com",
      "This address is already a contact on the project",
    );

    await act(async () => result.current.retry("b@acme.com"));
    expect(send).toHaveBeenCalledTimes(2);
    expect(send).toHaveBeenLastCalledWith(request("b@acme.com"));
    expect(result.current.pending).toEqual([]);
  });

  it("refreshes the list after a 202 until the contact appears", async () => {
    const refetchContacts = vi
      .fn()
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([contact("C@acme.com")]);
    const { result, options } = setup({
      send: vi.fn().mockResolvedValue("processing"),
      refetchContacts,
    });

    await act(async () => {
      result.current.invite(request("c@acme.com"));
    });
    expect(result.current.pending[0].status).toBe("inviting");

    await act(async () => vi.advanceTimersByTimeAsync(PENDING_INVITE_POLL_DELAYS_MS[0]));
    expect(result.current.pending[0].status).toBe("inviting");
    await act(async () => vi.advanceTimersByTimeAsync(PENDING_INVITE_POLL_DELAYS_MS[1]));

    expect(refetchContacts).toHaveBeenCalledTimes(2);
    expect(options.onInvited).toHaveBeenCalledWith("c@acme.com", expect.objectContaining({ contactEmail: "c@acme.com" }));
    expect(result.current.pending).toEqual([]);
  });

  it("marks a 202 invitation as processing, not failed, if it never appears", async () => {
    const { result, options } = setup({ send: vi.fn().mockResolvedValue("processing") });

    await act(async () => {
      result.current.invite(request("d@acme.com"));
    });
    const total = PENDING_INVITE_POLL_DELAYS_MS.reduce((a, b) => a + b, 0);
    await act(async () => vi.advanceTimersByTimeAsync(total));

    expect(result.current.pending[0].status).toBe("processing");
    expect(options.onFailed).not.toHaveBeenCalled();
    act(() => result.current.dismiss("d@acme.com"));
    expect(result.current.pending).toEqual([]);
  });

  it("runs invitations to different addresses side by side and refuses a duplicate", async () => {
    const { result, options } = setup({ send: vi.fn(() => new Promise<"created">(() => {})) });

    let second = true;
    let duplicate = true;
    act(() => {
      result.current.invite(request("e@acme.com"));
      second = result.current.invite(request("f@acme.com"));
    });
    act(() => {
      duplicate = result.current.invite(request("E@acme.com"));
    });

    expect(second).toBe(true);
    expect(duplicate).toBe(false);
    expect(options.send).toHaveBeenCalledTimes(2);
    expect(result.current.pending.map((p) => p.email)).toEqual(["f@acme.com", "e@acme.com"]);
  });

  it("keeps an invitation, and its outcome, when the page unmounts and comes back", async () => {
    let resolveSend: (v: "created") => void = () => {};
    const send = vi.fn(() => new Promise<"created">((r) => (resolveSend = r)));
    const first = setup({ send });

    act(() => {
      first.result.current.invite(request("g@acme.com"));
    });
    first.unmount();

    // Back on the page while the request is still running.
    const second = setup({ send });
    expect(second.result.current.pending).toEqual([
      expect.objectContaining({ email: "g@acme.com", status: "inviting" }),
    ]);

    await act(async () => resolveSend("created"));
    expect(second.result.current.pending).toEqual([]);
    expect(first.options.onInvited).toHaveBeenCalledWith("g@acme.com", expect.objectContaining({ contactEmail: "g@acme.com" }));
  });

  it("finishes an invitation while the page is away, and shows nothing stale on return", async () => {
    let resolveSend: (v: "created") => void = () => {};
    const first = setup({ send: vi.fn(() => new Promise<"created">((r) => (resolveSend = r))) });
    act(() => {
      first.result.current.invite(request("h@acme.com"));
    });
    first.unmount();

    await act(async () => resolveSend("created"));

    const second = setup();
    expect(second.result.current.pending).toEqual([]);
  });

  it("keeps each project's invitations to itself", () => {
    const a = setup({ projectId: "p-1", send: vi.fn(() => new Promise<"created">(() => {})) });
    act(() => {
      a.result.current.invite(request("i@acme.com"));
    });
    const b = setup({ projectId: "p-2" });
    expect(b.result.current.pending).toEqual([]);
    expect(a.result.current.pending).toHaveLength(1);
  });

  it("retries a processing invitation, and a retry of one that did commit ends as invited", async () => {
    const send = vi.fn().mockResolvedValue("processing");
    const refetchContacts = vi.fn().mockResolvedValue([]);
    const { result, options } = setup({ send, refetchContacts });

    await act(async () => {
      result.current.invite(request("r@acme.com"));
    });
    const total = PENDING_INVITE_POLL_DELAYS_MS.reduce((a, b) => a + b, 0);
    await act(async () => vi.advanceTimersByTimeAsync(total));
    expect(result.current.pending[0].status).toBe("processing");

    // The first request did commit after all: the retry is refused as a
    // duplicate, and the list check turns that into success.
    send.mockRejectedValueOnce(new Error("This address is already a contact on the project"));
    refetchContacts.mockResolvedValue([contact("r@acme.com")]);
    await act(async () => {
      result.current.retry("r@acme.com");
    });

    expect(send).toHaveBeenCalledTimes(2);
    expect(result.current.pending).toEqual([]);
    expect(options.onInvited).toHaveBeenCalledWith("r@acme.com", expect.objectContaining({ contactEmail: "r@acme.com" }));
    expect(options.onFailed).not.toHaveBeenCalled();
  });

  it("treats a failure as success when the contact turns out to exist", async () => {
    const { result, options } = setup({
      send: vi.fn().mockRejectedValue(new Error("upstream status 502")),
      refetchContacts: vi.fn().mockResolvedValue([contact("J@acme.com")]),
    });

    await act(async () => {
      result.current.invite(request("j@acme.com"));
    });

    expect(result.current.pending).toEqual([]);
    expect(options.onInvited).toHaveBeenCalledWith("j@acme.com", expect.objectContaining({ contactEmail: "j@acme.com" }));
    expect(options.onFailed).not.toHaveBeenCalled();
  });

  it("reports the original failure when the list cannot be read either", async () => {
    const { result, options } = setup({
      send: vi.fn().mockRejectedValue(new Error("Salesforce rejected the contact")),
      refetchContacts: vi.fn().mockRejectedValue(new Error("network down")),
    });

    await act(async () => {
      result.current.invite(request("k@acme.com"));
    });

    expect(result.current.pending[0]).toMatchObject({ status: "failed", error: "Salesforce rejected the contact" });
    expect(options.onFailed).toHaveBeenCalledWith("k@acme.com", "Salesforce rejected the contact");
  });
});
