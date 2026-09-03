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

import { type JSX, Suspense, useEffect, useState } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { ProtectedRoute } from "@asgardeo/react-router";
import { Outlet, useLocation, useNavigate } from "react-router";
import AppLayout from "@layouts/AppLayout";
import BareAuthLoader from "@layouts/BareAuthLoader";
import { POST_LOGIN_REDIRECT_KEY } from "@layouts/postLoginRedirect";
import { CurrentUserProvider } from "@context/current-user/CurrentUserContext";
import { useLogger } from "@hooks/useLogger";
import { trySilentSignInOnce } from "@hooks/silentSignIn";

export interface AuthGuardProps {
  /** Skips `AppLayout` (header, sidebar, banners, idle-timeout provider)
   * once authenticated, rendering a bare `<Outlet />` instead — for a route
   * that needs real authentication but must show nothing else on screen
   * (e.g. `/cs-monitor-dashboard`, a full-screen kiosk-style view).
   * `false` (the default) is every other route's normal, chrome-wrapped
   * behavior. Deliberately a prop on THIS guard rather than a second,
   * parallel guard component — the sign-in latching/redirect-preservation
   * logic below is exactly the same either way; only what renders once
   * signed in differs. */
  bare?: boolean;
}

/**
 * AuthGuard renders AppLayout (header/footer) so loading state is visible
 * and the IdP authentication flow can be observed. Redirects to home only
 * when not signed in and auth check is complete.
 *
 * Preserves the intended URL across the IdP sign-in redirect so that
 * deep-links (e.g. ServiceNow case links) land on the correct page after auth.
 *
 * Note: the customer-portal behaviour of auto-redirecting `/` to the last
 * visited project's dashboard is intentionally NOT replicated here. CSM is
 * engineer-scoped, so the landing route `/` resolves to the ABT dashboard
 * via App.tsx instead.
 *
 * @returns {JSX.Element} AppLayout (or, in `bare` mode, a plain Outlet) or a
 * redirect to home.
 */
