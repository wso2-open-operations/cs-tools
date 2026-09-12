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
import { ApiError } from "@utils/ApiError";

// Mutable so individual tests can flip `isSignedIn` across rerenders to
// simulate a token expiring mid-session, after an initial successful sign-in.
// Declared above the `vi.mock` factories below, which close over it.
const asgardeoState = { isSignedIn: false };

// Stubbed down to the branch order of the real component: children once
// signed in, otherwise the `fallback` element. Re-implementing its polling
// under jsdom isn't worth it, and the assertions below are about AuthGuard's
// own sign-in handling rather than ProtectedRoute internals.
//
// Because this stub never throws, it cannot catch the upstream
// `@asgardeo/react-router` 2.0.0 fall-through that crashed every signed-out
// visit — AuthGuard.protectedRoute.test.tsx runs the real component for that.
// It also renders a marker so tests can assert whether ProtectedRoute (and
// therefore its loader-swap behaviour) is even in the tree for a given render.
const protectedRouteRenderCount = vi.fn();
vi.mock("@asgardeo/react-router", () => ({
  ProtectedRoute: ({
    children,
    fallback,
    loader,
  }: {
    children?: React.ReactNode;
    fallback?: React.ReactNode;
    loader?: React.ReactNode;
  }) => {
    protectedRouteRenderCount();
    if (asgardeoState.isSignedIn) return children ?? null;
    return fallback ?? loader ?? null;
  },
}));

const signInSilentlyMock = vi.fn();
const signInMock = vi.fn();
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: asgardeoState.isSignedIn,
    signIn: signInMock,
    signInSilently: signInSilentlyMock,
  }),
}));

const appLayoutPropsMock = vi.fn();
vi.mock("@layouts/AppLayout", () => ({
  default: ({
    children,
    minimalHeader = false,
    showCaseTabs = true,
  }: {
    children?: React.ReactNode;
    minimalHeader?: boolean;
    showCaseTabs?: boolean;
  }) => {
    appLayoutPropsMock({ minimalHeader, showCaseTabs });
    return <>{children}</>;
  },
}));

vi.mock("@layouts/BareAuthLoader", () => ({
  default: () => <div data-testid="bare-auth-loader" />,
}));

// Mutable so individual tests can simulate a /users/me outcome. Defaults to
// "loaded fine, no error" — the common case for every pre-existing test in
// this file, which don't care about CurrentUserContext at all.
const currentUserState: { isLoading: boolean; isError: boolean; error: Error | null } = {
  isLoading: false,
  isError: false,
  error: null,
};

vi.mock("@context/current-user/CurrentUserContext", () => ({
  CurrentUserProvider: ({ children }: { children: React.ReactNode }) => children,
  useCurrentUser: () => ({
    user: undefined,
    isLoading: currentUserState.isLoading,
    isError: currentUserState.isError,
    error: currentUserState.error,
  }),
}));

const loggerDebugMock = vi.fn();
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: loggerDebugMock, error: vi.fn(), warn: vi.fn(), info: vi.fn() }),
}));

const { default: AuthGuard } = await import("./AuthGuard");

function renderAuthGuard() {
  return render(
    <MemoryRouter initialEntries={["/some/protected/path"]}>
      <AuthGuard />
    </MemoryRouter>,
  );
}

