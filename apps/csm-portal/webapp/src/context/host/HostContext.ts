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

import { createContext, useContext } from "react";

/** The application the portal is running inside, when it is loaded as a remote. */
export interface Host {
  /** Shown in the link back, e.g. "One WSO2". */
  name: string;
  /** Leaves the portal and returns to the host's own pages. */
  onExit: () => void;
  /** Opens or closes the host application launcher at the supplied button. */
  onToggleAppSwitcher?: (anchor: HTMLElement) => void;
  appSwitcherOpen?: boolean;
  /**
   * Tokens for the portal's backend, exchanged by the host from its own.
   * `idToken` is absent when the identity provider issued none.
   * `rejectedToken` is an access token the backend has just refused, so the
   * host must not answer with it again.
   */
  getTokens: (request: {
    clientId: string;
    scopes: readonly string[];
    rejectedToken?: string;
  }) => Promise<{ accessToken: string; idToken?: string }>;
}

export const HostContext = createContext<Host | null>(null);

/** The host application, or null when the portal runs on its own. */
export function useHost(): Host | null {
  return useContext(HostContext);
}
