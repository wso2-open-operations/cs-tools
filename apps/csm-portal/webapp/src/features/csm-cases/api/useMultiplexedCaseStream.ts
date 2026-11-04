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

import { useEffect, useRef, useState } from "react";
import EventSourcePolyfill from "@sanity/eventsource";
import { useQueryClient } from "@tanstack/react-query";
import { apiConfig } from "@config/apiConfig";
import { useAuthTokens } from "@hooks/useAuthTokens";
import { useLogger } from "@hooks/useLogger";
import {
  MAX_BACKOFF_ATTEMPTS,
  invalidateCaseById,
  invalidateCaseLists,
  invalidateCaseQueries,
  invalidateCaseThreadById,
  reconnectDelay,
} from "@features/csm-cases/api/caseStreamShared";

/** The backend rejects more distinct ids than this with a 400. */
export const MAX_STREAM_CASE_IDS = 20;
/** How long the id set must stay unchanged before the stream is (re)opened. */
export const ID_SET_DEBOUNCE_MS = 300;

/** Statuses meaning "this backend does not serve the multiplexed endpoint". */
const UNSUPPORTED_STATUSES = new Set([404, 405]);

/**
 * Pulls the case id out of a `case_updated` frame's `data`. The payload is
 * compact JSON `{caseId, type, timestamp}`; anything else (bad JSON, a
 * non-object, a missing/empty `caseId`) yields `undefined` so the caller can
 * ignore the frame rather than throw inside an event handler.
 */
export function parseCaseUpdatedCaseId(data: unknown): string | undefined {
  if (typeof data !== "string") return undefined;
  try {
    const parsed: unknown = JSON.parse(data);
    if (parsed && typeof parsed === "object" && "caseId" in parsed) {
      const { caseId } = parsed as { caseId: unknown };
      if (typeof caseId === "string" && caseId.length > 0) return caseId;
    }
  } catch {
    /* fall through */
  }
  return undefined;
}

/**
 * Keeps ONE Server-Sent Events connection to the activity-stream service's
 * `GET /cases/activities/stream?ids=a,b,c` open for every id in `caseIds`,
 * and on each `case_updated` frame:
 *  1. invalidates that case's comments/activities/detail/SLA queries plus the
 *     case lists (see `invalidateCaseQueries`), so any mounted view of it
 *     catches up without waiting out its own staleTime; and
 *  2. calls `onCaseUpdated(caseId)` so the caller can mark UI state (e.g. a
 *     case tab that is not on screen).
 *
 * Auth, backoff and gating follow the conventions of the earlier per-case
 * stream: `Authorization: Bearer <access token>` plus `x-user-id-token`, fetched
 * through `useAuthTokens` afresh on every (re)connect because EventSource
 * headers are fixed at construction time; exponential backoff with full jitter
 * (`reconnectDelay`); and a no-op when `apiConfig.streamEnabled` is off or
 * `apiConfig.streamUrl` is unset.
 *
 * - The id set is debounced (`ID_SET_DEBOUNCE_MS`): opening or closing several
 *   tabs in a burst reconnects once. At most one connection exists at any time;
 *   the old one is closed before the new one is opened.
 * - Ids the caller cannot read are silently dropped server-side, so a requested
 *   id is not guaranteed to be live. Frames for ids that were never requested
 *   are ignored.
 * - The connection stays open while the browser page is hidden, so a background
 *   case tab is still flagged when the user is on another browser tab. It is
 *   still a single connection, so keeping it costs one slot, not one per tab.
 * - A 404/405 means the backend predates this endpoint. Retrying every few
 *   seconds would be pointless, so the hook drops straight to the slow idle
 *   retry (`RECONNECT_IDLE_DELAY_MS`), which still recovers after a deploy.
 *   The app keeps working: queries fall back to their own staleTime.
 *
 * `caseIds` must be de-duplicated, at most `MAX_STREAM_CASE_IDS` long, and
 * contain only ids the backend accepts (UUIDs); one malformed id would 400 the
 * whole stream.
 */
