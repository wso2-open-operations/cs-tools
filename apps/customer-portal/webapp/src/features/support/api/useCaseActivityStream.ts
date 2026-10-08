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

import { useEffect } from "react";
import EventSourcePolyfill from "@sanity/eventsource";
import { useAsgardeo } from "@asgardeo/react";
import { useQueryClient } from "@tanstack/react-query";
import { apiConfig } from "@config/apiConfig";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useLogger } from "@hooks/useLogger";

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
 * Refetches the case's comments and details whenever the activity stream
 * reports the case changed. No-op unless both stream config keys are set.
 *
 * Uses `@sanity/eventsource` because native `EventSource` cannot set headers.
 * Sends the same headers as useAuthApiClient — the gateway injects
 * `x-jwt-assertion` itself, so this doesn't set it.
 *
 * Reconnects by hand rather than letting the polyfill retry: its headers are
 * fixed at construction, so a token expiring mid-connection would have it
 * retry forever with the same stale one.
 */
export function useCaseActivityStream(caseId: string | undefined): void {
  const queryClient = useQueryClient();
  const { getIdToken } = useAsgardeo();
  const logger = useLogger();

  useEffect(() => {
    if (!caseId || !apiConfig.streamEnabled || !apiConfig.streamUrl) return;

    let cancelled = false;
    let source: EventSourcePolyfill | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;

    const invalidateCaseQueries = (): void => {
      // Invalidated by key prefix rather than the queries' full keys, which
      // also carry the project id this hook isn't given — the same thing
      // usePostComment already does after posting a comment.
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_COMMENTS],
      });
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_DETAILS],
      });
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
        // Only *consecutive* failures should back off.
        attempt = 0;

        // Nothing published before the subscriber registers is replayed, so
        // refetch on every connection to cover that window.
        invalidateCaseQueries();
      });

      source.addEventListener("case_updated", invalidateCaseQueries);

      source.addEventListener("error", () => {
        logger.debug("[case-activity-stream] connection error, reconnecting");
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
      source?.close();
    };
  }, [caseId, queryClient, getIdToken, logger]);
}
