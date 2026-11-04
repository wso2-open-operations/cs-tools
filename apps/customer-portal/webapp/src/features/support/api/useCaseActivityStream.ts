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

import { useEffect, useRef } from "react";
import EventSourcePolyfill from "@sanity/eventsource";
import { useAsgardeo } from "@asgardeo/react";
import { useQueryClient } from "@tanstack/react-query";
import { apiConfig } from "@config/apiConfig";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { refreshCaseComments } from "@features/support/api/caseCommentsRefresh";
import { useIsIdle } from "@providers/IdleStateContext";

/** Base delay before the first reconnect attempt after the stream errors out or drops. */
const RECONNECT_BASE_DELAY_MS = 3_000;
/** Reconnect delay never grows past this while still within MAX_BACKOFF_ATTEMPTS. */
const RECONNECT_MAX_DELAY_MS = 30_000;
/**
 * After this many consecutive failures (the backoff curve has long since
 * maxed out at RECONNECT_MAX_DELAY_MS by then), stop retrying every ≤30s —
 * a sustained outage/misconfiguration (e.g. the backend rejecting every
 * caller) would otherwise have every open case tab call the backend
 * roughly twice a minute forever.
 */
const MAX_BACKOFF_ATTEMPTS = 10;
/** Retry interval once MAX_BACKOFF_ATTEMPTS is exceeded — still eventually recovers once the backend does, at a fraction of the call volume. */
const RECONNECT_IDLE_DELAY_MS = 5 * 60_000;

/**
 * A connection must stay open this long (or deliver an event) before it counts
 * as healthy and the backoff resets. The stream can answer 200 and then be cut
 * straight away; resetting on `open` would turn that into a tight reconnect loop.
 */
const STABLE_CONNECTION_MS = 30_000;
/** Minimum gap between case-details invalidations triggered by a routine reconnect, per case. */
const CATCH_UP_MIN_INTERVAL_MS = 60_000;

/** Event `type` values the server sends in the `case_updated` payload. */
const COMMENT_ADDED_TYPE = "case.comment_added";

/** Case-scoped queries that render the case's own attributes. */
const isCaseDetailsQuery = (caseId: string) => (query: { queryKey: readonly unknown[] }) =>
  query.queryKey[0] === ApiQueryKeys.CASE_DETAILS && query.queryKey[2] === caseId;

/**
 * Case list/search queries share the PROJECT_CASES prefix with the project
 * filters query (static metadata), which a case change never affects.
 */
const isCaseListQuery = (query: { queryKey: readonly unknown[] }): boolean =>
  query.queryKey[0] === ApiQueryKeys.PROJECT_CASES && query.queryKey[1] !== "filters";

/** Counts derived from case state and severity (open/ongoing/outstanding, SLA status). */
const CASE_STATS_KEYS = [
  ApiQueryKeys.CASES_STATS,
  ApiQueryKeys.SUPPORT_STATS,
  ApiQueryKeys.PROJECT_STATS,
] as const;

/** Reads the `type` of a `case_updated` payload; undefined if absent or unparseable. */
function parseEventType(event: Event): string | undefined {
  const data = (event as MessageEvent).data;
  if (typeof data !== "string") return undefined;
  try {
    const parsed: unknown = JSON.parse(data);
    if (parsed && typeof parsed === "object" && "type" in parsed) {
      const type = (parsed as { type: unknown }).type;
      return typeof type === "string" ? type : undefined;
    }
  } catch {
    // Malformed payload: treated as an unknown change.
  }
  return undefined;
}

/**
 * Exponential backoff with full jitter (attempt 0 is a random delay in
 * [0, base), attempt 1 in [0, base*2), ... capped at max) — a sustained
 * backend outage or misconfiguration would otherwise have every open
 * case-detail tab retry in lockstep every RECONNECT_BASE_DELAY_MS forever,
 * hammering the endpoint indefinitely instead of backing off. Past
 * MAX_BACKOFF_ATTEMPTS consecutive failures, falls back to a slow idle
 * retry instead of continuing to hammer the endpoint every ≤30s.
 */
function reconnectDelay(attempt: number): number {
  if (attempt >= MAX_BACKOFF_ATTEMPTS) return RECONNECT_IDLE_DELAY_MS;
  const capped = Math.min(RECONNECT_MAX_DELAY_MS, RECONNECT_BASE_DELAY_MS * 2 ** attempt);
  return Math.random() * capped;
}

/**
 * Keeps the case's comments, details and the lists/stats that show its
 * attributes fresh while the activity stream reports changes. A comment event
 * only pulls the newest comments into the cache; any other (or unrecognised)
 * change also refreshes case details, case lists and stats. No-op unless both
 * stream config keys are set.
 *
 * Uses `@sanity/eventsource` because native `EventSource` cannot set headers.
 * Sends the same headers as useAuthApiClient — the gateway injects
 * `x-jwt-assertion` itself, so this doesn't set it.
 *
 * Reconnects by hand rather than letting the polyfill retry: its headers are
 * fixed at construction, so a token expiring mid-connection would have it
 * retry forever with the same stale one.
 *
 * Connecting or reconnecting never calls the comments/activities API: the
 * server ends each stream cleanly every few minutes, so routine reconnects are
 * frequent. An event published in the short gap between connections is
 * recovered by the next event or a reload. A routine reconnect only
 * invalidates case details, at most once a minute per case.
 *
 * Stays disconnected while the user is idle (the session warning is open), so
 * an abandoned tab doesn't hold a long-lived stream open. The gap can be
 * minutes to hours, so the first `open` after idle clears catches up once:
 * the newest comments plus case details, regardless of the one-minute limit.
 */
