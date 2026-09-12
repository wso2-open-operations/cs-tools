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

import { useCallback, useEffect, useRef } from "react";
import { useAsgardeo } from "@asgardeo/react";
import {
  ASGARDEO_UNAUTHENTICATED_CODE,
  AUTH_NOT_READY_ERROR_MESSAGE,
} from "@constants/apiConstants";
import { useLogger } from "@hooks/useLogger";
import { trySilentSignInOnce } from "@hooks/silentSignIn";

// Shared across every caller's hook instance. Each useAuthTokens() call
// creates its own closures, so this lives at module scope to ensure only ONE
// full sign-in redirect is triggered even when many concurrent callers
// (across different hook instances) discover a dead refresh token at once.
let signInInFlight = false;

// Only the Asgardeo "unauthenticated" code means the token was expired/missing
// when the call ran (e.g. the refresh token itself has expired, so the SDK's
// periodic background refresh can no longer mint a new access token). Anything
// else (network failures, real backend 5xx) must propagate untouched so
// existing error handling and error pages still work. Without this
// classification, a dead refresh token sends the SDK's periodic background
// refresh into an infinite loop of failing refresh-grant requests instead of
// bouncing the user to sign-in.
function isTokenExpiredError(error: unknown): boolean {
  return (
    error != null &&
    typeof error === "object" &&
    "code" in error &&
    (error as { code: string }).code === ASGARDEO_UNAUTHENTICATED_CODE
  );
}

/**
 * True when `getAccessToken()` failed because the Asgardeo SDK had not finished
 * initializing yet (code `SPA-AUTH_CLIENT-VM-NF01`, "The SDK must be
 * initialized first"). This is a transient race on first paint — the silent
 * refresh added in @asgardeo/react 0.25.5 can ask for a token a tick before the
 * SDK is ready — so callers should treat it as "auth not ready, retry", not a
 * hard error.
 */
function isSdkNotInitializedError(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  if ((error as { code?: string }).code === "SPA-AUTH_CLIENT-VM-NF01") {
    return true;
  }
  return /SDK (?:must be initialized|is not initialized)/i.test(
    `${error.name} ${error.message}`,
  );
}

export interface AuthTokens {
  token: string;
  idToken: string;
}

/**
 * Resolves a fresh access/ID token pair, recovering from a dead refresh
 * token the same way for every caller: retry once (a concurrent caller, or
 * the SDK's own periodic background refresh, may have already re-minted the
 * token), then a silent hidden-iframe re-auth if the IdP session is still
 * alive, then a full sign-in redirect as the last resort — never leaving a
 * caller to just retry forever against a refresh token that's actually
 * dead.
 *
 * useCaseActivityStream is this hook's consumer today: its EventSource headers
 * are fixed at construction time, so they cannot be refreshed through fetch's
 * own Authorization-header path. useAuthApiClient no longer uses this hook —
 * it resolves tokens itself — so the two coordinate through the shared
 * `trySilentSignInOnce` rather than by sharing this code. Anything else
 * needing raw tokens outside a fetch should come through here rather than
 * calling useAsgardeo() directly and reimplementing (or omitting) the
 * recovery dance.
 */
