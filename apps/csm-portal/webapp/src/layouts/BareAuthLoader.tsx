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

import { Box } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import RouteSuspenseFallback from "@components/route-fallback/RouteSuspenseFallback";

/**
 * `AuthGuard`'s own `loader` for a `bare` route (see `AuthGuardProps`) while
 * the Asgardeo sign-in flow is still resolving — a full-viewport, chrome-free
 * equivalent of `AppLayout`'s own loading state (no header/sidebar/banners,
 * matching what the route itself renders once signed in). Kept as its own
 * small component, not inlined in `AuthGuard`, so a `bare` route's loading
 * state can evolve independently of the normal `AppLayout` one.
 *
 * The centered progress bar itself is `RouteSuspenseFallback` — the same
 * spinner every lazy route chunk shows inside the app shell — wrapped here
 * in a full-viewport `Box` since a `bare` route has no app shell for it to
 * fill. Reusing it keeps the two in sync if the spinner is ever restyled.
 */
export default function BareAuthLoader(): JSX.Element {
  return (
    <Box sx={{ height: "100dvh", width: "100%", display: "flex", flexDirection: "column" }}>
      <RouteSuspenseFallback />
    </Box>
  );
}
