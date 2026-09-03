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

import "@testing-library/jest-dom/vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { lazy, type JSX } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

// `ProtectedRoute`'s real implementation polls `isSignedIn` on ~1s interval
// and only calls `onSignIn` once it decides the user isn't authenticated —
// that's exactly the race under test here (see AuthGuard.tsx), but it's not
// something worth re-implementing under jsdom. Stub it down to "call
// onSignIn immediately with a fake defaultSignIn, so the assertions below
// are about AuthGuard's own onSignIn handler, not ProtectedRoute internals.
// It also renders a marker so tests can assert whether ProtectedRoute (and
// therefore its loader-swap behaviour) is even in the tree for a given render.
const defaultSignInMock = vi.fn();
const protectedRouteRenderCount = vi.fn();
// Captures whichever `loader` element AuthGuard passed on the most recent
// render — the mock below never renders `loader` itself (see its own
// comment), so a test that cares about `bare`'s loader-swap has to inspect
// this directly instead of looking for it in the rendered output.
let capturedLoader: React.ReactNode;
vi.mock("@asgardeo/react-router", () => ({
  ProtectedRoute: ({
    loader,
    onSignIn,
    children,
  }: {
    loader?: React.ReactNode;
    onSignIn?: (
      defaultSignIn: (options?: Record<string, unknown>) => void,
      signInOptions?: Record<string, unknown>,
    ) => void;
    children?: React.ReactNode;
  }) => {
    protectedRouteRenderCount();
    capturedLoader = loader;
    onSignIn?.(defaultSignInMock, { some: "option" });
    return children ?? null;
  },
}));

const signInSilentlyMock = vi.fn();
const signInMock = vi.fn();
// Mutable so individual tests can flip `isSignedIn` across rerenders to
// simulate a token expiring mid-session, after an initial successful sign-in.
const asgardeoState = { isSignedIn: false };
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: asgardeoState.isSignedIn,
    signIn: signInMock,
    signInSilently: signInSilentlyMock,
  }),
}));

vi.mock("@layouts/AppLayout", () => ({
  default: () => <div data-testid="app-layout" />,
}));

vi.mock("@layouts/BareAuthLoader", () => ({
  default: () => <div data-testid="bare-auth-loader" />,
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  CurrentUserProvider: ({ children }: { children: React.ReactNode }) => children,
}));

const loggerDebugMock = vi.fn();
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: loggerDebugMock, error: vi.fn(), warn: vi.fn(), info: vi.fn() }),
}));

const { default: AuthGuard } = await import("./AuthGuard");
const { default: BareAuthLoader } = await import("./BareAuthLoader");

function renderAuthGuard() {
  return render(
    <MemoryRouter initialEntries={["/some/protected/path"]}>
      <AuthGuard />
    </MemoryRouter>,
  );
}

describe("AuthGuard onSignIn (before any successful sign-in)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    asgardeoState.isSignedIn = false;
  });

  it("attempts a silent re-auth first and does not force a full sign-in redirect when it succeeds", async () => {
    signInSilentlyMock.mockResolvedValue(true);

    await act(async () => {
      renderAuthGuard();
    });

    await waitFor(() => expect(signInSilentlyMock).toHaveBeenCalledTimes(1));
    expect(defaultSignInMock).not.toHaveBeenCalled();
  });

  it("falls back to the full sign-in redirect when silent re-auth resolves falsy", async () => {
    signInSilentlyMock.mockResolvedValue(false);

    await act(async () => {
      renderAuthGuard();
    });

    await waitFor(() =>
      expect(defaultSignInMock).toHaveBeenCalledWith({ some: "option" }),
    );
  });

  it("falls back to the full sign-in redirect when silent re-auth rejects", async () => {
    signInSilentlyMock.mockRejectedValue(new Error("iframe blocked"));

    await act(async () => {
      renderAuthGuard();
    });

    await waitFor(() =>
      expect(defaultSignInMock).toHaveBeenCalledWith({ some: "option" }),
    );
    expect(loggerDebugMock).toHaveBeenCalledWith(
      "[auth] silent sign-in failed",
      "iframe blocked",
    );
  });
});

