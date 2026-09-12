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

import { isCaseFieldFilterArray, type WidgetCaseFieldFilterLike } from "./widgetPreviewUrl";

/**
 * Merges a pie/bar widget's per-slice `query` under its own base `query`
 * (see `PieSlice`'s doc comment on the backend: "slice keys win on
 * conflict"), the way `useWidgetPieData`/`DashboardWidgetTile`'s click-through
 * both need. Both arguments are criteria objects — the widget-config key
 * that carries them was renamed `filters` -> `query`, but the criteria
 * object's OWN inner `filters` array (the case-search DSL below) keeps its
 * name, and so does the search request body's `filters` property.
 *
 * For every resourceType except `case`, criteria are a flat
 * `{ [namedField]: values }` record, so a plain object spread already gives
 * "slice keys win on conflict" for free — the slice's own keys simply
 * overwrite the base's same-named keys, and every other base key survives.
 *
 * `case` widgets carry the generic field/op/values DSL nested under a single
 * `filters` array property (`{ filters: BeCaseFieldFilter[] }`). A plain
 * object spread there is wrong: both objects have exactly one key
 * (`"filters"`), so the slice's array would silently replace the base's
 * array wholesale instead of overriding just the fields it actually
 * specifies — e.g. a "Critical" severity slice would lose the widget's own
 * base state filter entirely, and start counting cases in every state, not
 * just the open/in-progress ones the base widget itself is scoped to. This
 * function detects that shape and merges the two arrays by `field`, keeping
 * every base entry whose field the slice doesn't itself specify.
 *
 * The criteria object's `anyOf` (cross-field OR: an array of
 * `{filters: [...]}` branches, OR'd against each other) gets the same
 * treatment for the same reason, distributed rather than concatenated since
 * ANDing two OR sets is a cross product. See the comment at the merge itself.
 */
/** Same shape check as `isCaseFieldFilterArray`, but also accepts a
 * genuinely empty array — a slice or base widget legitimately carrying zero
 * extra filter conditions is not the same as "not this shape at all", and
 * must still trigger the array-merge path below rather than the naive
 * fallback spread (which would otherwise wipe out the other side's
 * non-empty array whenever one side happens to be empty). */
function isCaseFieldFilterArrayOrEmpty(value: unknown): value is WidgetCaseFieldFilterLike[] {
  return (Array.isArray(value) && value.length === 0) || isCaseFieldFilterArray(value);
}

/** One `anyOf` branch: its own predicate array, ANDed within the branch. The
 * branches are OR'd against each other. */
interface WidgetFilterBranch {
  filters: WidgetCaseFieldFilterLike[];
}

/** Structural check for the `anyOf` branch array, matching the backend's
 * `CaseFilterBranch` (`{filters: [...]}`, at least one predicate each). */
function isBranchArray(value: unknown): value is WidgetFilterBranch[] {
  return (
    Array.isArray(value) &&
    value.length > 0 &&
    value.every(
      (b) =>
        typeof b === "object" &&
        b !== null &&
        isCaseFieldFilterArrayOrEmpty((b as { filters?: unknown }).filters),
    )
  );
}

/** Merges two predicate arrays by `field`: every base entry whose field the
 * slice does not itself specify survives, and the slice wins on conflict. */
function mergeFilterArrays(
  baseArr: WidgetCaseFieldFilterLike[],
  sliceArr: WidgetCaseFieldFilterLike[],
): WidgetCaseFieldFilterLike[] {
  const sliceFields = new Set(sliceArr.map((f) => f.field));
  return [...baseArr.filter((f) => !sliceFields.has(f.field)), ...sliceArr];
}

export function mergeWidgetFilters(
  base: Record<string, unknown> | null | undefined,
  slice: Record<string, unknown> | null | undefined,
): Record<string, unknown> {
  // Both a widget's own base `query` and a slice's own `query` are legally
  // absent on the wire — a slices-only widget (no top-level `query`, e.g. a
  // shape "bar" widget whose every slice supplies its own criteria) marshals
  // its base as JSON `null` (see `BeDashboardWidget.query`'s doc comment),
  // and a slice with no criteria of its own beyond the base is exactly as
  // legitimate. Normalizing to `{}` up front means every access below can
  // assume an object, the same way an always-present `query` always could.
  const baseObj = base ?? {};
  const sliceObj = slice ?? {};
  const merged = { ...baseObj, ...sliceObj };

  const baseArr = baseObj.filters;
  const sliceArr = sliceObj.filters;
  if (isCaseFieldFilterArrayOrEmpty(baseArr) && isCaseFieldFilterArrayOrEmpty(sliceArr)) {
    merged.filters = mergeFilterArrays(baseArr, sliceArr);
  }

  // `anyOf` has exactly the same problem the inner `filters` array had, and
  // it is not hypothetical: the backend loader actively PRODUCES `anyOf` by
  // migrating the legacy `orGroups` key, so a migrated widget carrying an OR
  // group plus a slice that also uses one would, under a plain spread, lose
  // every base branch — silently widening that slice's count rather than
  // narrowing it, exactly the failure the inner-array merge above exists to
  // prevent.
  //
  // The branches are OR'd, so ANDing the slice's set under the base's is a
  // distribution, not a concatenation: (B1 | B2) AND (S1 | S2) becomes the
  // four pairwise-merged branches (B1∧S1 | B1∧S2 | B2∧S1 | B2∧S2), each pair
  // merged by `field` on the same "slice wins" rule as the flat array. Both
  // sides are single-digit in practice, so the product stays small.
  //
  // Only relevant when BOTH sides set it: a plain spread already does the
  // right thing when just one does (the base's survives, or the slice's is
  // adopted). Anything not matching the branch shape falls through to the
  // spread's last-writer-wins rather than being mangled into a query that
  // would be accepted but mean something else.
  const baseBranches = baseObj.anyOf;
  const sliceBranches = sliceObj.anyOf;
  if (isBranchArray(baseBranches) && isBranchArray(sliceBranches)) {
    merged.anyOf = baseBranches.flatMap((b) =>
      sliceBranches.map((s) => ({ ...b, ...s, filters: mergeFilterArrays(b.filters, s.filters) })),
    );
  }

  return merged;
}
