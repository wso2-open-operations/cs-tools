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

import { useCallback } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { AUTH_SCOPES, tokenExchangeClientId } from "@config/tokenExchangeConfig";
import { useHost } from "@context/host/HostContext";

/**
 * Where the tokens sent to the backend come from.
 *
 * On its own, the portal signs the user in and uses that session's tokens.
 * Loaded inside a host, the session belongs to the host and its tokens are
 * meant for the host's client, so the portal asks the host for a pair
 * exchanged for its own client id instead. If the exchange yields no ID
 * token, the session's is used.
 *
 * `rejectedToken` is an access token the backend has just answered 401 to. A
 * host drops it and exchanges again; the SDK renews by its own means, so on
 * its own the argument is unused.
 */
export function useTokenSource() {
  const { getAccessToken: getSessionAccessToken, getIdToken: getSessionIdToken } = useAsgardeo();
  const host = useHost();

  const getAccessToken = useCallback(
    async (rejectedToken?: string): Promise<string> => {
      if (!host) return getSessionAccessToken();
      const tokens = await host.getTokens({ clientId: tokenExchangeClientId(), scopes: AUTH_SCOPES, rejectedToken });
      return tokens.accessToken;
    },
    [host, getSessionAccessToken],
  );

  // Asked for alongside the access token, so in a host this is answered from
  // the same exchange rather than a second one.
  const getIdToken = useCallback(async (): Promise<string> => {
    if (!host) return getSessionIdToken();
    const tokens = await host.getTokens({ clientId: tokenExchangeClientId(), scopes: AUTH_SCOPES });
    return tokens.idToken ?? getSessionIdToken();
  }, [host, getSessionIdToken]);

  return { getAccessToken, getIdToken };
}
