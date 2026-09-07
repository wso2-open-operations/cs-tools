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

// Route wrapper that redirects unauthenticated users to sign-in. Simplified
// from csm-portal's AuthGuard.tsx (SPEC §11): this app has no entitlement
// gate — any signed-in org user is authorized (D7) — so there is no
// CurrentUserProvider/"/users/me" check and no silent-sign-in retry loop.
// It does keep csm-portal's sign-out latch (below), minus that file's
// separate "explicit sign-out" tracking — this app has no sign-out action
// to distinguish from a transient `isSignedIn` flip.
import { type JSX, type ReactNode, useEffect, useRef, useState } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { ProtectedRoute } from "@asgardeo/react-router";
import { Box, CircularProgress } from "@mui/material";
import { setAccessTokenGetter, setUnauthorizedHandler } from "@api/client";
import AppShell from "./AppShell";

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
function AuthBridge({ children }: { children: ReactNode }): JSX.Element {
  const { getAccessToken, signIn } = useAsgardeo();

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

export default function AuthGuard(): JSX.Element {
  const { isSignedIn } = useAsgardeo();

  // Latches true the first time `isSignedIn` is observed true, and never
  // resets. Set directly in the render body (React's documented "adjusting
  // state during rendering" pattern, not an effect) so the very same render
  // that first sees `isSignedIn` also switches branches, instead of
  // committing one extra render through `ProtectedRoute` first.
  //
  // Without this, `ProtectedRoute` swaps to `fallback` (unmounting
  // `AuthBridge`/`AppShell`) for as long as `isSignedIn` is false, however
  // briefly and however recoverable — a full React-level unmount that
  // remounts `SignInRedirect` fresh, re-arming its `started` guard and
  // firing `signIn()` again. Ported from csm-portal's AuthGuard.tsx, which
  // hit this live: a transient `isSignedIn` flip (there, a token-clock
  // check) destroyed in-progress work on every occurrence, and — as
  // reproduced against this app's own Asgardeo tenant — the flip can recur
  // continuously, since `AsgardeoProvider`'s post-sign-in bookkeeping
  // re-fires from the remounted `SignInRedirect` every time, turning a
  // single flip into an unbounded loop.
  const [hasSignedInOnce, setHasSignedInOnce] = useState(false);
  if (isSignedIn && !hasSignedInOnce) {
    setHasSignedInOnce(true);
  }

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
