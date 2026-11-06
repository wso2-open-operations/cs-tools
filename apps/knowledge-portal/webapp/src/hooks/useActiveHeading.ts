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

/**
 * Id of the outline heading currently nearest the top of the viewport, for
 * marking the active entry of a table of contents. Falls back to the first id
 * before any heading has scrolled into view, and to nothing when the page has
 * no IntersectionObserver (older browsers, tests).
 */
export function useActiveHeading(ids: readonly string[]): string | undefined {
  const [active, setActive] = useState<string | undefined>(ids[0]);
  const key = ids.join("\u0000");

  useEffect(() => {
    setActive(ids[0]);
    if (ids.length === 0 || typeof IntersectionObserver === "undefined") return;
    const elements = ids
      .map((id) => document.getElementById(id))
      .filter((el): el is HTMLElement => el !== null);
    if (elements.length === 0) return;

    const visible = new Map<string, number>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) visible.set(entry.target.id, entry.boundingClientRect.top);
          else visible.delete(entry.target.id);
        }
        if (visible.size === 0) return;
        // The visible heading closest to the top of the reading area wins.
        const [topMost] = Array.from(visible.entries()).sort((a, b) => a[1] - b[1]);
        setActive(topMost[0]);
      },
      { rootMargin: "-96px 0px -70% 0px", threshold: [0, 1] },
    );
    elements.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
    // `key` stands in for the array identity.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  return active;
}