describe("AuthGuard sign-in fallback (before any successful sign-in)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    asgardeoState.isSignedIn = false;
    currentUserState.isLoading = false;
    currentUserState.isError = false;
    currentUserState.error = null;
    signInMock.mockResolvedValue(undefined);
  });

  it("attempts a silent re-auth first and does not force a full sign-in redirect when it succeeds", async () => {
    signInSilentlyMock.mockResolvedValue(true);

    await act(async () => {
      renderAuthGuard();
    });

    await waitFor(() => expect(signInSilentlyMock).toHaveBeenCalledTimes(1));
    expect(signInMock).not.toHaveBeenCalled();
  });

  it("falls back to the full sign-in redirect when silent re-auth resolves falsy", async () => {
    signInSilentlyMock.mockResolvedValue(false);

    await act(async () => {
      renderAuthGuard();
    });

    await waitFor(() => expect(signInMock).toHaveBeenCalledTimes(1));
  });

  it("falls back to the full sign-in redirect when silent re-auth rejects", async () => {
    signInSilentlyMock.mockRejectedValue(new Error("iframe blocked"));

    await act(async () => {
      renderAuthGuard();
    });

    await waitFor(() => expect(signInMock).toHaveBeenCalledTimes(1));
    expect(loggerDebugMock).toHaveBeenCalledWith(
      "[auth] silent sign-in failed",
      "iframe blocked",
    );
  });

  it("saves the intended deep link before redirecting to the IdP", async () => {
    signInSilentlyMock.mockResolvedValue(false);

    await act(async () => {
      renderAuthGuard();
    });

    await waitFor(() => expect(signInMock).toHaveBeenCalledTimes(1));
    expect(sessionStorage.getItem("post_login_redirect")).toBe(
      "/some/protected/path",
    );
  });
});

describe("AuthGuard after an initial successful sign-in (transient token-clock expiry)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    asgardeoState.isSignedIn = false;
    currentUserState.isLoading = false;
    currentUserState.isError = false;
    currentUserState.error = null;
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

describe("AuthGuard's response to a /users/me failure once signed in", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    asgardeoState.isSignedIn = true;
    currentUserState.isLoading = false;
    currentUserState.isError = false;
    currentUserState.error = null;
  });

  it("shows the not-authorized page when /users/me fails with 401 (a token useAuthApiClient's own recovery chain could not fix)", async () => {
    currentUserState.isError = true;
    currentUserState.error = new ApiError(401, "Unauthorized");
    let rerender!: ReturnType<typeof renderAuthGuard>["rerender"];

    await act(async () => {
      ({ rerender } = renderAuthGuard());
    });
    // Re-render to let the render-time `setHasSignedInOnce(true)` commit —
    // see the equivalent comment above on the pre-existing sign-in tests.
    await act(async () => {
      rerender(
        <MemoryRouter initialEntries={["/some/protected/path"]}>
          <AuthGuard />
        </MemoryRouter>,
      );
    });

    expect(
      screen.getByText("You don't have access to this portal yet"),
    ).toBeInTheDocument();
    // minimalHeader (which also suppresses the sidebar in the real AppLayout)
    // must be true on every call, including the very first — it's derived
    // synchronously from the same render as `children`, not settled a render
    // later via an effect, so the sidebar never flashes on screen first.
    for (const call of appLayoutPropsMock.mock.calls) {
      expect(call[0]).toEqual({ minimalHeader: true, showCaseTabs: true });
    }
    expect(appLayoutPropsMock).toHaveBeenCalled();
  });

  it("shows the not-authorized page when /users/me fails with 403", async () => {
    currentUserState.isError = true;
    currentUserState.error = new ApiError(403, "Forbidden");
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

    expect(
      screen.getByText("You don't have access to this portal yet"),
    ).toBeInTheDocument();
  });

  it("suppresses the routed page (shows a loading state instead) while /users/me is still loading, rather than mounting it before access is confirmed", async () => {
    currentUserState.isLoading = true;
    currentUserState.isError = false;
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

    expect(
      screen.queryByText("You don't have access to this portal yet"),
    ).not.toBeInTheDocument();
    // minimalHeader true here too: the routed page (and its sidebar) has no
    // business being reachable before we know this caller is authorized.
    // showCaseTabs stays true (the default): CurrentUserProvider is still an
    // ancestor here, just its data hasn't resolved yet, and useCurrentUser
    // only throws with no provider at all, not while it's loading.
    expect(appLayoutPropsMock).toHaveBeenCalledWith({
      minimalHeader: true,
      showCaseTabs: true,
    });
  });

  it("does not show the not-authorized page for an unrelated /users/me failure (e.g. 500)", async () => {
    currentUserState.isError = true;
    currentUserState.error = new ApiError(500, "Internal Server Error");
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

    expect(
      screen.queryByText("You don't have access to this portal yet"),
    ).not.toBeInTheDocument();
    expect(appLayoutPropsMock).toHaveBeenCalledWith({
      minimalHeader: false,
      showCaseTabs: true,
    });
  });
});

