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

/** Signed-in user fields stored in Redux after Asgardeo `getBasicUserInfo()`. */
export interface UserInfo {
  /** Display or login name. */
  username: string;
  /** Email when provided by the IdP. */
  email: string;
  /** OIDC subject identifier. */
  sub: string;
}