export function useCaseActivityStream(caseId: string | undefined): void {
  const queryClient = useQueryClient();
  const { getIdToken } = useAsgardeo();
  const logger = useLogger();
  const isIdle = useIsIdle();
  const authFetch = useAuthApiClient();

  // authFetch is a new closure every render; keep it out of the effect deps so
  // a re-render does not tear down and reopen the stream.
  const authFetchRef = useRef(authFetch);
  useEffect(() => {
    authFetchRef.current = authFetch;
  }, [authFetch]);

  // Survives the effect re-running (idle -> active) for the same case, so the
  // first open after idle counts as a reconnect and the details rate limit holds.
  // `catchUpOnOpen` is set when idle begins on an already-opened stream and
  // consumed by the next `open`.
  const connectionRef = useRef<{
    caseId?: string;
    hasOpened: boolean;
    lastCatchUpAt: number;
    catchUpOnOpen: boolean;
  }>({
    hasOpened: false,
    lastCatchUpAt: 0,
    catchUpOnOpen: false,
  });

  useEffect(() => {
    if (isIdle) {
      const idleConnection = connectionRef.current;
      if (idleConnection.caseId === caseId && idleConnection.hasOpened) {
        idleConnection.catchUpOnOpen = true;
      }
      return;
    }
    if (!caseId || !apiConfig.streamEnabled || !apiConfig.streamUrl) return;

    if (connectionRef.current.caseId !== caseId) {
      connectionRef.current = { caseId, hasOpened: false, lastCatchUpAt: 0, catchUpOnOpen: false };
    }
    const connection = connectionRef.current;

    let cancelled = false;
    let source: EventSourcePolyfill | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let stableTimer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;

    const refreshComments = (): void => {
      void refreshCaseComments(queryClient, authFetchRef.current, caseId, logger);
    };

    const invalidateCaseDetails = (): void => {
      void queryClient.invalidateQueries({ predicate: isCaseDetailsQuery(caseId) });
    };

    // Case attributes changed (or we cannot tell what changed): refresh
    // everything that renders them. Only mounted queries refetch right away;
    // the rest are marked stale and refetch on next mount. Case lists are
    // matched by predicate so the static project filters query is left alone.
    const invalidateCaseAttributes = (): void => {
      invalidateCaseDetails();
      void queryClient.invalidateQueries({ predicate: isCaseListQuery });
      for (const key of CASE_STATS_KEYS) {
        void queryClient.invalidateQueries({ queryKey: [key] });
      }
    };

    const handleCaseUpdated = (event: Event): void => {
      // An event proves the connection is alive.
      attempt = 0;
      refreshComments();
      if (parseEventType(event) !== COMMENT_ADDED_TYPE) {
        invalidateCaseAttributes();
      }
    };

    const scheduleReconnect = (): void => {
      const delay = reconnectDelay(attempt);
      attempt += 1;
      reconnectTimer = setTimeout(() => void connect(), delay);
    };

    const connect = async (): Promise<void> => {
      let idToken: string | undefined;
      try {
        idToken = await getIdToken();
      } catch (error) {
        logger.debug(
          "[case-activity-stream] failed to get ID token",
          error instanceof Error ? error.message : "Unknown token error",
        );
      }
      if (cancelled) return;
      if (!idToken) {
        scheduleReconnect();
        return;
      }

      const url = `${apiConfig.streamUrl}/cases/${encodeURIComponent(caseId)}/activities/stream`;
      source = new EventSourcePolyfill(url, {
        headers: {
          Authorization: `Bearer ${idToken}`,
          "x-user-id-token": idToken,
        },
      });

      source.addEventListener("open", () => {
        // Only a connection that stays up resets the backoff; one that opens
        // and is cut straight away keeps backing off.
        clearTimeout(stableTimer);
        stableTimer = setTimeout(() => {
          attempt = 0;
        }, STABLE_CONNECTION_MS);

        // No comments/activities request here: routine reconnects are frequent
        // and an event in the reconnect gap is recovered by the next event.
        const isReconnect = connection.hasOpened;
        connection.hasOpened = true;
        const now = Date.now();

        // Resume after idle: the gap can be long, so catch up once, exempt
        // from the rate limit.
        if (connection.catchUpOnOpen) {
          connection.catchUpOnOpen = false;
          connection.lastCatchUpAt = now;
          refreshComments();
          invalidateCaseDetails();
          return;
        }

        // Routine reconnect: refresh case details only, rate-limited so a
        // flapping connection cannot turn this into a request storm.
        if (now - connection.lastCatchUpAt < CATCH_UP_MIN_INTERVAL_MS) return;
        connection.lastCatchUpAt = now;
        if (isReconnect) invalidateCaseDetails();
      });

      source.addEventListener("case_updated", handleCaseUpdated);

      source.addEventListener("error", () => {
        logger.debug("[case-activity-stream] connection error, reconnecting");
        clearTimeout(stableTimer);
        source?.close();
        if (!cancelled) {
          scheduleReconnect();
        }
      });
    };

    void connect();

    return () => {
      cancelled = true;
      clearTimeout(reconnectTimer);
      clearTimeout(stableTimer);
      source?.close();
    };
  }, [caseId, isIdle, queryClient, getIdToken, logger]);
}
