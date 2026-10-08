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
import { useQueryClient } from "@tanstack/react-query";
import EventSourcePolyfill from "@sanity/eventsource";
import { STREAM_URL } from "@config/endpoints";
import { getAccessToken, getIdToken, refreshToken } from "@src/services/auth";
import { Logger } from "@utils/logger";

/** Base delay before the first reconnect attempt after the stream errors out or drops. */
const RECONNECT_BASE_DELAY_MS = 3_000;
/** Reconnect delay never grows past this while still within MAX_BACKOFF_ATTEMPTS. */
const RECONNECT_MAX_DELAY_MS = 30_000;
/**
 * After this many consecutive failures (the backoff curve has long since
 * maxed out at RECONNECT_MAX_DELAY_MS by then), stop retrying every ≤30s —
 * a sustained outage/misconfiguration (e.g. the backend rejecting every
 * caller with a 403) would otherwise have every open case-detail tab call
 * the backend roughly twice a minute forever.
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
 * Opens a live Server-Sent Events connection to csm-portal-backend's
 * `GET /cases/{id}/activities/stream` (its dedicated :9092 listener — see
 * that backend's cmd/server/main.go) and invalidates the case's comments
 * (`["case", caseId, "comments"]`) and activities (`["case", caseId,
 * "activities"]`) queries — see services/cases.ts / services/activities.ts —
 * whenever it emits a `case_updated` event, so the Activities tab reflects a
 * new comment or status change without the viewer refreshing manually.
 *
 * Uses `@sanity/eventsource` rather than the browser's native `EventSource`
 * because native EventSource cannot set custom headers — it only supports
 * cookies/query params for auth. Unlike services/apiClient.ts, which sets
 * `Authorization` and relies on Choreo's gateway to translate it into
 * `x-jwt-assertion` before the main :8080 API ever sees it, this connects to
 * this stream's own, separately-declared Choreo endpoint (see
 * openapi-stream.yaml) — not guaranteed to apply the same translation — so
 * this hook sets `x-jwt-assertion`/`x-user-id-token` directly, matching
 * exactly what the backend's `middleware.Auth` reads. There is no separate
 * ticket/token-exchange step.
 *
 * Headers are fixed at construction time, so they can't be refreshed on the
 * library's own built-in reconnect — a token that expires mid-connection
 * would otherwise have the polyfill retry forever with the same stale
 * header. Instead, `error` closes the current connection and this hook
 * opens a fresh one after an exponentially backed-off delay (see
 * reconnectDelay), rather than relying on that built-in retry.
 * refreshToken(false) on that reconnect — not forced — so an ordinary
 * transient failure (network blip, backend restart) doesn't force a
 * redundant IdP round-trip on every single retry; refreshToken still
 * refreshes on its own whenever the cached token is actually expiring.
 *
 * A no-op when `caseId` is unset or STREAM_URL isn't configured (Event Hub —
 * and therefore this endpoint — is optional on the backend); callers fall
 * back to the comments/activities queries' own staleTime.
 */
export function useCaseActivityStream(caseId: string | undefined): void {
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!caseId || !STREAM_URL) return;

    let cancelled = false;
    let source: EventSourcePolyfill | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;

    const scheduleReconnect = (): void => {
      const delay = reconnectDelay(attempt);
      attempt += 1;
      reconnectTimer = setTimeout(() => void connect(false), delay);
    };

    const connect = async (forceRefresh: boolean): Promise<void> => {
      try {
        await refreshToken(forceRefresh);
      } catch (error) {
        Logger.warn(
          "Failed to refresh token for case activity stream",
          error instanceof Error ? error.message : "Unknown token error",
        );
      }
      if (cancelled) return;

      const token = getAccessToken();
      const idToken = getIdToken();
      if (!token || !idToken) {
        scheduleReconnect();
        return;
      }

      const url = `${STREAM_URL}/cases/${encodeURIComponent(caseId)}/activities/stream`;
      source = new EventSourcePolyfill(url, {
        headers: {
          "x-jwt-assertion": token,
          "x-user-id-token": idToken,
        },
      });

      // A successful connection resets the backoff — only *consecutive*
      // failures should back off, not the cumulative count over the
      // component's whole lifetime.
      source.addEventListener("open", () => {
        attempt = 0;
      });

      source.addEventListener("case_updated", () => {
        void queryClient.invalidateQueries({ queryKey: ["case", caseId, "comments"] });
        void queryClient.invalidateQueries({ queryKey: ["case", caseId, "activities"] });
      });

      source.addEventListener("error", () => {
        Logger.warn("Case activity stream error, reconnecting");
        source?.close();
        if (!cancelled) {
          scheduleReconnect();
        }
      });
    };

    void connect(false);

    return () => {
      cancelled = true;
      clearTimeout(reconnectTimer);
      source?.close();
    };
  }, [caseId, queryClient]);
}
