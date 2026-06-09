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

import { useEffect, useRef, useState } from "react";
import { useAsgardeo } from "@asgardeo/react";
import type { IdTokenClaims } from "@utils/userClaims";

// Decodes and caches the ID token payload once the user is signed in. The IdP
// SDK exposes a `user` field on the auth context, but only when
// `preferences.user.fetchUserProfile` is true, which triggers an additional
// SCIM call we don't need. The ID token is already in storage; just decode it.
//
// We capture `getDecodedIdToken` via a ref instead of putting it in the deps
// array because the SDK does not guarantee referential stability for the
// callback — adding it would re-run the effect on every render and trigger
// repeated token decodes.
export function useIdTokenClaims(): IdTokenClaims | undefined {
  const { getDecodedIdToken, isSignedIn } = useAsgardeo();
  const getDecodedIdTokenRef = useRef(getDecodedIdToken);
  useEffect(() => {
    getDecodedIdTokenRef.current = getDecodedIdToken;
  }, [getDecodedIdToken]);

  const [claims, setClaims] = useState<IdTokenClaims | undefined>(undefined);

  useEffect(() => {
    if (!isSignedIn) {
      // Clear claims when signed out; the async branch below subscribes to the
      // external Asgardeo token and setStates from its callback.
      // eslint-disable-next-line react-hooks/set-state-in-effect -- syncs to external auth state
      setClaims(undefined);
      return;
    }

    let cancelled = false;
    void (async () => {
      try {
        const decoded = (await getDecodedIdTokenRef.current()) as unknown as IdTokenClaims;
        if (!cancelled) setClaims(decoded);
      } catch {
        if (!cancelled) setClaims(undefined);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isSignedIn]);

  return claims;
}
