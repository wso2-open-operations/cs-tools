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
 * The wire shape of an SLA condition predicate (SLA SPEC.md R4): a tree of
 * `all`/`any`/`not` groups over `{field, op, value}` comparisons. This
 * mirrors entity-service's own JSONB storage byte for byte — no translation
 * happens on save, only on load (a condition saved by this editor is read
 * back exactly as it was sent).
 */

export type PredicateOp =
  | "eq"
  | "neq"
  | "in"
  | "not_in"
  | "is_null"
  | "is_not_null"
  | "changed"
  | "changed_to"
  | "changed_from";

/** Operators that take a value (R7) — the rest (is_null/is_not_null/changed)
 *  need none. */
export const OPS_WITH_VALUE: ReadonlySet<PredicateOp> = new Set([
  "eq",
  "neq",
  "in",
  "not_in",
  "changed_to",
  "changed_from",
]);

/** Operators whose value is a list rather than a single value. */
export const OPS_WITH_LIST_VALUE: ReadonlySet<PredicateOp> = new Set(["in", "not_in"]);

export const OP_LABELS: Record<PredicateOp, string> = {
  eq: "is",
  neq: "is not",
  in: "is any of",
  not_in: "is none of",
  is_null: "is empty",
  is_not_null: "is not empty",
  changed: "changed",
  changed_to: "changed to",
  changed_from: "changed from",
};

export interface PredicateLeaf {
  field: string;
  op: PredicateOp;
  value?: unknown;
}

export interface PredicateAllGroup {
  all: PredicateNode[];
}

export interface PredicateAnyGroup {
  any: PredicateNode[];
}

export interface PredicateNotGroup {
  not: PredicateNode;
}

export type PredicateNode = PredicateLeaf | PredicateAllGroup | PredicateAnyGroup | PredicateNotGroup;

export function isLeaf(node: PredicateNode): node is PredicateLeaf {
  return "field" in node;
}

export function isAllGroup(node: PredicateNode): node is PredicateAllGroup {
  return "all" in node;
}

export function isAnyGroup(node: PredicateNode): node is PredicateAnyGroup {
  return "any" in node;
}

export function isNotGroup(node: PredicateNode): node is PredicateNotGroup {
  return "not" in node;
}

export function emptyLeaf(defaultField: string): PredicateLeaf {
  return { field: defaultField, op: "eq", value: "" };
}