export function useAuthTokens() {
  const { getAccessToken, getIdToken, signIn, signInSilently } = useAsgardeo();
  const logger = useLogger();

  // getAccessToken/getIdToken are stable across AsgardeoProvider re-renders,
  // but signIn/signInSilently are not — the provider recreates them every
  // render. Reading them through a ref (updated on every render, but never
  // itself a dependency) keeps redirectToSignIn/trySilentSignIn — and
  // everything downstream that depends on this hook's returned callback,
  // e.g. useAuthApiClient's authFetch — stable too, instead of getting a new
  // identity on every provider re-render.
  const signInRef = useRef(signIn);
  const signInSilentlyRef = useRef(signInSilently);
  useEffect(() => {
    signInRef.current = signIn;
    signInSilentlyRef.current = signInSilently;
  }, [signIn, signInSilently]);

  // Redirect to a full sign-in, single-flighted so concurrent auth failures
  // don't fire multiple redirects. Returns a never-resolving promise so
  // callers don't fall through to treating this as a real failure while the
  // browser navigates away.
  const redirectToSignIn = useCallback((): Promise<AuthTokens> => {
    if (!signInInFlight) {
      signInInFlight = true;
      void Promise.resolve(signInRef.current()).finally(() => {
        signInInFlight = false;
      });
    }
    return new Promise<AuthTokens>(() => {});
  }, []);

  // Before giving up and bouncing the whole tab to a full sign-in redirect
  // (which discards any in-progress work — an open comment draft, an unsaved
  // dialog), try a silent, hidden-iframe re-authentication. If the user's IdP
  // session (SSO cookie) is still alive, this mints a fresh token without any
  // visible navigation; only a genuinely dead IdP session falls through to
  // `redirectToSignIn`.
  //
  // Single-flighted via the shared `trySilentSignInOnce`, NOT a guard local to
  // this module. A dead refresh token is typically noticed by several callers
  // at once — the SSE stream through this hook, and any REST fetch through
  // useAuthApiClient — and a module-local guard cannot see the other module's
  // attempt, so each would open its own hidden iframe. That is exactly the
  // uncoordinated double re-auth `silentSignIn.ts` exists to prevent, and it
  // can drop an in-flight request such as a comment POST.
  const trySilentSignIn = useCallback((): Promise<boolean> => {
    return trySilentSignInOnce(
      () => signInSilentlyRef.current(),
      (message) => logger.debug("[auth] silent sign-in failed", message),
    );
  }, [logger]);

  const attemptGetTokens = useCallback(async (): Promise<AuthTokens> => {
    let token: string | undefined;
    let idToken: string | undefined;
    try {
      [token, idToken] = await Promise.all([getAccessToken(), getIdToken()]);
    } catch (error) {
      // Normalise the SDK-not-initialized race into the shared "auth not
      // ready" signal so callers warn-and-retry instead of surfacing a raw
      // AsgardeoAuthException as a hard error.
      if (isSdkNotInitializedError(error)) {
        throw new Error(AUTH_NOT_READY_ERROR_MESSAGE);
      }
      throw error;
    }
    if (!token) {
      throw new Error("Unable to retrieve access token");
    }
    if (!idToken) {
      throw new Error("Unable to retrieve ID token");
    }
    return { token, idToken };
  }, [getAccessToken, getIdToken]);

  return useCallback(async (): Promise<AuthTokens> => {
    try {
      return await attemptGetTokens();
    } catch (error) {
      // Only an expired/missing token is recoverable here; anything else
      // (auth-not-ready, unexpected errors) must surface to the caller.
      if (!isTokenExpiredError(error)) {
        throw error;
      }

      // A concurrent caller, or the provider's periodic background refresh,
      // may have re-minted the token in the meantime, so retry once to pick
      // it up. If nothing refreshed it the retry fails again and we fall
      // through to the sign-in redirect below.
      try {
        return await attemptGetTokens();
      } catch (retryError) {
        if (!isTokenExpiredError(retryError)) {
          throw retryError;
        }

        // Still unauthenticated after the retry — the refresh token is
        // dead. Try a silent re-auth first: if the IdP session is still
        // alive this mints a fresh token with no visible navigation, so
        // in-progress work survives.
        if (await trySilentSignIn()) {
          try {
            return await attemptGetTokens();
          } catch (afterSilentSignInError) {
            if (!isTokenExpiredError(afterSilentSignInError)) {
              throw afterSilentSignInError;
            }
            // Silent sign-in reported success but the token still won't
            // authenticate (e.g. a race with a session that expired a
            // moment later) — fall through to the hard redirect below.
          }
        }

        // Silent re-auth was unavailable or the IdP session itself is
        // gone. Redirect for a full sign-in instead of letting the caller
        // keep retrying forever against a refresh token that's dead.
        return redirectToSignIn();
      }
    }
  }, [attemptGetTokens, redirectToSignIn, trySilentSignIn]);
}
