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
 * A code dashboard's operations calls (`portal.countOperations`,
 * `portal.searchOperations`): incidents, change requests and problems, each
 * through its own `/search` and `/aggregate`. Their values are
 * case-sensitive and differ per kind (an incident's state is `NEW`, a change
 * request's `new`), and a wrong one is a 400 that gives no reason, so every
 * arg is checked here against the kind's own lists first.
 */

/** One filter in an operations query's `filters` list. */
interface OperationsFilter {
  field: string;
  op: string;
  values: string[];
}

/** What one kind of operations record takes and where it is served. */
interface OperationsKind {
  path: string;
  itemsKey: string;
  /** Flat list keys and the values each takes. */
  lists: Record<string, readonly string[]>;
  /** `filters` fields: the op and the values each takes (`undefined`: one date). */
  fields: Record<string, { op: readonly string[]; values?: readonly string[] }>;
  sortFields: readonly string[];
}

const DATE_FIELD = { op: ["gte", "lte"] } as const;
const YES_NO = { op: ["eq"], values: ["true", "false"] } as const;

const OPERATIONS_KINDS: Record<string, OperationsKind> = {
  incident: {
    path: "/incidents",
    itemsKey: "incidents",
    lists: { priorities: ["CRITICAL", "HIGH", "MODERATE", "LOW", "PLANNING"] },
    fields: {
      state: { op: ["in"], values: ["NEW", "IN_PROGRESS", "ON_HOLD", "RESOLVED", "CLOSED", "CANCELLED"] },
      createdOn: DATE_FIELD,
      slaViolated: YES_NO,
      madeSla: YES_NO,
    },
    sortFields: ["createdOn", "updatedOn", "openedOn"],
  },
  change_request: {
    path: "/change-requests",
    itemsKey: "changeRequests",
    lists: {
      states: [
        "new",
        "assess",
        "authorize",
        "customer_approval",
        "scheduled",
        "implement",
        "review",
        "customer_review",
        "closed",
        "canceled",
        "rollback",
      ],
      impacts: ["high", "medium", "low"],
    },
    fields: {
      approval: { op: ["eq"], values: ["not requested", "requested", "approved", "rejected"] },
      createdOn: DATE_FIELD,
    },
    sortFields: ["createdOn", "updatedOn"],
  },
  problem: {
    path: "/problems",
    itemsKey: "problems",
    lists: {},
    fields: {
      state: { op: ["in"], values: ["NEW", "ASSESS", "ROOT_CAUSE_ANALYSIS", "FIX_IN_PROGRESS", "RESOLVED", "CLOSED"] },
    },
    sortFields: [],
  },
};

const OPERATIONS_KIND_NAMES = Object.keys(OPERATIONS_KINDS);

/** `portal.searchOperations`' default page and its cap. */
const SEARCH_OPERATIONS_DEFAULT_LIMIT = 10;
const SEARCH_OPERATIONS_MAX_LIMIT = 50;

/** The query both endpoints take, in `filters`. */
export interface OperationsQuery {
  priorities?: string[];
  states?: string[];
  impacts?: string[];
  filters?: OperationsFilter[];
}

/** The kind an args object names, or throws. */
function kindOf(fn: string, args: Record<string, unknown>): OperationsKind {
  const kind = OPERATIONS_KINDS[args.kind as string];
  if (typeof args.kind !== "string" || !kind) {
    throw new Error(`${fn}: kind must be one of ${OPERATIONS_KIND_NAMES.join(", ")}`);
  }
  return kind;
}

/**
 * Checks the query args both calls share (`kind`, the kind's flat lists,
 * `filters`, `mine`) and returns the query. `mine` is the viewer's own
 * records (`assignedUserId`), the only person a code dashboard can name, so
 * no user id is taken from the code. Any other arg not in `extra` is refused.
 */
function buildOperationsQuery(
  fn: string,
  args: Record<string, unknown>,
  extra: readonly string[],
  userId: string | undefined,
): { kind: OperationsKind; query: OperationsQuery } {
  const kind = kindOf(fn, args);
  const lists = Object.keys(kind.lists);
  for (const key of Object.keys(args)) {
    if (!["kind", "filters", "mine", ...lists, ...extra].includes(key)) {
      throw new Error(`${fn}: ${args.kind} takes no "${key}"`);
    }
  }
  const query: OperationsQuery = {};
  for (const list of lists) {
    const value = args[list];
    if (value === undefined) continue;
    const allowed = kind.lists[list];
    if (!Array.isArray(value) || value.length === 0 || !value.every((v) => allowed.includes(v))) {
      throw new Error(`${fn}: ${list} must be a non-empty list of ${allowed.join(", ")}`);
    }
    query[list as keyof Omit<OperationsQuery, "filters">] = value as string[];
  }
  const filters: OperationsFilter[] = [];
  if (args.filters !== undefined) {
    if (!Array.isArray(args.filters)) throw new Error(`${fn}: filters must be an array of {field, op, values}`);
    for (const f of args.filters) {
      const { field, op, values } = (f ?? {}) as { field?: unknown; op?: unknown; values?: unknown };
      const rule = kind.fields[field as string];
      if (typeof field !== "string" || !rule) {
        throw new Error(`${fn}: a ${args.kind} filter's field must be one of ${Object.keys(kind.fields).join(", ")}`);
      }
      if (!rule.op.includes(op as string)) {
        throw new Error(`${fn}: a ${field} filter's op must be ${rule.op.join(" or ")}`);
      }
      const one = !rule.values || op === "eq";
      if (
        !Array.isArray(values) ||
        values.length === 0 ||
        (one && values.length !== 1) ||
        !values.every((v) => typeof v === "string" && v !== "" && (!rule.values || rule.values.includes(v)))
      ) {
        throw new Error(
          rule.values
            ? `${fn}: a ${field} filter's values must be ${one ? "one of" : "from"} ${rule.values.join(", ")}`
            : `${fn}: a ${field} filter takes one date (YYYY-MM-DD or a date token)`,
        );
      }
      filters.push({ field, op: op as string, values: values as string[] });
    }
  }
  if (args.mine !== undefined && typeof args.mine !== "boolean") throw new Error(`${fn}: mine must be true or false`);
  if (args.mine) {
    if (!userId) throw new Error(`${fn}: the signed-in user is not loaded`);
    filters.push({ field: "assignedUserId", op: "in", values: [userId] });
  }
  if (filters.length > 0) query.filters = filters;
  return { kind, query };
}

/** Body of POST /incidents/aggregate (and the other kinds'). */
export interface OperationsAggregateRequest {
  filters: OperationsQuery;
  groupBy: "state";
  maxGroups?: number;
}

/** Response of an operations `/aggregate`. */
export interface OperationsAggregateResponse {
  groups?: { key: string; label: string; count: number }[] | null;
  othersCount?: number;
  totalRecords?: number;
}

/**
 * Turns `portal.countOperations(args)` into the kind's `/aggregate` path
 * and body, or throws (the code sees the message). Args: the query args,
 * `groupBy` (`"state"` only) and `maxGroups`. The endpoint always needs a
 * `groupBy`; a plain count groups by state and keeps only the total.
 */
export function buildCountOperationsRequest(
  args: unknown,
  userId: string | undefined,
): { path: string; body: OperationsAggregateRequest; grouped: boolean } {
  const all = (args ?? {}) as Record<string, unknown>;
  const { kind, query } = buildOperationsQuery("countOperations", all, ["groupBy", "maxGroups"], userId);
  const { groupBy, maxGroups } = all;
  if (groupBy !== undefined && groupBy !== "state") throw new Error('countOperations: groupBy must be "state"');
  if (maxGroups !== undefined && (groupBy === undefined || !(Number.isInteger(maxGroups) && (maxGroups as number) > 0))) {
    throw new Error("countOperations: maxGroups must be a positive integer, with groupBy");
  }
  return {
    path: `${kind.path}/aggregate`,
    body: { filters: query, groupBy: "state", ...(maxGroups !== undefined ? { maxGroups: maxGroups as number } : {}) },
    grouped: groupBy !== undefined,
  };
}

/** Reshapes an operations `/aggregate` response into what `portal.countOperations` returns. */
export function toCountOperationsResult(
  res: OperationsAggregateResponse,
  grouped: boolean,
): { total: number; groups?: { label: string; count: number }[] } {
  const total = res.totalRecords ?? 0;
  if (!grouped) return { total };
  const groups = (res.groups ?? []).map(({ label, count }) => ({ label, count }));
  if (res.othersCount) groups.push({ label: "Others", count: res.othersCount });
  return { total, groups };
}

/** Body of an operations `/search`, as `portal.searchOperations` sends it: always the first page. */
export interface OperationsSearchRequest {
  filters: OperationsQuery;
  sortBy?: { field: string; order: "asc" | "desc" };
  pagination: { offset: 0; limit: number };
}

/**
 * Turns `portal.searchOperations(args)` into the kind's `/search` path and
 * body, or throws. Args: the query args, `sortBy: {field, order?}` (the
 * kind's sort fields; a problem has none) and `limit` (1 to 50, default 10).
 */
export function buildSearchOperationsRequest(
  args: unknown,
  userId: string | undefined,
): { path: string; itemsKey: string; body: OperationsSearchRequest } {
  const all = (args ?? {}) as Record<string, unknown>;
  const { kind, query } = buildOperationsQuery("searchOperations", all, ["sortBy", "limit"], userId);
  const { sortBy, limit } = all;
  if (
    limit !== undefined &&
    !(Number.isInteger(limit) && (limit as number) >= 1 && (limit as number) <= SEARCH_OPERATIONS_MAX_LIMIT)
  ) {
    throw new Error(`searchOperations: limit must be an integer from 1 to ${SEARCH_OPERATIONS_MAX_LIMIT}`);
  }
  let sort: OperationsSearchRequest["sortBy"];
  if (sortBy !== undefined) {
    if (kind.sortFields.length === 0) throw new Error(`searchOperations: a ${all.kind} list takes no sortBy`);
    const { field, order } = (sortBy ?? {}) as { field?: unknown; order?: unknown };
    if (!kind.sortFields.includes(field as string)) {
      throw new Error(`searchOperations: sortBy.field must be one of ${kind.sortFields.join(", ")}`);
    }
    if (order !== undefined && order !== "asc" && order !== "desc") {
      throw new Error('searchOperations: sortBy.order must be "asc" or "desc"');
    }
    sort = { field: field as string, order: (order as "asc" | "desc" | undefined) ?? "desc" };
  }
  return {
    path: `${kind.path}/search`,
    itemsKey: kind.itemsKey,
    body: {
      filters: query,
      ...(sort ? { sortBy: sort } : {}),
      pagination: { offset: 0, limit: (limit as number | undefined) ?? SEARCH_OPERATIONS_DEFAULT_LIMIT },
    },
  };
}

/** An operations record as its `/search` returns it (the fields read here). */
interface OperationsSearchView {
  id: string;
  number?: string | null;
  subject?: string | null;
  state?: string | null;
  priority?: string | null;
  impact?: string | null;
  createdOn?: string | null;
  updatedOn?: string | null;
  openedOn?: string | null;
  assignedTo?: { name?: string | null } | null;
  assignedEngineer?: { name?: string | null } | null;
}

/** One record as `portal.searchOperations` returns it: plain values, the assignee by name. */
export interface CodeDashboardOperationsRecord {
  id: string;
  number: string | null;
  subject: string | null;
  state: string | null;
  priority: string | null;
  impact: string | null;
  createdOn: string | null;
  updatedOn: string | null;
  openedOn: string | null;
  assignee: string | null;
}

/** Reshapes an operations `/search` response into what `portal.searchOperations` returns. */
export function toSearchOperationsResult(
  res: Record<string, unknown>,
  itemsKey: string,
): { total: number; items: CodeDashboardOperationsRecord[] } {
  const records = (res[itemsKey] as OperationsSearchView[] | null | undefined) ?? [];
  const items = records.map(
    (r): CodeDashboardOperationsRecord => ({
      id: r.id,
      number: r.number ?? null,
      subject: r.subject ?? null,
      state: r.state ?? null,
      priority: r.priority ?? null,
      impact: r.impact ?? null,
      createdOn: r.createdOn ?? null,
      updatedOn: r.updatedOn ?? null,
      openedOn: r.openedOn ?? null,
      assignee: r.assignedTo?.name ?? r.assignedEngineer?.name ?? null,
    }),
  );
  return { total: typeof res.total === "number" ? res.total : items.length, items };
}
