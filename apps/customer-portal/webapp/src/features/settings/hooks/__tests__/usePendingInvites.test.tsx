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
  beforeEach(() => vi.useFakeTimers());
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
    expect(options.onInvited).toHaveBeenCalledWith("a@acme.com");
    expect(result.current.pending).toEqual([]);
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
    expect(options.onInvited).toHaveBeenCalledWith("c@acme.com");
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
});
