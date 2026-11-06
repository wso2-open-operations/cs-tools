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

import type { BeStandardChangeSuggestion, BeStandardChangeSuggestionsResponse } from "@api/backend/types";

// The Suggested section of Standard changes (StandardChangeSuggestions.tsx):
// what each suggestion offers, and what a proposal started from one cites.
// entity-service finds and judges the suggestions; these only read them.

/** The dismissal reason's limit (entity-service refuses a longer one). */
export const MAX_SUGGESTION_DISMISS_REASON_LENGTH = 1000;

/** A proposal cites at most this many change requests (entity-service's cap). */
export const MAX_PROPOSAL_EVIDENCE = 100;

/** How a suggestion relates to the customer's live templates, in words. */
export function suggestionMatchLabel(s: Pick<BeStandardChangeSuggestion, "match" | "template">): string {
  if (s.match === "covered" && s.template) return `Covered by ${s.template.number}`;
  if (s.match === "close" && s.template) return `Close to ${s.template.number}: revise it`;
  return "New";
}

export interface SuggestionAction {
  /** Why it cannot be done now (the button is shown disabled with it); null when it can. */
  blockedReason: string | null;
}

export interface SuggestionActions {
  propose?: SuggestionAction;
  revise?: SuggestionAction;
  openTemplate?: true;
  dismiss?: true;
  restore?: true;
}

function joinNumbers(refs: Array<{ name: string; id: string }>): string {
  return refs.map((r) => r.name || r.id).join(", ");
}

/**
 * What a suggestion offers the viewer. A new one is proposed, a close one is a
 * revision of the template it is close to, a covered one opens its template.
 * Only a person who can write starts a proposal or dismisses (the backend
 * refuses anyone else); a dismissed one can only be restored. A near miss shows
 * its action disabled with what it misses, and so does one already being
 * proposed (a second proposal would duplicate it) or one with no clean change
 * request to start from.
 */
export function suggestionActions(s: BeStandardChangeSuggestion, canWrite: boolean): SuggestionActions {
  const out: SuggestionActions = {};
  if (s.template && (s.match === "covered" || s.match === "close")) out.openTemplate = true;
  if (!canWrite) return out;
  if (s.dismissal) {
    out.restore = true;
    return out;
  }
  out.dismiss = true;
  const notYet = s.eligible ? null : `Not yet a candidate: ${s.unmet.join("; ")}.`;
  const alreadyProposed = s.openProposals.length > 0 ? `Already being proposed in ${joinNumbers(s.openProposals)}.` : null;
  if (s.match === "new") {
    out.propose = {
      blockedReason:
        notYet ??
        alreadyProposed ??
        (s.proposeFrom ? null : "No closed change request without a deviation from its implementation plan to start from."),
    };
  } else if (s.match === "close") {
    out.revise = { blockedReason: notYet ?? alreadyProposed };
    // A close template is revised, not opened: the revision opens it anyway.
    delete out.openTemplate;
  }
  return out;
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

/** The evidence a revision of the close template cites: every closed member, newest first, within the cap. */
export function suggestionRevisionEvidence(s: Pick<BeStandardChangeSuggestion, "proposeFrom" | "evidence">): string[] {
  const ids = s.proposeFrom ? [s.proposeFrom.id] : [];
  return [...ids, ...s.evidence.map((e) => e.id)].slice(0, MAX_PROPOSAL_EVIDENCE);
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
