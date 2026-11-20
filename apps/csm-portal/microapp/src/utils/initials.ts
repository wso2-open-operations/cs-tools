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

// Avatar fallback shown when there's no profile picture (or it fails to load) — e.g. "Jane Doe" -> "JD".
import { stripStaffNameMarker } from "@utils/staffNameMarker";

export function initialsOf(fullName: string): string {
  // Only the first two words are read, after the staff marker is removed (see staffNameMarker.ts):
  // a trailing marker, or anything else after the last name, never reaches the avatar.
  return (
    stripStaffNameMarker(fullName)
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase())
      .join("") || "?"
  );
}
