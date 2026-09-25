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

import { useCallback, useRef, useState } from "react";
import type { PostProjectContactOutcome } from "@features/settings/api/usePostProjectContact";
import type {
  CreateProjectContactRequest,
  ProjectContact,
} from "@features/settings/types/users";

/**
 * Where an invitation the admin just sent currently stands, until it shows
 * up as a real row in the contact list.
 *
 * - `inviting`: the request is still running.
 * - `failed`: the request failed; `error` says why and it can be retried.
 * - `processing`: the backend accepted it but the contact had not appeared
 *   by the time the list stopped being refreshed.
 */
export type PendingInviteStatus = "inviting" | "failed" | "processing";

export interface PendingInvite {
  email: string;
  request: CreateProjectContactRequest;
  status: PendingInviteStatus;
  error?: string;
}

/** Waits between list refreshes after a 202, about 45 seconds in total. */
export const PENDING_INVITE_POLL_DELAYS_MS = [3000, 6000, 12000, 24000];

const sameEmail = (a?: string | null, b?: string | null): boolean =>
  !!a && !!b && a.trim().toLowerCase() === b.trim().toLowerCase();

const sleep = (ms: number): Promise<void> =>
  new Promise((resolve) => setTimeout(resolve, ms));

export interface UsePendingInvitesOptions {
  /** Sends one invitation; resolves with how it ended, rejects on failure. */
  send: (request: CreateProjectContactRequest) => Promise<PostProjectContactOutcome>;
  /** Refetches the contact list and resolves with the fresh rows. */
  refetchContacts: () => Promise<ProjectContact[] | undefined>;
  /** Called once an invitation is confirmed as a real row. */
  onInvited: (email: string) => void;
  /** Called when an invitation fails, with the reason. */
  onFailed: (email: string, message: string) => void;
}

export interface UsePendingInvitesResult {
  pending: PendingInvite[];
  /** Starts an invitation. Returns false if one for this email is already running. */
  invite: (request: CreateProjectContactRequest) => boolean;
  retry: (email: string) => void;
  dismiss: (email: string) => void;
}

/**
 * Tracks invitations the admin has sent but that are not yet rows in the
 * contact list, so the page never blocks on an invitation that takes several
 * seconds. Each invitation runs on its own, so several can be in flight.
 *
 * @param {UsePendingInvitesOptions} options - How to send, refetch and report.
 * @returns {UsePendingInvitesResult} The pending rows and the actions on them.
 */
export function usePendingInvites({
  send,
  refetchContacts,
  onInvited,
  onFailed,
}: UsePendingInvitesOptions): UsePendingInvitesResult {
  const [pending, setPending] = useState<PendingInvite[]>([]);
  // Mirror of `pending` for the async flow, which must not act on a stale
  // closure (a second invite started while the first is still running).
  const pendingRef = useRef<PendingInvite[]>([]);

  const update = useCallback((next: (prev: PendingInvite[]) => PendingInvite[]) => {
    pendingRef.current = next(pendingRef.current);
    setPending(pendingRef.current);
  }, []);

  const setStatus = useCallback(
    (email: string, status: PendingInviteStatus, error?: string) =>
      update((prev) =>
        prev.map((p) => (sameEmail(p.email, email) ? { ...p, status, error } : p)),
      ),
    [update],
  );

  const remove = useCallback(
    (email: string) => update((prev) => prev.filter((p) => !sameEmail(p.email, email))),
    [update],
  );

  const run = useCallback(
    async (request: CreateProjectContactRequest) => {
      const email = request.contactEmail;
      try {
        const outcome = await send(request);
        if (outcome === "created") {
          await refetchContacts();
          remove(email);
          onInvited(email);
          return;
        }
        // Accepted but not finished: refresh until the contact appears.
        for (const delay of PENDING_INVITE_POLL_DELAYS_MS) {
          await sleep(delay);
          const rows = await refetchContacts();
          if (rows?.some((c) => sameEmail(c.email, email))) {
            remove(email);
            onInvited(email);
            return;
          }
        }
        setStatus(email, "processing");
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        setStatus(email, "failed", message);
        onFailed(email, message);
      }
    },
    [send, refetchContacts, remove, setStatus, onInvited, onFailed],
  );

  const invite = useCallback(
    (request: CreateProjectContactRequest): boolean => {
      const running = pendingRef.current.find((p) => sameEmail(p.email, request.contactEmail));
      if (running && running.status === "inviting") return false;
      update((prev) => [
        { email: request.contactEmail, request, status: "inviting" },
        ...prev.filter((p) => !sameEmail(p.email, request.contactEmail)),
      ]);
      void run(request);
      return true;
    },
    [update, run],
  );

  const retry = useCallback(
    (email: string) => {
      const item = pendingRef.current.find((p) => sameEmail(p.email, email));
      if (!item || item.status === "inviting") return;
      setStatus(email, "inviting");
      void run(item.request);
    },
    [run, setStatus],
  );

  return { pending, invite, retry, dismiss: remove };
}
