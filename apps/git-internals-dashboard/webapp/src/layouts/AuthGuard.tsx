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

// Route wrapper that redirects unauthenticated users to sign-in. This app
// has no entitlement gate — any signed-in org user is authorized — so there
// is no CurrentUserProvider/"/users/me" check and no silent-sign-in retry
// loop. It does keep a sign-out latch (below) to absorb a transient
// `isSignedIn` flip; since this app has no sign-out action of its own, a
// real sign-out and that transient flip are handled by the same
// grace-period logic (REAUTH_GRACE_MS) rather than tracked separately.
import { type JSX, type ReactNode, useEffect, useRef, useState } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { ProtectedRoute } from "@asgardeo/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { Box, CircularProgress } from "@mui/material";
import { setAccessTokenGetter, setUnauthorizedHandler } from "@api/client";
import AppShell from "./AppShell";

// How long a false `isSignedIn` must persist before it's treated as a real
// sign-out rather than AsgardeoProvider's known transient flip (see
// hasSignedInOnce below) — long enough to absorb that flip, short enough
// that a genuinely lost session doesn't keep rendering stale, unprotected
// UI (and this session's cached query data) for long.
const REAUTH_GRACE_MS = 2000;

/** A full-viewport centered loading spinner. */
function CenteredSpinner(): JSX.Element {
  return (
    <Box sx={{ display: "flex", minHeight: "100vh", alignItems: "center", justifyContent: "center" }}>
      <CircularProgress size={32} sx={{ color: "var(--sla-primary)" }} />
    </Box>
  );
}

// Starts sign-in for a signed-out visitor and renders a loader while the
// browser navigates to the IdP. The ref guard keeps a re-render (or
// StrictMode's double-invoked effect) from firing a second authorize
// request; a failure to even start signing in is rethrown from render so
// the app's error boundary can show it, rather than an unhandled rejection.
function SignInRedirect(): JSX.Element {
  const { signIn } = useAsgardeo();
  const started = useRef(false);
  const [fatal, setFatal] = useState<Error | null>(null);
  if (fatal) throw fatal;

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    void signIn().catch((error: unknown) => {
      setFatal(error instanceof Error ? error : new Error(String(error)));
    });
  }, [signIn]);

  return <CenteredSpinner />;
}

// Wires the plain-fetch API client (@api/client) to this session's token and
// to `signIn()` on a 401, for as long as the tree beneath it is mounted —
// i.e. only once ProtectedRoute has confirmed the caller is signed in.
// Unwires on unmount so a stale getter/handler from a previous session
// can never leak into a later one.
//
// The wiring itself happens directly in the render body, not only in a
// useEffect: `children` (AppShell and its data-fetching descendants —
// useOverview, useSyncStatus, useTaxonomy, ...) renders right after this
// component returns, in the same commit, and their useQuery hooks fetch
// from *their own* mount effects. Effects fire child-first, parent-last
// within a commit, so wiring only in this component's effect would still
// run after those children's effects — api/client's getAccessToken() would
// see no tokenGetter yet and send their first request with no Authorization
// header, a guaranteed 401 on every fresh sign-in. Wiring during render
// instead guarantees it's set before React ever descends into `children`.
// The setters are plain idempotent module-level assignments (no React
// state), so re-running them on every render — including React's
// StrictMode double-render — is harmless.
//
// The effect below re-registers the same two setters (not just cleanup):
// StrictMode's dev-only mount replay runs this effect's cleanup and then
// re-invokes its setup with no render in between, so a cleanup-only effect
// would leave both callbacks null until something else happens to
// re-render this component. Re-registering in the setup closes that
// window; the render-time assignment above still covers everything else.
function AuthBridge({ children }: { children: ReactNode }): JSX.Element {
  const { getAccessToken, signIn } = useAsgardeo();

  setAccessTokenGetter(() => getAccessToken());
  setUnauthorizedHandler(() => {
    void signIn();
  });

  useEffect(() => {
    setAccessTokenGetter(() => getAccessToken());
    setUnauthorizedHandler(() => {
      void signIn();
    });
    return () => {
      setAccessTokenGetter(null);
      setUnauthorizedHandler(null);
    };
  }, [getAccessToken, signIn]);

  return <>{children}</>;
}

/** Redirects a signed-out visitor to sign-in; renders AppShell once signed in. */
export default function AuthGuard(): JSX.Element {
  const { isSignedIn } = useAsgardeo();
  const queryClient = useQueryClient();

  // Latches true the first time `isSignedIn` is observed true. Set directly
  // in the render body (React's documented "adjusting state during
  // rendering" pattern, not an effect) so the very same render that first
  // sees `isSignedIn` also switches branches, instead of committing one
  // extra render through `ProtectedRoute` first.
  //
  // Without this, `ProtectedRoute` swaps to `fallback` (unmounting
  // `AuthBridge`/`AppShell`) for as long as `isSignedIn` is false, however
  // briefly and however recoverable — a full React-level unmount that
  // remounts `SignInRedirect` fresh, re-arming its `started` guard and
  // firing `signIn()` again. `AsgardeoProvider` is known to flip
  // `isSignedIn` to false transiently, as part of its own internal
  // bookkeeping and independent of any real sign-out. As reproduced against
  // this app's own Asgardeo tenant, that flip can recur continuously: each
  // remount of `SignInRedirect` re-triggers `AsgardeoProvider`'s
  // post-sign-in bookkeeping, which flips `isSignedIn` again — turning a
  // single transient flip into an unbounded remount/sign-in loop.
  //
  // The latch is NOT permanent, though: a `false` reading that survives
  // REAUTH_GRACE_MS (below) is treated as a real sign-out, not the transient
  // bookkeeping flip — otherwise a stale/expired session could keep
  // rendering `AppShell`, with a later session's sign-in doing nothing to
  // reset it and this session's cached query data still sitting in the
  // shared QueryClient for that later session to see.
  const [hasSignedInOnce, setHasSignedInOnce] = useState(false);
  if (isSignedIn && !hasSignedInOnce) {
    setHasSignedInOnce(true);
  }

  useEffect(() => {
    if (isSignedIn || !hasSignedInOnce) return;
    // isSignedIn just went false while still latched in. If it flips back
    // true before REAUTH_GRACE_MS elapses, this cleanup cancels the timer
    // and nothing happens (the transient case above). Otherwise, this is a
    // real sign-out: drop any cached data from this session before
    // resetting the latch, so ProtectedRoute takes over again and a
    // subsequent sign-in never renders through stale cache.
    const timer = window.setTimeout(() => {
      queryClient.clear();
      setHasSignedInOnce(false);
    }, REAUTH_GRACE_MS);
    return () => window.clearTimeout(timer);
  }, [isSignedIn, hasSignedInOnce, queryClient]);

  if (hasSignedInOnce) {
    return (
      <AuthBridge>
        <AppShell />
      </AuthBridge>
    );
  }

  return (
    <ProtectedRoute loader={<CenteredSpinner />} fallback={<SignInRedirect />}>
      <AuthBridge>
        <AppShell />
      </AuthBridge>
    </ProtectedRoute>
  );
}
