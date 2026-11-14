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
 * UI-facing mirror of the backend's `UserReference` schema (see
 * `BeUserReference` in `api/backend/types.ts`): id, email and display name,
 * nothing else. Every person-valued field the backend returns now carries one
 * of these as a sibling of whatever it returned before.
 *
 * `id` is nullable — populated only where the backing data source already
 * resolved the actor to a user record. When it's null but `email` looks like
 * a real address, {@link UserRefLink} resolves it through the cached
 * email-to-id lookup (`useResolvedUserId`) rather than blocking render on it.
 */
export interface UserReference {
  id: string | null;
  email: string;
  name: string;
  /** True when the person is an internal user (WSO2 staff, such as a support
   * engineer). Absent means "not known to be internal". */
  isInternal?: boolean;
}
