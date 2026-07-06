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

import { useIdTokenClaims } from "@hooks/useIdTokenClaims";

/**
 * Display name for the signed-in engineer, resolved from the ID token: full
 * name, else given + family name, else the email local part, else a generic
 * fallback. Used to attribute session-authored comments and attachments before
 * the backend resolves its own author.
 */
export function useEngineerDisplayName(): string {
  const claims = useIdTokenClaims();
  return (
    claims?.name ||
    [claims?.given_name, claims?.family_name].filter(Boolean).join(" ").trim() ||
    claims?.email?.split("@")[0] ||
    "Unknown engineer"
  );
}
