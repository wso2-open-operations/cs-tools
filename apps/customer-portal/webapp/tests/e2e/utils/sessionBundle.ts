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

//
// Replaying the captured session by hand, for the few specs that must not use
// `withSession`.
//
// Those specs are the ones that BREAK a session on purpose — expiring it, or
// signing out of it. They need to control exactly what gets restored, which the
// fixture deliberately does not expose.
//

import fs from "node:fs";
import { sessionPath, type BrowserContext } from "../fixtures/test";

/** The parts of a captured bundle these specs read or rewrite. */
export interface SessionBundle {
  origin?: string;
  identity?: string;
  localStorage?: Record<string, string>;
  sessionStorage?: Record<string, string>;
}

/**
 * Reads a captured session bundle from disk.
 *
 * @param name - Bundle name; defaults to the shared session.
 * @returns The parsed bundle.
 */
export function readSessionBundle(name?: string): SessionBundle {
  return JSON.parse(
    fs.readFileSync(name ? sessionPath(name) : sessionPath(), "utf8"),
  ) as SessionBundle;
}

/**
 * Replays a bundle into a context WITHOUT its IdP cookies.
 *
 * What `applySession` does, minus the cookies — and the omission is the whole
 * point rather than a shortcut:
 *
 * - For an expired session, cookies would let the hidden refresh iframe reach a
 *   live IdP session and silently re-authenticate, so the session under test
 *   would not actually be dead.
 * - For a sign-out, the cookies are what identify the IdP's SERVER-side session.
 *   Replaying them would mean the sign-out terminates the session that every
 *   other spec in the run is relying on. Without them the app still clears its
 *   own tokens and leaves — which is the behaviour under test — while the shared
 *   session survives.
 *
 * Storage is written only into the captured origin, for the same reason the
 * fixture scopes it: the init script runs in every document including the SDK's
 * cross-origin iframe, and the portal's tokens must not leak into another
 * origin.
 *
 * @param context - Context to prime; should be one the caller owns and closes.
 * @param bundle - The bundle to replay.
 */
export async function replayWithoutCookies(
  context: BrowserContext,
  bundle: SessionBundle,
): Promise<void> {
  await context.addInitScript((b: SessionBundle) => {
    if (!b.origin || window.location.origin !== b.origin) return;
    try {
      for (const [k, v] of Object.entries(b.localStorage ?? {})) {
        window.localStorage.setItem(k, v);
      }
      for (const [k, v] of Object.entries(b.sessionStorage ?? {})) {
        window.sessionStorage.setItem(k, v);
      }
    } catch {
      // Storage not reachable on this document yet; the next navigation re-runs
      // this script.
    }
  }, bundle);
}
