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

import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import type {
  BeStandardChangeSuggestion,
  BeStandardChangeSuggestionSkipped,
  BeStandardChangeSuggestionsResponse,
} from "@api/backend/types";

// The Suggested section of Standard changes (StandardChangeSuggestions.tsx):
// what each suggestion offers, and what a proposal started from one cites.
// entity-service finds and judges the suggestions on a schedule (once a day)
// and the page reads its last snapshot; these only read it.

/** The dismissal reason's limit (entity-service refuses a longer one). */
export const MAX_SUGGESTION_DISMISS_REASON_LENGTH = 1000;

/** A proposal cites at most this many change requests (entity-service's cap). */
export const MAX_PROPOSAL_EVIDENCE = 100;

export interface SuggestionAction {
  /** Why it cannot be done now (the button is shown disabled with it); null when it can. */
  blockedReason: string | null;
}

export interface SuggestionActions {
  propose?: SuggestionAction;
  dismiss?: true;
  restore?: true;
}

function joinNumbers(refs: Array<{ name: string; id: string }>): string {
  return refs.map((r) => r.name || r.id).join(", ");
}

/**
 * What a suggestion offers the viewer: Propose (a new standard change from its
 * newest clean change request) and Dismiss, or Restore for a dismissed one.
 * Only a person who can write gets them (the backend refuses anyone else). A
 * near miss shows Propose disabled with what it misses, and so does one already
 * being proposed (a second proposal would duplicate it) or one with no clean
 * change request to start from. A suggestion is deliberately not related to
 * the customer's existing templates (entity-service CLAUDE.md, "Standard change
 * suggestions"): whether one already covers it is for the reader to judge.
 */
export function suggestionActions(s: BeStandardChangeSuggestion, canWrite: boolean): SuggestionActions {
  if (!canWrite) return {};
  if (s.dismissal) return { restore: true };
  const notYet = s.eligible ? null : `Not yet a candidate: ${s.unmet.join("; ")}.`;
  const alreadyProposed = s.openProposals.length > 0 ? `Already being proposed in ${joinNumbers(s.openProposals)}.` : null;
  return {
    propose: {
      blockedReason:
        notYet ??
        alreadyProposed ??
        (s.proposeFrom ? null : "No closed change request without a deviation from its implementation plan to start from."),
    },
    dismiss: true,
  };
}

/**
 * The evidence a NEW proposal started from the suggestion cites: the change
 * request it starts from first (entity-service makes it the first evidence),
 * then the other closed members, newest first, within the proposal's cap.
 */
export function suggestionProposalEvidence(s: Pick<BeStandardChangeSuggestion, "proposeFrom" | "evidence">): string[] {
  if (!s.proposeFrom) return [];
  return [s.proposeFrom.id, ...s.evidence.map((e) => e.id)].slice(0, MAX_PROPOSAL_EVIDENCE);
}

/** The line above the list: how many, from what, judged by which thresholds. */
export function suggestionCriteriaText(r: BeStandardChangeSuggestionsResponse): string {
  const n = r.suggestions.length;
  const c = r.criteria;
  return (
    `${n} ${n === 1 ? "suggestion" : "suggestions"} from ${r.scanned.changeRequests} change requests completed in the last ${c.windowMonths} months` +
    ` that were not raised from a template. A suggestion is the same change closed at least ${c.minClosed} times,` +
    ` at least ${c.minSuccessPercent}% of them without a rollback, with no incident linked and no high risk or impact.`
  );
}

const count = (n: number): string => n.toLocaleString("en-US");

/**
 * The short label of a suggestion built from part of its subject group
 * (entity-service compares the newest 500): "Newest 500 of 3,000"; null when
 * the whole group was compared.
 */
export function suggestionSampleLabel(s: Pick<BeStandardChangeSuggestion, "sample">): string | null {
  return s.sample ? `Newest ${count(s.sample.compared)} of ${count(s.sample.total)}` : null;
}

/** The same in a sentence, for the details. */
export function suggestionSampleText(s: Pick<BeStandardChangeSuggestion, "sample">): string | null {
  return s.sample
    ? `Based on the newest ${count(s.sample.compared)} of ${count(s.sample.total)} change requests with this subject; the older ones were not compared.`
    : null;
}

/** How many skipped customers the notice names before "and N more". */
const SKIPPED_NAMED = 5;

/**
 * The notice for customers the latest refresh could not compute in time (too
 * many change requests): their suggestions are missing from the snapshot, which
 * is not the same as having none. null when there are none.
 */
export function suggestionSkippedText(skipped?: BeStandardChangeSuggestionSkipped[] | null): string | null {
  if (!skipped?.length) return null;
  const names = skipped.map((s) => s.customer.name || s.customer.id);
  const why = "it has too many change requests to compute in time";
  if (names.length === 1) return `${names[0]}'s suggestions were not computed in the latest refresh: ${why}.`;
  const shown = names.slice(0, SKIPPED_NAMED).join(", ");
  const more = names.length > SKIPPED_NAMED ? ` and ${names.length - SKIPPED_NAMED} more` : "";
  return `The suggestions of ${names.length} customers were not computed in the latest refresh (${shown}${more}): each has too many change requests to compute in time.`;
}

/** What the page says before the first snapshot has been computed. */
export const SUGGESTIONS_NOT_COMPUTED_YET =
  "Suggestions are computed once a day from completed change requests. None have been computed yet.";

/** "Updated Nov 6, 02:00 PM": when the snapshot was published, in the viewer's time zone; null before the first. */
export function suggestionUpdatedText(computedOn?: string | null, timeZone?: string): string | null {
  const when = formatBackendTimestampForDisplay(
    computedOn,
    { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" },
    timeZone,
  );
  return when ? `Updated ${when}` : null;
}
