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

import { Tooltip } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { formatRelativeTime } from "@features/csm-dashboard/utils/abtDashboard";
import { formatAbsoluteForUser } from "@utils/dateTime";

interface RelativeTimeProps {
  /** Backend timestamp string (assumed UTC if no zone is present). */
  iso: string | null | undefined;
  /**
   * Optional permalink target. When provided, the time renders as an anchor
   * (Twitter/Facebook pattern: time = permalink to the entry). May be a hash
   * fragment (e.g. `#cmt-1001-2`) or a route.
   */
  href?: string;
  /** Optional className passthrough for layout tweaks. */
  className?: string;
}

/**
 * Renders a relative timestamp ("7h ago") with the full absolute datetime
 * (in the user's preferred zone) shown on hover. If `href` is provided, the
 * text becomes a permalink to that entry.
 */
export default function RelativeTime({
  iso,
  href,
  className,
}: RelativeTimeProps): JSX.Element {
  const relative = formatRelativeTime(iso);
  const absolute = formatAbsoluteForUser(iso) ?? "Unknown time";

  const inner = href ? (
    <a
      href={href}
      className={className}
      style={{
        color: "inherit",
        textDecoration: "none",
        whiteSpace: "nowrap",
        cursor: "pointer",
      }}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.textDecoration =
          "underline";
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.textDecoration = "none";
      }}
    >
      {relative}
    </a>
  ) : (
    <span className={className} style={{ whiteSpace: "nowrap" }}>
      {relative}
    </span>
  );

  return (
    <Tooltip title={absolute} placement="top" arrow>
      {inner}
    </Tooltip>
  );
}
