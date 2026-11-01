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

import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import type {
  AllocationSortField,
  SearchAllocationsPayload,
  SortOrder,
} from "../types";
import { ALLOCATION_STATES } from "./allocations";

export const DEFAULT_ROWS_PER_PAGE = 20;
export const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

export const SORT_FIELDS: readonly AllocationSortField[] = [
  "startDate",
  "endDate",
  "state",
  "allocationId",
  "accountName",
  "engineerName",
  "utilization",
];

export const DEFAULT_SORT: { field: AllocationSortField; order: SortOrder } = {
  field: "startDate",
  order: "desc",
};

/** The list's filters, sort and paging as the URL holds them. */
export interface ListParams {
  q: string;
  accountId: string;
  accountLabel: string;
  engineerId: string;
  engineerLabel: string;
  states: string[];
  from: string;
  to: string;
  sort: AllocationSortField;
  order: SortOrder;
  page: number;
  rows: number;
}

const DATE = /^\d{4}-\d{2}-\d{2}$/;

export function readListParams(sp: URLSearchParams): ListParams {
  const sort = sp.get("sort") as AllocationSortField | null;
  const order = sp.get("order");
  const page = Number.parseInt(sp.get("page") ?? "", 10);
  const rows = Number.parseInt(sp.get("rows") ?? "", 10);
  const states = sp.getAll("state").filter((s) => (ALLOCATION_STATES as readonly string[]).includes(s));
  const from = sp.get("from") ?? "";
  const to = sp.get("to") ?? "";
  return {
    q: sp.get("q") ?? "",
    accountId: sp.get("account") ?? "",
    accountLabel: sp.get("accountLabel") ?? "",
    engineerId: sp.get("engineer") ?? "",
    engineerLabel: sp.get("engineerLabel") ?? "",
    states,
    from: DATE.test(from) ? from : "",
    to: DATE.test(to) ? to : "",
    sort: sort && SORT_FIELDS.includes(sort) ? sort : DEFAULT_SORT.field,
    order: order === "asc" || order === "desc" ? order : DEFAULT_SORT.order,
    page: Number.isFinite(page) && page > 0 ? page : 0,
    rows: ROWS_PER_PAGE_OPTIONS.includes(rows) ? rows : DEFAULT_ROWS_PER_PAGE,
  };
}

/** Writes `p` to a fresh query string, leaving defaults out of the URL. */
export function writeListParams(p: ListParams): URLSearchParams {
  const sp = new URLSearchParams();
  if (p.q) sp.set("q", p.q);
  if (p.accountId) {
    sp.set("account", p.accountId);
    if (p.accountLabel) sp.set("accountLabel", p.accountLabel);
  }
  if (p.engineerId) {
    sp.set("engineer", p.engineerId);
    if (p.engineerLabel) sp.set("engineerLabel", p.engineerLabel);
  }
  for (const s of p.states) sp.append("state", s);
  if (p.from) sp.set("from", p.from);
  if (p.to) sp.set("to", p.to);
  if (p.sort !== DEFAULT_SORT.field || p.order !== DEFAULT_SORT.order) {
    sp.set("sort", p.sort);
    sp.set("order", p.order);
  }
  if (p.page > 0) sp.set("page", String(p.page));
  if (p.rows !== DEFAULT_ROWS_PER_PAGE) sp.set("rows", String(p.rows));
  return sp;
}

/** True when the date range is set at both ends and runs backwards. */
export function isBackwardsRange(from: string, to: string): boolean {
  return Boolean(from && to && from > to);
}

/** The search request for `p`. Empty filters are left out; a backwards range is not sent. */
export function buildSearchPayload(p: ListParams): SearchAllocationsPayload {
  const backwards = isBackwardsRange(p.from, p.to);
  const q = p.q.trim();
  return {
    pagination: { limit: p.rows, offset: p.page * p.rows },
    filters: {
      ...(q && { searchQuery: q }),
      ...(p.accountId && { accountId: p.accountId }),
      ...(p.engineerId && { engineerId: p.engineerId }),
      ...(p.states.length > 0 && { states: p.states }),
      ...(p.from && !backwards && { from: p.from }),
      ...(p.to && !backwards && { to: p.to }),
    },
    sortBy: { field: p.sort, order: p.order },
  };
}
