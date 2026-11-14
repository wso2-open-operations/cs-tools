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

import type { JSX } from "react";
import StaffNameIcon from "@components/StaffNameIcon";
import { stripStaffMark } from "@utils/staffName";

/**
 * The name of whoever a work item is assigned to, as plain text. An assignee is
 * a support engineer, so the icon follows the name; the circled W the data may
 * carry is never shown. With no name it shows `fallback` (no icon).
 *
 * @returns {JSX.Element} The name and icon, or the fallback text.
 */
export default function AssigneeName({
  name,
  fallback = "—",
}: {
  name: string | null | undefined;
  fallback?: string;
}): JSX.Element {
  const shown = stripStaffMark(name).trim();
  if (!shown) return <>{fallback}</>;
  return (
    <>
      {shown}
      <StaffNameIcon size={14} />
    </>
  );
}
