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

import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { Box } from "@mui/material";

/**
 * /issues's issue-number search box, debounced into the `q` URL param.
 * Rendered in the app header rather than on the page itself, as an ordinary
 * flex sibling of the sync button and user profile, so it reflows the same
 * way those do (e.g. when the sync button's hover-expanded status text grows)
 * instead of sitting at a layout the header's own flex/wrap doesn't govern.
 */
export function IssueSearchBox() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [qInput, setQInput] = useState(params.get("q") ?? "");

  useEffect(() => {
    const t = setTimeout(() => {
      // This effect re-runs on every params change (e.g. clicking to page 2
      // sets ?page=1, which re-fires it) since it's re-based on the current
      // `params` every time so a filter change applied while this timer is
      // pending is never clobbered by a stale snapshot when it finally fires
      // — see below. But that means it must bail out here whenever qInput
      // isn't actually introducing a new search, or it would unconditionally
      // strip `page` on every unrelated param change (e.g. pagination),
      // bouncing the page back to 1 a moment after any click.
      if ((params.get("q") ?? "") === qInput) return;
      const next = new URLSearchParams(params);
      if (qInput) next.set("q", qInput);
      else next.delete("q");
      next.delete("page"); // a new search always starts back at page 1
      void navigate(`/issues?${next.toString()}`.replace(/\?$/, ""), { replace: true });
    }, 300);
    return () => clearTimeout(t);
  }, [qInput, navigate, params]);

  return (
    <Box
      component="input"
      placeholder="Search by issue #…"
      value={qInput}
      onChange={(e) => setQInput((e.target as HTMLInputElement).value.replace(/\D/g, ""))}
      sx={{
        height: 36, width: 192, borderRadius: "9px", border: "1px solid var(--sla-border)", bgcolor: "var(--sla-card)",
        px: 1.5, fontSize: 13, color: "var(--sla-fg)", fontFamily: "inherit", "&:focus": { outline: "none", borderColor: "var(--sla-fg3)" },
      }}
    />
  );
}
