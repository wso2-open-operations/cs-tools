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
 * Whether this document is the top-level page rather than a frame.
 *
 * The IdP SDK runs its silent re-auth in a hidden iframe whose redirect_uri is
 * this app's own origin, so that iframe boots the whole app a second time —
 * sharing sessionStorage with the real page. Anything that assumes "this
 * document is what the user navigated to" has to be gated on this: recording
 * the entry deep link (main.tsx) and starting a sign-in redirect (AuthGuard)
 * are both meaningless, and actively harmful, from inside that frame.
 *
 * @returns {boolean} `true` for the top-level page, `false` inside a frame.
 */
export function isTopLevelWindow(): boolean {
  try {
    return window.top === window.self;
  } catch {
    // Cross-origin parent: not our own silent-auth frame, but not a page the
    // user deep-linked to either. Treat it as framed.
    return false;
  }
}
