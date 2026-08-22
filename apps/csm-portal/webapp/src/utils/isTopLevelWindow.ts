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

/**
 * Whether this document is the SDK's own hidden silent-auth iframe rather than
 * the real page.
 *
 * `signInSilently()` recovers an expired token by loading the IdP's authorize
 * URL with `prompt=none` inside a hidden iframe. Because this app is served at
 * the same redirect_uri as the top-level app, that iframe's document is a full
 * second load of this same bundle, sharing this origin's `sessionStorage`. Two
 * things must therefore be gated on this:
 *
 * - Nothing below `AsgardeoProvider` should mount there (`AppWithConfig`), or
 *   the router and `AuthGuard` mount too, notice "not signed in" inside the
 *   frame, and start their own silent sign-in — nesting another frame inside
 *   this one. Observed live: one token expiry cascading into 7 nested loads,
 *   dropping an in-flight POST.
 * - Nothing may record the entry deep link (`main.tsx`) or start a sign-in
 *   redirect (`AuthGuard`), since the frame's URL is not a user navigation and
 *   `sessionStorage` is shared with the page that does have one.
 *
 * This app is never legitimately embedded by anything else, so being framed
 * unambiguously means "I am the SDK's hidden auth iframe", not a real embedding
 * to support.
 *
 * @returns {boolean} `true` for the real top-level page, `false` inside a frame.
 */
export function isTopLevelWindow(): boolean {
  // Comparing WindowProxy references is allowed cross-origin and never throws;
  // only reaching into the other window's properties would.
  return typeof window === "undefined" || window.self === window.top;
}

/**
 * Whether this silent-auth frame can still complete the SDK's handshake.
 *
 * The SDK detects its silent-sign-in state in the frame's URL and hands the
 * authorization code back to the opener, so `AsgardeoProvider` does have to
 * mount in a frame carrying `?code=`. A frame carrying `?error=` instead (the
 * IdP answering `prompt=none` with `login_required`, which is what every
 * browser that blocks cross-site cookies will produce) has nothing to hand
 * back. Mounting the provider there only lets its own start-up effect call
 * `signIn()` on a URL it then rejects, surfacing as an uncaught
 * `login_required` in the console on every signed-out visit.
 *
 * @returns {boolean} `true` if this frame still has a handshake to finish.
 */
export function silentAuthFrameCanComplete(): boolean {
  const params = new URLSearchParams(window.location.search);
  return params.has("code") || !params.has("error");
}
