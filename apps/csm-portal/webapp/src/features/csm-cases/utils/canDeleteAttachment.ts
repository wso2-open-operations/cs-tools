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
 * Whether to offer Delete on an attachment: its uploader may, and so may an
 * admin; nobody once the case is closed. UX only: the backend and entity-service
 * enforce the same rule, so this only hides a button that would fail.
 */
export function canDeleteAttachment(args: {
  caseClosed: boolean;
  isAdmin: boolean;
  /** The signed-in person's email. */
  myEmail?: string | null;
  /** The email of whoever uploaded the attachment. */
  uploaderEmail?: string | null;
}): boolean {
  if (args.caseClosed) return false;
  if (args.isAdmin) return true;
  const mine = args.myEmail?.trim().toLowerCase();
  const uploader = args.uploaderEmail?.trim().toLowerCase();
  return !!mine && mine === uploader;
}
