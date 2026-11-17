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

import { useCallback, useLayoutEffect, useRef, useSyncExternalStore } from "react";
import type { PostProjectContactOutcome } from "@features/settings/api/usePostProjectContact";
import { ApiError } from "@utils/ApiError";
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

// Pending invitations live here, keyed by project, rather than in component
// state. An invitation keeps running when the admin leaves the page, and it
// must still be showing, in its current state, when they come back: state
// inside the component is thrown away on unmount and the background work
// would have nowhere to report. A full page reload still clears it; the
// contact list then shows the invitation once it has committed.
const pendingByProject = new Map<string, PendingInvite[]>();
const listeners = new Set<() => void>();
const NONE: PendingInvite[] = [];

function readPending(projectId: string): PendingInvite[] {
  return pendingByProject.get(projectId) ?? NONE;
}

function writePending(
  projectId: string,
  next: (prev: PendingInvite[]) => PendingInvite[],
): void {
  pendingByProject.set(projectId, next(readPending(projectId)));
  listeners.forEach((listener) => listener());
}

function subscribePending(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** Clears every project's pending invitations. For tests only. */
export function resetPendingInvitesForTests(): void {
  pendingByProject.clear();
  listeners.forEach((listener) => listener());
}

export interface UsePendingInvitesOptions {
  projectId: string;
  /** Sends one invitation; resolves with how it ended, rejects on failure. */
  send: (request: CreateProjectContactRequest) => Promise<PostProjectContactOutcome>;
  /**
   * Refetches the contact list and resolves with the fresh rows. Must keep
   * working after the page has unmounted.
   */
  refetchContacts: () => Promise<ProjectContact[] | undefined>;
  /** Called once an invitation is confirmed as a real row. */
  onInvited: (email: string, request: CreateProjectContactRequest) => void;
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
 * seconds. Each invitation runs on its own, so several can be in flight, and
 * each survives the admin navigating away and back.
 *
 * @param {UsePendingInvitesOptions} options - Project, and how to send, refetch and report.
 * @returns {UsePendingInvitesResult} The pending rows and the actions on them.
 */
export function usePendingInvites(options: UsePendingInvitesOptions): UsePendingInvitesResult {
  const { projectId } = options;
  // The latest callbacks, so work started on an earlier render reports
  // through the current ones, and through the last ones after unmount.
  const optionsRef = useRef(options);
  useLayoutEffect(() => {
    optionsRef.current = options;
  });

  const pending = useSyncExternalStore(
    subscribePending,
    () => readPending(projectId),
    () => readPending(projectId),
  );

  const setStatus = (email: string, status: PendingInviteStatus, error?: string) =>
    writePending(projectId, (prev) =>
      prev.map((p) => (sameEmail(p.email, email) ? { ...p, status, error } : p)),
    );

  const remove = useCallback(
    (email: string) =>
      writePending(projectId, (prev) => prev.filter((p) => !sameEmail(p.email, email))),
    [projectId],
  );

  const contactExists = async (email: string): Promise<boolean> => {
    const rows = await optionsRef.current.refetchContacts();
    return !!rows?.some((c) => sameEmail(c.email, email));
  };

  const run = async (request: CreateProjectContactRequest) => {
    const email = request.contactEmail;
    try {
      const outcome = await optionsRef.current.send(request);
      if (outcome === "created") {
        await optionsRef.current.refetchContacts();
        remove(email);
        optionsRef.current.onInvited(email, request);
        return;
      }
      // Accepted but not finished: refresh until the contact appears.
      for (const delay of PENDING_INVITE_POLL_DELAYS_MS) {
        await sleep(delay);
        if (await contactExists(email)) {
          remove(email);
          optionsRef.current.onInvited(email, request);
          return;
        }
      }
      setStatus(email, "processing");
    } catch (err) {
      // A failure can still hide a committed invitation, when the answer was
      // lost on the way back. Look once before reporting it.
      try {
        if (await contactExists(email)) {
          remove(email);
          optionsRef.current.onInvited(email, request);
          return;
        }
      } catch {
        // The list could not be read either; report the original failure.
      }
      const message = err instanceof Error ? err.message : String(err);
      if (err instanceof ApiError && err.status >= 400 && err.status < 500) {
        remove(email);
      } else {
        setStatus(email, "failed", message);
      }
      optionsRef.current.onFailed(email, message);
    }
  };

  const invite = (request: CreateProjectContactRequest): boolean => {
    const running = readPending(projectId).find((p) => sameEmail(p.email, request.contactEmail));
    if (running && running.status === "inviting") return false;
    writePending(projectId, (prev) => [
      { email: request.contactEmail, request, status: "inviting" },
      ...prev.filter((p) => !sameEmail(p.email, request.contactEmail)),
    ]);
    void run(request);
    return true;
  };

  const retry = (email: string) => {
    const item = readPending(projectId).find((p) => sameEmail(p.email, email));
    if (!item || item.status === "inviting") return;
    setStatus(email, "inviting");
    void run(item.request);
  };

  return { pending, invite, retry, dismiss: remove };
}
