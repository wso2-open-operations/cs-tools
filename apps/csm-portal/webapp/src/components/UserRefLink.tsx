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
import { Link as RouterLink } from "react-router";
import { useResolvedUserId } from "@features/csm-users/api/useResolvedUserId";
import StaffNameIcon from "@components/StaffNameIcon";
import { stripStaffMark } from "@utils/staffName";

interface UserRefLinkProps {
  /** Display name shown as the link text (or plain text when there's no id
   * to link to). */
  name: string;
  /**
   * The user's email. Used to resolve a user id when {@link userId} is
   * absent/null (see `useResolvedUserId`) — the profile route keys on the id,
   * not the email, so a person with no id available yet can't be linked
   * until resolution succeeds (or ever, if the email doesn't resolve).
   */
  email?: string;
  /**
   * The user's id, when already known (e.g. a `UserReference.id` the backend
   * populated directly, such as a comment author or the case assignee).
   * `null`/`undefined` both mean "not known here" and fall through to
   * resolving it from {@link email} instead of failing closed — this is also
   * what makes the component backward compatible with a backend that
   * predates `UserReference` and only ever sent a bare email.
   */
  userId?: string | null;
  /** Optional className passthrough for layout tweaks. */
  className?: string;
  /**
   * Set to `false` to skip the hover-underline treatment — for callers whose
   * container already signals clickability on its own (e.g. a table row
   * that's itself clickable with its own hover background), where the
   * underline would just be redundant. Defaults to `true`, since in most
   * usages this link is the only clickability signal on the name.
   */
  underlineOnHover?: boolean;
  /** True for an internal user (a support engineer): an icon follows the
   * name. The circled W the data may carry on a name is never shown. */
  isInternal?: boolean;
}

/**
 * Renders a person's name as a link to their profile page (`/people/:id`)
 * once an id is available — either passed directly via `userId` or resolved
 * from `email` through the cached email-to-id lookup — with the same
 * hover-underline treatment {@link RelativeTime} uses for its permalink
 * anchor. Falls back to plain text immediately (never a spinner) when there's
 * no id and none can be resolved.
 */
export default function UserRefLink({
  name,
  email,
  userId,
  className,
  underlineOnHover = true,
  isInternal = false,
}: UserRefLinkProps): JSX.Element {
  const resolvedId = useResolvedUserId(email, userId);

  const shownName = stripStaffMark(name);
  const icon = isInternal ? <StaffNameIcon /> : null;

  if (!resolvedId) {
    return (
      <span className={className}>
        {shownName}
        {icon}
      </span>
    );
  }

  return (
    <>
      <RouterLink
        to={`/people/${encodeURIComponent(resolvedId)}`}
        className={className}
        style={{
          color: "inherit",
          textDecoration: "none",
          cursor: "pointer",
        }}
        onMouseEnter={(e) => {
          if (!underlineOnHover) return;
          (e.currentTarget as HTMLAnchorElement).style.textDecoration =
            "underline";
        }}
        onMouseLeave={(e) => {
          (e.currentTarget as HTMLAnchorElement).style.textDecoration = "none";
        }}
      >
        {shownName}
      </RouterLink>
      {icon}
    </>
  );
}
