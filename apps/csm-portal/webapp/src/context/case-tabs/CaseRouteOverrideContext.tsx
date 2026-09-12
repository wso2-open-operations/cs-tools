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

/* eslint-disable react-refresh/only-export-components -- Provider component and its useXxx hook are colocated per the repo's context idiom (fast-refresh DX only) */

import { createContext, useContext, type JSX, type ReactNode } from "react";
import type { NavigateOptions, To } from "react-router";
import type { CaseRouteKind } from "@context/case-tabs/caseTabsTypes";

/**
 * What `CaseTabIsolatedRouter` overrides for a background/kept-alive
 * `CsmCaseDetailPage` instance, in place of what `useParams`/`useLocation`/
 * `useNavigate` would otherwise return for it.
 *
 * IMPORTANT: this is a plain React Context, deliberately NOT a second
 * react-router `<Router>` — react-router refuses to render a `<Router>`
 * inside another `<Router>` (an unconditional invariant, not a config
 * issue), and the app already has exactly one (`<BrowserRouter>` in
 * `App.tsx`). A second one per open tab is fundamentally incompatible with
 * that, so per-tab identity is layered on top of the SAME single real router
 * instead: `CsmCaseDetailPage` still reads the real `useParams`/
 * `useLocation`/`useNavigate` (via `useNormalizedIdParam`/`useNavTransition`)
 * exactly as it always has when there's no override in context (e.g. the
 * un-tabbed fallback for a case opened past the tab cap — see
 * `CaseDetailRouteSync`), and prefers this override's values instead when
 * one is present.
 */
export interface CaseRouteOverrideValue {
  caseId: string;
  kind: CaseRouteKind;
  pathname: string;
  search: string;
  hash: string;
  state: unknown;
  /** Same call signature as `useNavTransition`'s return value — a drop-in
   * replacement for it, not react-router's raw `useNavigate`. */
  navigate: (to: To | number, options?: NavigateOptions) => void;
}

const CaseRouteOverrideContext = createContext<CaseRouteOverrideValue | undefined>(
  undefined,
);

export function CaseRouteOverrideProvider({
  value,
  children,
}: {
  value: CaseRouteOverrideValue;
  children: ReactNode;
}): JSX.Element {
  return (
    <CaseRouteOverrideContext.Provider value={value}>
      {children}
    </CaseRouteOverrideContext.Provider>
  );
}

/** `undefined` outside a `CaseTabIsolatedRouter` — the normal/default case
 * (a directly-routed page, or any page other than `CsmCaseDetailPage`). */
export function useCaseRouteOverride(): CaseRouteOverrideValue | undefined {
  return useContext(CaseRouteOverrideContext);
}
