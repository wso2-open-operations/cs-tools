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

// Module-scope, shared by every caller across the app (AuthGuard's route-mount
// check and useAuthApiClient's fetch-401 recovery both call this). The identity provider
// SDK's `signInSilently()` opens its own hidden iframe per invocation with no
// coordination of its own — if two independent callers each notice the same
// expired token and call it separately, the app ends up running two
// unsynchronized hidden-iframe re-auth cycles at once, which can starve or
// drop whichever in-flight request (e.g. a POST creating a comment) is
// racing against them. Single-flighting at this shared module scope, rather
// than per-caller, ensures the whole app ever has at most one silent
// re-auth attempt in flight, and every caller awaits the SAME outcome.
let inFlight: Promise<boolean> | null = null;

/**
 * Run `signInSilently` at most once concurrently across the whole app.
 * Every caller while an attempt is in flight awaits that same attempt instead
 * of starting a new one. `onError` (optional) is invoked with a short message
 * — never the raw error object — if the attempt rejects.
 */
export function trySilentSignInOnce(
  signInSilently: () => Promise<unknown>,
  onError?: (message: string) => void,
): Promise<boolean> {
  if (!inFlight) {
    inFlight = Promise.resolve(signInSilently())
      .then((result) => Boolean(result))
      .catch((error) => {
        const message = error instanceof Error ? error.message : "unknown error";
        onError?.(message);
        return false;
      })
      .finally(() => {
        inFlight = null;
      });
  }
  return inFlight;
}