describe("AuthGuard after an initial successful sign-in (transient token-clock expiry)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    asgardeoState.isSignedIn = false;
  });

  it("stops rendering ProtectedRoute (and therefore its loader-swap) once signed in, and never re-enters it for a later transient clock expiry", async () => {
    asgardeoState.isSignedIn = true;
    let rerender!: ReturnType<typeof renderAuthGuard>["rerender"];

    await act(async () => {
      ({ rerender } = renderAuthGuard());
    });
    // Re-render to let the render-time `setHasSignedInOnce(true)` commit.
    await act(async () => {
      rerender(
        <MemoryRouter initialEntries={["/some/protected/path"]}>
          <AuthGuard />
        </MemoryRouter>,
      );
    });
    expect(protectedRouteRenderCount).not.toHaveBeenCalled();

    // Token's local clock expires — a transient drop, not a real sign-out.
    // AuthGuard must NOT proactively react to this itself (no background
    // signInSilently() poll here — see AuthGuard.tsx's comment on why an
    // earlier version's poller raced useAuthApiClient's own call-level
    // recovery and left an in-flight mutation stuck). Recovery for this case
    // is useAuthApiClient's job alone, exercised on the next real API call,
    // not AuthGuard's.
    asgardeoState.isSignedIn = false;
    await act(async () => {
      rerender(
        <MemoryRouter initialEntries={["/some/protected/path"]}>
          <AuthGuard />
        </MemoryRouter>,
      );
    });

    expect(protectedRouteRenderCount).not.toHaveBeenCalled();
    expect(signInSilentlyMock).not.toHaveBeenCalled();
    expect(signInMock).not.toHaveBeenCalled();
    expect(defaultSignInMock).not.toHaveBeenCalled();
  });

  it("resets the latch on an explicit sign-out (app:signing-out), falling back to ProtectedRoute instead of continuing to render stale protected content", async () => {
    asgardeoState.isSignedIn = true;
    let rerender!: ReturnType<typeof renderAuthGuard>["rerender"];

    await act(async () => {
      ({ rerender } = renderAuthGuard());
    });
    await act(async () => {
      rerender(
        <MemoryRouter initialEntries={["/some/protected/path"]}>
          <AuthGuard />
        </MemoryRouter>,
      );
    });
    expect(protectedRouteRenderCount).not.toHaveBeenCalled();

    // The user clicks "Sign out" — this app's existing signal fires
    // immediately before the SDK's signOut() call (see UserProfile.tsx /
    // IdleTimeoutProvider.tsx), before isSignedIn has necessarily flipped.
    await act(async () => {
      window.dispatchEvent(new CustomEvent("app:signing-out"));
    });

    // isSignedIn drops (the SDK's own state update, ahead of its redirect
    // actually navigating away).
    asgardeoState.isSignedIn = false;
    await act(async () => {
      rerender(
        <MemoryRouter initialEntries={["/some/protected/path"]}>
          <AuthGuard />
        </MemoryRouter>,
      );
    });

    // Falls back to ProtectedRoute's neutral loader — not stale protected
    // content — for the brief window before the browser actually navigates
    // away to the IdP's sign-out endpoint.
    expect(protectedRouteRenderCount).toHaveBeenCalled();
  });
});

describe("AuthGuard bare mode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    asgardeoState.isSignedIn = false;
  });

  function renderBareAuthGuard() {
    return render(
      <MemoryRouter initialEntries={["/cs-monitor-dashboard"]}>
        <Routes>
          <Route element={<AuthGuard bare />}>
            <Route
              path="cs-monitor-dashboard"
              element={<div data-testid="bare-route-content" />}
            />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
  }

  it("passes BareAuthLoader (not AppLayout) as ProtectedRoute's own loader before any sign-in", async () => {
    signInSilentlyMock.mockResolvedValue(true);

    await act(async () => {
      renderBareAuthGuard();
    });

    // The mock ProtectedRoute above never renders `loader` itself (only
    // `children`), so this checks the element type captured from the prop
    // directly rather than looking for it in the DOM.
    expect((capturedLoader as { type?: unknown } | undefined)?.type).toBe(BareAuthLoader);
  });

  it("renders a plain Outlet (the matched route's own element, not AppLayout) once signed in", async () => {
    asgardeoState.isSignedIn = true;
    let rerender!: ReturnType<typeof renderBareAuthGuard>["rerender"];

    await act(async () => {
      ({ rerender } = renderBareAuthGuard());
    });
    // Re-render to let the render-time `setHasSignedInOnce(true)` commit —
    // same pattern the non-bare "after an initial successful sign-in"
    // tests above use.
    await act(async () => {
      rerender(
        <MemoryRouter initialEntries={["/cs-monitor-dashboard"]}>
          <Routes>
            <Route element={<AuthGuard bare />}>
              <Route
                path="cs-monitor-dashboard"
                element={<div data-testid="bare-route-content" />}
              />
            </Route>
          </Routes>
        </MemoryRouter>,
      );
    });

    expect(screen.getByTestId("bare-route-content")).toBeInTheDocument();
    expect(screen.queryByTestId("app-layout")).not.toBeInTheDocument();
  });

  // Regression test: `bare` mode skips `AppLayout`, which is the ONLY
  // place elsewhere in the app that provides a `Suspense` boundary around
  // routed content (see App.tsx's own routing doc comment). Every route
  // page in App.tsx — including CsMonitorDashboardPage — is
  // `React.lazy`-loaded, so a `bare` route with no `Suspense` of its own
  // would have nothing to suspend against. This uses a REAL lazy import
  // (an artificially delayed one, so it actually suspends instead of
  // resolving synchronously) rather than a plain element, specifically to
  // exercise that gap.
  it("renders its own Suspense fallback (not a crash) while a lazy-loaded route element is still loading, once signed in", async () => {
    asgardeoState.isSignedIn = true;
    const LazyRouteContent = lazy(
      () =>
        new Promise<{ default: () => JSX.Element }>((resolve) => {
          setTimeout(
            () => resolve({ default: () => <div data-testid="lazy-loaded-content" /> }),
            10,
          );
        }),
    );
    let rerender!: ReturnType<typeof renderBareAuthGuard>["rerender"];

    await act(async () => {
      ({ rerender } = renderBareAuthGuard());
    });
    await act(async () => {
      rerender(
        <MemoryRouter initialEntries={["/cs-monitor-dashboard"]}>
          <Routes>
            <Route element={<AuthGuard bare />}>
              <Route path="cs-monitor-dashboard" element={<LazyRouteContent />} />
            </Route>
          </Routes>
        </MemoryRouter>,
      );
    });

    // Immediately after the lazy element is first rendered, it's still
    // suspended — AuthGuard's own Suspense fallback (BareAuthLoader) is
    // what should show, not a blank tree or a thrown error.
    expect(screen.getByTestId("bare-auth-loader")).toBeInTheDocument();
    expect(screen.queryByTestId("lazy-loaded-content")).not.toBeInTheDocument();

    // Once the lazy import resolves, the real content takes over.
    await waitFor(() => expect(screen.getByTestId("lazy-loaded-content")).toBeInTheDocument());
  });
});