export function useMultiplexedCaseStream(
  caseIds: readonly string[],
  onCaseUpdated?: (caseId: string) => void,
): void {
  const queryClient = useQueryClient();
  const getTokens = useAuthTokens();
  const logger = useLogger();

  // Order-insensitive, so reordering tabs never reconnects.
  const idsKey = [...new Set(caseIds)].sort().join(",");

  const [settledKey, setSettledKey] = useState(idsKey);
  useEffect(() => {
    if (idsKey === settledKey) return;
    const timer = setTimeout(() => setSettledKey(idsKey), ID_SET_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [idsKey, settledKey]);

  // Read through refs so a new callback/logger/token-getter identity never
  // reconnects the stream: this hook lives in a component that re-renders on
  // every tab-state change, and only a changed id set may
  // tear the connection down.
  const onCaseUpdatedRef = useRef(onCaseUpdated);
  const getTokensRef = useRef(getTokens);
  const loggerRef = useRef(logger);
  useEffect(() => {
    onCaseUpdatedRef.current = onCaseUpdated;
    getTokensRef.current = getTokens;
    loggerRef.current = logger;
  });

  // Ids of the most recent connection that opened, kept across effect runs
  // (an id-set change tears one connection down and opens the next).
  const openedIdsRef = useRef<Set<string> | null>(null);

  // Lower-cased ids of the previous settled set; null until the first settle
  // with the stream enabled.
  const settledIdsRef = useRef<Set<string> | null>(null);

  useEffect(() => {
    if (!apiConfig.streamEnabled || !apiConfig.streamUrl) return;

    const ids = settledKey ? settledKey.split(",") : [];

    // An id that re-enters the set after being absent got no frames while it
    // was unsubscribed, and its comments/activities may still be inside their
    // staleTime when its view remounts, so revalidate them. The first settle
    // is skipped: nothing was ever cached for an id that was never absent
    // (mount fetches it). Ids that stay subscribed are left alone. Runs even
    // when `ids` is empty so that a later reopen of the last closed case counts
    // as re-entering.
    const previousSettled = settledIdsRef.current;
    settledIdsRef.current = new Set(ids.map((id) => id.toLowerCase()));
    if (previousSettled) {
      for (const id of ids) {
        if (!previousSettled.has(id.toLowerCase())) invalidateCaseThreadById(queryClient, id);
      }
    }

    if (ids.length === 0) return;

    const requested = new Map(ids.map((id) => [id.toLowerCase(), id]));
    let cancelled = false;
    let source: EventSourcePolyfill | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;

    const scheduleReconnect = (): void => {
      const delay = reconnectDelay(attempt);
      attempt += 1;
      reconnectTimer = setTimeout(() => void connect(), delay);
    };

    const connect = async (): Promise<void> => {
      let token: string | undefined;
      let idToken: string | undefined;
      try {
        ({ token, idToken } = await getTokensRef.current());
      } catch (error) {
        loggerRef.current.debug(
          "[case-stream] failed to get tokens",
          error instanceof Error ? error.message : "Unknown token error",
        );
      }
      if (cancelled) return;
      if (!token || !idToken) {
        scheduleReconnect();
        return;
      }

      const url = `${apiConfig.streamUrl}/cases/activities/stream?ids=${ids
        .map(encodeURIComponent)
        .join(",")}`;
      const current = new EventSourcePolyfill(url, {
        headers: {
          Authorization: `Bearer ${token}`,
          "x-user-id-token": idToken,
        },
      });
      source = current;

      // A successful connection resets the backoff: only *consecutive*
      // failures should back off.
      current.addEventListener("open", () => {
        attempt = 0;
        // Frames published between the previous connection closing (error,
        // or the id set changing) and this one opening were never delivered
        // and are not replayed, so catch up the cases that were already
        // subscribed (newly added ids were just loaded) and the case lists.
        const previous = openedIdsRef.current;
        if (previous) {
          const carriedOver = ids.filter((id) => previous.has(id.toLowerCase()));
          for (const id of carriedOver) invalidateCaseById(queryClient, id);
          // The lists are shared by every case, so they go stale on any
          // gap, even when no id carried over.
          invalidateCaseLists(queryClient);
        }
        openedIdsRef.current = new Set(ids.map((id) => id.toLowerCase()));
      });

      current.addEventListener("case_updated", (event) => {
        const caseId = parseCaseUpdatedCaseId((event as MessageEvent).data);
        if (!caseId) {
          loggerRef.current.debug("[case-stream] ignoring case_updated frame with no usable caseId");
          return;
        }
        const requestedId = requested.get(caseId.toLowerCase());
        if (!requestedId) {
          loggerRef.current.debug("[case-stream] ignoring case_updated frame for an id that was not requested");
          return;
        }
        invalidateCaseQueries(queryClient, requestedId);
        onCaseUpdatedRef.current?.(requestedId);
      });

      current.addEventListener("error", (event) => {
        const status = (event as Event & { status?: number }).status;
        current.close();
        if (cancelled) return;
        if (status !== undefined && UNSUPPORTED_STATUSES.has(status)) {
          loggerRef.current.debug(
            `[case-stream] endpoint answered ${status}; backend does not support the multiplexed stream yet, backing off`,
          );
          attempt = MAX_BACKOFF_ATTEMPTS;
        } else {
          loggerRef.current.debug("[case-stream] connection error, reconnecting");
        }
        scheduleReconnect();
      });
    };

    void connect();

    return () => {
      cancelled = true;
      clearTimeout(reconnectTimer);
      source?.close();
    };
  }, [settledKey, queryClient]);
}
