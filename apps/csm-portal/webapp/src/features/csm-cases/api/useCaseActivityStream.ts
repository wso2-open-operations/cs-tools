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

import { useEffect, useRef, useSyncExternalStore } from "react";
import EventSourcePolyfill from "@sanity/eventsource";
import { useQueryClient } from "@tanstack/react-query";
import { apiConfig } from "@config/apiConfig";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useIsCaseTabVisible } from "@context/case-tabs/CaseTabVisibilityContext";
import { useAuthTokens } from "@hooks/useAuthTokens";
import { useLogger } from "@hooks/useLogger";

/** Base delay before the first reconnect attempt after the stream errors out or drops. */
const RECONNECT_BASE_DELAY_MS = 3_000;
/** Reconnect delay never grows past this, no matter how many consecutive failures. */
const RECONNECT_MAX_DELAY_MS = 30_000;

/**
 * Exponential backoff with full jitter (attempt 0 is a random delay in
 * [0, base), attempt 1 in [0, base*2), ... capped at max) — a sustained
 * backend outage or misconfiguration would otherwise have every open
 * case-detail tab retry in lockstep every RECONNECT_BASE_DELAY_MS forever,
 * hammering the endpoint indefinitely instead of backing off.
 */
function reconnectDelay(attempt: number): number {
  const capped = Math.min(RECONNECT_MAX_DELAY_MS, RECONNECT_BASE_DELAY_MS * 2 ** attempt);
  return Math.random() * capped;
}

function subscribePageVisibility(onChange: () => void): () => void {
  document.addEventListener("visibilitychange", onChange);
  return () => document.removeEventListener("visibilitychange", onChange);
}

/** Whether the browser tab itself is in the foreground. */
function usePageVisible(): boolean {
  return useSyncExternalStore(
    subscribePageVisibility,
    () => document.visibilityState !== "hidden",
    () => true,
  );
}

/**
 * Opens a live Server-Sent Events connection to csm-portal-backend's
 * `GET /cases/{id}/activities/stream` (its dedicated :9092 listener — see
 * that backend's cmd/server/main.go) and invalidates the case's comments and
 * activities queries whenever it emits a `case_updated` event, so the
 * Activities tab reflects a new comment or status change without the viewer
 * having to wait out CSM_CASE_COMMENTS'/CSM_CASE_ACTIVITIES' own staleTime or
 * refresh manually.
 *
 * Uses `@sanity/eventsource` rather than the browser's native `EventSource`
 * because native EventSource cannot set custom headers — it only supports
 * cookies/query params for auth. Unlike useAuthApiClient.ts, which sets
 * `Authorization` and relies on Choreo's gateway to translate it into
 * `x-jwt-assertion` before the main :8080 API ever sees it, this connects to
 * this stream's own, separately-declared Choreo endpoint (see
 * openapi-stream.yaml) — not guaranteed to apply the same translation — so
 * this hook sets `x-jwt-assertion`/`x-user-id-token` directly, matching
 * exactly what backend's `middleware.Auth` reads. There is no separate
 * ticket/token-exchange step.
 *
 * Headers are fixed at construction time, so they can't be refreshed on the
 * library's own built-in reconnect — a token that expires mid-connection
 * would otherwise have the polyfill retry forever with the same stale
 * header. Instead, `error` closes the current connection and this hook
 * opens a fresh one with newly-fetched tokens after an exponentially
 * backed-off delay (see reconnectDelay), rather than relying on that
 * built-in retry.
 *
 * Token acquisition goes through useAuthTokens — the same recovery path
 * useAuthApiClient uses for every other backend call — rather than calling
 * the identity SDK hook directly: a genuinely dead refresh token then gets the same
 * silent-reauth-then-sign-in-redirect treatment as the rest of the app,
 * instead of this hook just retrying forever with no way to ever recover
 * and no visible signal that anything's wrong.
 *
 * The connection is held only while it can be seen: this page is the active
 * case tab (`useIsCaseTabVisible` — the workspace keeps every opened tab's page
 * mounted, hidden with CSS) AND the browser tab is in the foreground. Each
 * open stream permanently occupies one of the browser's per-host connections
 * (6 on HTTP/1.1), so one stream per mounted hidden tab starved every other
 * request once six case tabs were open. Going inactive closes the stream and
 * cancels any pending reconnect; coming back invalidates the queries once,
 * because events emitted in the meantime were not delivered. That
 * revalidation (case detail, comments, activities) also runs when no stream is
 * configured.
 *
 * A no-op when `caseId` is unset, `apiConfig.streamEnabled` is false (the
 * feature's master switch, `CSM_PORTAL_STREAM_ENABLED` — defaults off), or
 * `apiConfig.streamUrl` isn't configured (Event Hub — and therefore this
 * endpoint — is optional on the backend); callers fall back to the
 * comments/activities queries' own staleTime.
 */
export function useCaseActivityStream(caseId: string | undefined): void {
  const queryClient = useQueryClient();
  const getTokens = useAuthTokens();
  const logger = useLogger();
  // Both hooks run unconditionally (no `&&` short-circuit between the calls).
  const caseTabVisible = useIsCaseTabVisible();
  const pageVisible = usePageVisible();
  const active = caseTabVisible && pageVisible;
  // True while the case tab / browser tab is inactive, so the run that
  // reactivates it knows it missed events.
  const wasInactiveRef = useRef(false);

  // Revalidation on re-activation is independent of the stream: every opened
  // case tab stays mounted (hidden), and window-focus refetch is off app-wide,
  // so without this a backgrounded tab shows stale data when the stream is
  // disabled or unconfigured. Runs whether or not a stream exists.
  useEffect(() => {
    if (!caseId) return;
    if (!active) {
      wasInactiveRef.current = true;
      return;
    }
    if (wasInactiveRef.current) {
      wasInactiveRef.current = false;
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASE_DETAIL, caseId] });
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, caseId] });
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASE_ACTIVITIES, caseId] });
    }
  }, [caseId, active, queryClient]);

  useEffect(() => {
    if (!caseId || !apiConfig.streamEnabled || !apiConfig.streamUrl) return;
    if (!active) return;

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
        ({ token, idToken } = await getTokens());
      } catch (error) {
        logger.debug(
          "[case-activity-stream] failed to get tokens",
          error instanceof Error ? error.message : "Unknown token error",
        );
      }
      if (cancelled) return;
      if (!token || !idToken) {
        scheduleReconnect();
        return;
      }

      const url = `${apiConfig.streamUrl}/cases/${encodeURIComponent(caseId)}/activities/stream`;
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
        void queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, caseId],
        });
        void queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CSM_CASE_ACTIVITIES, caseId],
        });
      });

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
  }, [caseId, active, queryClient, getTokens, logger]);
}