export default function AuthGuard({ bare = false }: AuthGuardProps): JSX.Element {
  const { isSignedIn, signInSilently } = useAsgardeo();
  const location = useLocation();
  const navigate = useNavigate();
  const logger = useLogger();

  // Latches true the first time `isSignedIn` is observed true, and never
  // resets — see the render branch below for why. Set directly in the render
  // body (React's documented "adjusting state during rendering" pattern, not
  // an effect) so the very same render that first sees `isSignedIn` also
  // switches branches, instead of committing one extra render through
  // `ProtectedRoute` first.
  const [hasSignedInOnce, setHasSignedInOnce] = useState(false);
  // One-way latch, separate from `hasSignedInOnce`: once an explicit
  // sign-out starts, the "bypass ProtectedRoute" branch below must stop
  // unconditionally, and stay stopped, regardless of what `isSignedIn` does
  // afterward. `app:signing-out` (this app's existing signal, dispatched by
  // every manual "Sign out" action in `UserProfile.tsx`/
  // `IdleTimeoutProvider.tsx`) fires *before* the SDK's `signOut()` call —
  // i.e. before `isSignedIn` has necessarily flipped to `false` yet. A
  // reset tied to `isSignedIn` directly would race the render-time
  // `hasSignedInOnce` latch below (still seeing `isSignedIn === true` on
  // the very next render) and get immediately re-latched back to `true` in
  // the same update; `isSigningOut` sidesteps that race entirely by not
  // depending on `isSignedIn` at all once it's set.
  const [isSigningOut, setIsSigningOut] = useState(false);
  if (isSignedIn && !hasSignedInOnce && !isSigningOut) {
    setHasSignedInOnce(true);
  }

  // Without this, an explicit sign-out was indistinguishable from a
  // transient token-clock expiry once `hasSignedInOnce` had latched true —
  // both just flip `isSignedIn` to `false` eventually — and this component
  // would keep `CurrentUserProvider`/`AppLayout` mounted with the
  // just-signed-out user's data during the brief window before the SDK's
  // `signOut()` redirect actually navigates away, instead of falling back
  // to `ProtectedRoute`'s neutral loader.
  useEffect(() => {
    const handleSigningOut = (): void => setIsSigningOut(true);
    window.addEventListener("app:signing-out", handleSigningOut);
    return () => window.removeEventListener("app:signing-out", handleSigningOut);
  }, []);

  // After login, restore the saved deep link so it survives the Asgardeo SDK
  // reloading the page to `afterSignInUrl` ("/") after the callback (which would
  // otherwise drop us on the default landing). The key is consumed by
  // PostLoginRedirectConsumer once we arrive at the target — that consumer runs
  // above <Routes> so it also clears the key for routes AuthGuard never mounts
  // (e.g. the 404 page); clearing here would strand the key on a dead deep link
  // and bounce the next `/` visit back to it. The default `/` landing is
  // deferred to RootLanding in App.tsx while a redirect is pending.
  useEffect(() => {
    if (!isSignedIn) return;
    const redirect = sessionStorage.getItem(POST_LOGIN_REDIRECT_KEY);
    if (!redirect) return;
    // Compare (and restore) the full location including the hash, so anchor
    // permalinks like `/cases/:id#description` are honoured, not stripped.
    const here = location.pathname + location.search + location.hash;
    if (here !== redirect) {
      void navigate(redirect, { replace: true });
    }
  }, [isSignedIn, navigate, location.pathname, location.search, location.hash]);

  // Once a session has been established at least once, never again let
  // `ProtectedRoute` hide the app behind its `loader` for a transient
  // client-side token-clock expiry. `ProtectedRoute` swaps to `loader`
  // (unmounting `children`) for as long as `isSignedIn` is false, however
  // briefly and however recoverable — that is a full React-level unmount of
  // everything under it, indistinguishable in effect from a page reload even
  // though the browser itself never navigates. Live reproduction confirmed
  // this destroys in-progress work (an open case-comment composer and its
  // draft) within the same instant the token's local clock expires, well
  // before any silent-reauth attempt could even begin — a plain in-memory
  // marker on `window` survived the whole cycle while the React tree's own
  // state did not.
  //
  // No background effect proactively re-runs `signInSilently()` here for
  // this case (an earlier version of this fix added one, driven by
  // `isSignedIn` transitions) — it raced `useAuthApiClient.ts`'s own
  // call-level recovery: the shared single-flight guard only dedupes
  // *concurrent* attempts, so a background poller and a real failing
  // request could still each trigger their own attempt back-to-back, and
  // that duplicate cycle was observed to leave an in-flight mutation's own
  // promise chain stuck (a case comment created successfully server-side,
  // but the composer's "Sending…" state never cleared). `useAuthApiClient`
  // already recovers correctly on any actual 401 from real use — once
  // signed in, that is the ONLY recovery trigger this app needs; a token
  // expiring while the user does nothing at all needs no proactive fix.
  // Computed once, used in both return branches below. In `bare` mode this
  // is a lazy-loaded page's route element with NO ancestor `Suspense`
  // anywhere else in the tree — `AppLayout` is the only place that
  // provides one (see its own `<Suspense fallback={<RouteSuspenseFallback
  // />}>`), and `bare` mode's whole point is skipping `AppLayout`. Without
  // this, a `React.lazy` page rendered here has nothing to suspend
  // against. Reusing `BareAuthLoader` as the fallback here too — the same
  // "nothing but a centered progress bar" moment either way, whether the
  // wait is for auth or for the page's own chunk to download.
  const content = bare ? (
    <Suspense fallback={<BareAuthLoader />}>
      <Outlet />
    </Suspense>
  ) : (
    <AppLayout />
  );

  if (hasSignedInOnce && !isSigningOut) {
    return <CurrentUserProvider>{content}</CurrentUserProvider>;
  }

  return (
    <ProtectedRoute
      loader={bare ? <BareAuthLoader /> : <AppLayout />}
      onSignIn={(defaultSignIn, signInOptions) => {
        const intended =
          location.pathname + location.search + location.hash;
        if (intended !== "/") {
          sessionStorage.setItem(POST_LOGIN_REDIRECT_KEY, intended);
        }
        trySilentSignInOnce(signInSilently, (message) =>
          logger.debug("[auth] silent sign-in failed", message),
        ).then((result) => {
          if (!result) {
            defaultSignIn(signInOptions);
          }
        });
      }}
    >
      <CurrentUserProvider>{content}</CurrentUserProvider>
    </ProtectedRoute>
  );
}