describe("AuthGuard's AppLayout showCaseTabs wiring while auth itself hasn't resolved", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    asgardeoState.isSignedIn = false;
    currentUserState.isLoading = false;
    currentUserState.isError = false;
    currentUserState.error = null;
    signInMock.mockResolvedValue(undefined);
  });

  it("passes showCaseTabs=false to AppLayout while signed out (no CurrentUserProvider ancestor), so a restored case tab can't crash on a cold reload", async () => {
    signInSilentlyMock.mockResolvedValue(false);

    await act(async () => {
      renderAuthGuard();
    });

    await waitFor(() => expect(signInMock).toHaveBeenCalledTimes(1));
    expect(appLayoutPropsMock).toHaveBeenCalled();
    for (const call of appLayoutPropsMock.mock.calls) {
      expect(call[0]).toMatchObject({ showCaseTabs: false });
    }
  });
});

describe("AuthGuard bare mode", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    asgardeoState.isSignedIn = false;
    currentUserState.isLoading = false;
    currentUserState.isError = false;
    currentUserState.error = null;
    signInMock.mockResolvedValue(undefined);
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

  it("shows BareAuthLoader (no AppLayout chrome) while still signed out", async () => {
    signInSilentlyMock.mockResolvedValue(true);

    await act(async () => {
      renderBareAuthGuard();
    });

    await waitFor(() => expect(signInSilentlyMock).toHaveBeenCalledTimes(1));
    // Signed out: ProtectedRoute renders its `fallback` (SignInRedirect),
    // which in `bare` mode holds a chrome-free BareAuthLoader - never
    // AppLayout - until the IdP round-trip lands.
    expect(screen.getByTestId("bare-auth-loader")).toBeInTheDocument();
    expect(screen.queryByTestId("app-layout")).not.toBeInTheDocument();
  });

  it("renders the matched route's own element (not AppLayout / AuthorizedAppShell) once signed in", async () => {
    asgardeoState.isSignedIn = true;
    let rerender!: ReturnType<typeof renderBareAuthGuard>["rerender"];

    await act(async () => {
      ({ rerender } = renderBareAuthGuard());
    });
    // Re-render to let the render-time `setHasSignedInOnce(true)` commit -
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

  // Regression test (rksk review): a `bare` kiosk route must never follow a
  // stale POST_LOGIN_REDIRECT_KEY left in sessionStorage by an abandoned
  // sign-in elsewhere in the same browser session — it stays put and drops
  // the key.
  it("ignores and clears a stale post-login redirect key instead of navigating away from the kiosk route", async () => {
    asgardeoState.isSignedIn = true;
    sessionStorage.setItem("post_login_redirect", "/cases/999");
    let rerender!: ReturnType<typeof renderBareAuthGuard>["rerender"];

    await act(async () => {
      ({ rerender } = renderBareAuthGuard());
    });
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
    expect(sessionStorage.getItem("post_login_redirect")).toBeNull();
  });

  // Regression test: `bare` mode skips `AppLayout`, the only place
  // elsewhere in the app that wraps routed content in a `Suspense`
  // boundary. A lazily-loaded `bare` route element would have nothing to
  // suspend against without AuthGuard providing its own. Uses a REAL
  // (artificially delayed) lazy import so it actually suspends.
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

    // Still suspended: AuthGuard's own Suspense fallback (BareAuthLoader)
    // shows, not a blank tree or a thrown error.
    expect(screen.getByTestId("bare-auth-loader")).toBeInTheDocument();
    expect(screen.queryByTestId("lazy-loaded-content")).not.toBeInTheDocument();

    // Once the lazy import resolves, the real content takes over.
    await waitFor(() => expect(screen.getByTestId("lazy-loaded-content")).toBeInTheDocument());
  });
});
