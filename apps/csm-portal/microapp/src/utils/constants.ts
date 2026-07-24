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

export const ErrorMessages = {
  NATIVE_BRIDGE_NOT_AVAILABLE: "Native bridge is not available",
};

// Prefixed per-app so a token cached here can never be picked up by a sibling
// microapp (e.g. customer-portal) sharing the same storage partition. Each
// microapp is registered as its own OAuth client in the superapp, so the two
// apps' tokens carry different audiences — they were never interchangeable.
// Before this, both apps used the bare "accessToken"/"idToken" keys, so
// refreshToken()'s isTokenExpiringSoon() short-circuit (added in 1644c9cf3)
// could reuse whatever token was cached under this key regardless of which
// app it was actually issued for, causing 403s against the CSM backend after
// visiting Customer Portal first (confirmed live).
export const LocalStorageKeys = {
  accessToken: "csm_portal_accessToken",
  idToken: "csm_portal_idToken",
};
