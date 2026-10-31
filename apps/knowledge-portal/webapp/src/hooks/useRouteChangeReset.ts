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

import { useEffect, useRef, type RefObject } from "react";
import { useLocation } from "react-router";

/**
 * On client-side navigation to a different path, scroll back to the top and
 * move focus to the main landmark so keyboard and screen-reader users start at
 * the new page rather than wherever the old one left them. Query-string-only
 * changes (a filter or search on the same page) are left alone. The initial
 * page load is left alone too.
 */
export function useRouteChangeReset(mainRef: RefObject<HTMLElement | null>): void {
  const { pathname } = useLocation();
  const previous = useRef(pathname);

  useEffect(() => {
    if (previous.current === pathname) return;
    previous.current = pathname;
    window.scrollTo(0, 0);
    mainRef.current?.focus({ preventScroll: true });
  }, [pathname, mainRef]);
}
