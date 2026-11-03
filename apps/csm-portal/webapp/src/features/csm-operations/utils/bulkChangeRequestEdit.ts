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

import {
  changeRequestApprovedReadOnlyReason,
  changeRequestTransitionRequiresReason,
  customerApprovalLockedReason,
  customerProjectNeededForApproval,
  customerReviewLockedReason,
  EMERGENCY_CUSTOMER_STEPS_HELPER,
  isChangeRequestCreationPhase,
  isChangeRequestTerminal,
  isEmergencyChangeRequestType,
} from "@features/csm-operations/utils/changeRequests";
import { parseBackendTimestamp } from "@utils/dateTime";
import type { BeChangeRequestDetail, BePatchChangeRequestPayload } from "@api/backend/types";

/**
 * The one change a bulk edit applies to every selected change request: only
 * the fields set here are sent, so each change request keeps its own value for
 * the rest. Every field `PATCH /change-requests/{id}` takes from the edit form,
 * plus the state (a move every selected change request can make, from
 * `bulkChangeRequestTargets`), on hold and the two journals. Not here: Request
 * approval and the answers to a customer's proposed time, which are one change
 * request's own conversation.
 *
 * `workNote` is required with a state that needs a stated reason (Roll back,
 * Cancel change): it is that reason, and it rides in the same request as the
 * state, so a refused move records no reason either.
 */
export type BulkChangeRequestEdit = Pick<
  BePatchChangeRequestPayload,
  | "state"
  | "title"
  | "description"
  | "type"
  | "category"
  | "impact"
  | "priority"
  | "onHold"
  | "onHoldReason"
  | "serviceId"
  | "serviceOfferingId"
  | "assignedTeamId"
  | "assignedEngineerId"
  | "requestedById"
  | "caseId"
  | "plannedStartOn"
  | "plannedEndOn"
  | "rollbackDurationText"
  | "justification"
  | "implementationPlan"
  | "impactDescription"
  | "rollbackPlan"
  | "testPlan"
  | "serviceOutage"
  | "communicationPlan"
  | "affectedServicesText"
  | "affectedComponentsText"
  | "projectId"
  | "deploymentIds"
  | "deploymentProductIds"
  | "customerApprovalRequired"
  | "customerReviewRequired"
  | "isPlanningVisibleToCustomers"
  | "workNote"
  | "comment"
> & {
  /**
   * Past New the dialog shows the Customer Project every selected change request
   * shares, fixed, and sends it only to change the deployments within it: a
   * record whose project is no longer that one is skipped, never moved to it.
   * Never sent itself.
   */
  projectFixed?: boolean;
};

/** What is sent to one change request, or why nothing is. */
export type BulkChangeRequestPlan = { patch: BePatchChangeRequestPayload } | { skip: string };

/**
 * What can still be sent to a change request that has been approved: the
 * lifecycle (state, on hold) and the two journals. Everything else is its own
 * content, which the entity service then refuses (`approvedLockFields`), so an
 * allow-list: a field added later is read-only once approved until it is listed.
 */
const APPROVED_ALLOWED_FIELDS: ReadonlySet<string> = new Set(["state", "onHold", "onHoldReason", "workNote", "comment"]);

/** Keys of an edit that steer the plan and are never sent. */
const PLAN_ONLY_FIELDS: ReadonlySet<string> = new Set(["projectFixed"]);

/**
 * Whether `key` is set in `edit`: a blank text is not (a bulk edit never clears
 * a field), while `false` and an empty list of deployments are values.
 */
function isSet(edit: BulkChangeRequestEdit, key: keyof BulkChangeRequestEdit): boolean {
  const value = edit[key];
  if (typeof value === "string") return value.trim() !== "";
  return value !== undefined && value !== null;
}

/** Whether `edit` changes any of the change request's own fields (not just its state, hold or journals). */
export function bulkEditTouchesContent(edit: BulkChangeRequestEdit): boolean {
  return (Object.keys(edit) as Array<keyof BulkChangeRequestEdit>).some(
    (key) => !APPROVED_ALLOWED_FIELDS.has(key) && !PLAN_ONLY_FIELDS.has(key) && isSet(edit, key),
  );
}

/** Why a bulk edit cannot be applied as it stands, or `null`. */
export function bulkChangeRequestEditProblem(edit: BulkChangeRequestEdit): string | null {
  if (edit.state && changeRequestTransitionRequiresReason(edit.state) && !edit.workNote?.trim()) {
    return "A reason is required for this state change. It is added to every selected change request as an internal work note.";
  }
  const start = parseBackendTimestamp(edit.plannedStartOn);
  const end = parseBackendTimestamp(edit.plannedEndOn);
  if (start && end && end.getTime() <= start.getTime()) {
    return "Planned end must be after planned start.";
  }
  if (edit.onHoldReason && edit.onHold !== true) {
    return "A hold reason goes with putting the change requests on hold.";
  }
  if (isEmergencyChangeRequestType(edit.type) && (edit.customerApprovalRequired || edit.customerReviewRequired)) {
    return EMERGENCY_CUSTOMER_STEPS_HELPER;
  }
  return null;
}

/** The words for a change request that has been approved and was asked to change a field. */
export const APPROVED_FIELDS_SKIPPED =
  "Approved, so its fields are read-only; only the state, on hold and notes can be changed.";

/** The words for a change request that has left New and was asked for another type or Customer Project. */
export const NEW_ONLY_FIELDS_SKIPPED =
  "Approval has been requested, so its type and Customer Project can no longer be changed.";

/** The words for a change request whose project is no longer the one the dialog showed fixed. */
export const PROJECT_CHANGED_SKIPPED =
  "Its Customer Project has changed since it was picked, so it was not changed.";

/** The words for a change request whose planned window the edit would turn inside out. */
export const PLANNED_WINDOW_SKIPPED =
  "Its planned end would not be after its planned start: the date set doesn't fit its other planned date.";

/** The words for a change request on hold that was asked to change state without being taken off hold. */
export const ON_HOLD_STATE_SKIPPED = "On hold, so its state can't change. Take it off hold in the same edit.";

/** The words for a change request with no assignment group that was asked to request approval. */
export const REQUEST_APPROVAL_NEEDS_TEAM_SKIPPED =
  "No assignment group, so approval can't be requested. Set one in the same edit.";

/**
 * The words for a change request with a customer step ticked and no Customer Project that was asked to request
 * approval. Unlike the assignment group, the project can't always be set in the same edit: a bulk edit sets it only
 * when every selected change request is New.
 */
export const REQUEST_APPROVAL_NEEDS_PROJECT_SKIPPED =
  "A customer step is ticked and there is no Customer Project, so approval can't be requested until it has one.";

/** What a bulk plan needs to know about one change request (its detail, read just before the run). */
export type BulkChangeRequestRecord = Partial<
  Pick<
    BeChangeRequestDetail,
    | "state"
    | "type"
    | "onHold"
    | "project"
    | "assignedTeam"
    | "customerApprovalRequired"
    | "customerReviewRequired"
    | "plannedStartOn"
    | "plannedEndOn"
  >
>;

/**
 * The PATCH for one change request in a bulk edit, from its current state.
 *
 * - A closed, canceled or rolled-back one is never edited.
 * - Once approved (Customer Approval and later) its fields are read-only: only
 *   the state, on hold and the journals are sent.
 * - The type and the Customer Project change only in New (resending its own is
 *   a no-op); the deployments change until approval, within its own project.
 *   One that has left New and is asked for another type or project is skipped.
 * - The planned window is judged as it will stand: one bound set alone is
 *   checked against the change request's own other bound.
 * - A customer requirement follows the edit form's add-only rule; one that
 *   already reads as asked is not sent (Emergency or not), and one this change
 *   request can't take (Emergency, its gate passed, no Customer Project) skips
 *   it with the reason.
 *   Re-typing to Emergency turns both off in the same request.
 * - On hold: putting one on hold that already is only updates the reason (a
 *   second "on hold" would clear it), taking one off that isn't is not sent, and
 *   one on hold can't change state unless the same edit takes it off hold.
 * - Request Approval (state `assess`) needs an assignment group, and in New a
 *   Customer Project when a customer step is ticked on a non-Emergency change,
 *   each as it will stand (the edit's own value, else the stored one): one
 *   without is skipped with the reason, as the backend would refuse it.
 *
 * A change request left with nothing to send is skipped with the reason. The
 * backend stays the authority: what it refuses for one change request (e.g.
 * nobody on the Customer Project to ask) is reported for that one and the rest
 * go through.
 */
export function bulkChangeRequestPlan(cr: BulkChangeRequestRecord, edit: BulkChangeRequestEdit): BulkChangeRequestPlan {
  if (isChangeRequestTerminal(cr.state)) {
    return { skip: "Closed, canceled or rolled back: it can't be edited." };
  }
  const approved = !!changeRequestApprovedReadOnlyReason(cr.state);
  const patch: BePatchChangeRequestPayload = {};
  for (const key of Object.keys(edit) as Array<keyof BulkChangeRequestEdit>) {
    if (PLAN_ONLY_FIELDS.has(key) || !isSet(edit, key) || (approved && !APPROVED_ALLOWED_FIELDS.has(key))) continue;
    const value = edit[key];
    (patch as Record<string, unknown>)[key] = typeof value === "string" ? value.trim() : value;
  }
  if (!approved && !isChangeRequestCreationPhase(cr.state)) {
    // Fixed once approval was requested; resending the stored value is accepted as a no-op.
    if (patch.type && patch.type !== cr.type?.trim().toLowerCase()) return { skip: NEW_ONLY_FIELDS_SKIPPED };
    delete patch.type;
    if (patch.projectId && patch.projectId !== cr.project?.id) return { skip: NEW_ONLY_FIELDS_SKIPPED };
  }
  if (!approved && edit.projectFixed && patch.projectId && patch.projectId !== cr.project?.id) {
    // The project was shown fixed, to change the deployments within it: never a move to it (in New too).
    return { skip: PROJECT_CHANGED_SKIPPED };
  }

  if (!approved && (patch.plannedStartOn || patch.plannedEndOn)) {
    // Nothing downstream orders the two bounds, so the window is judged here as it will stand. Both sides
    // are read as backend UTC: the edit's zoneless "YYYY-MM-DD HH:mm:ss" and the detail's RFC 3339 alike.
    const start = parseBackendTimestamp(patch.plannedStartOn ?? cr.plannedStartOn);
    const end = parseBackendTimestamp(patch.plannedEndOn ?? cr.plannedEndOn);
    if (start && end && end.getTime() <= start.getTime()) return { skip: PLANNED_WINDOW_SKIPPED };
  }

  if (!approved) {
    const emergency = isEmergencyChangeRequestType(patch.type ?? cr.type);
    if (isEmergencyChangeRequestType(patch.type)) {
      // An Emergency change has no customer steps: re-typing to it turns both off.
      patch.customerApprovalRequired = false;
      patch.customerReviewRequired = false;
    }
    const hasProject = !!(patch.projectId ?? cr.project?.id);
    const boxes = [
      ["customerApprovalRequired", cr.customerApprovalRequired ?? false, customerApprovalLockedReason],
      ["customerReviewRequired", cr.customerReviewRequired ?? false, customerReviewLockedReason],
    ] as const;
    for (const [key, stored, lockedReason] of boxes) {
      const wanted = patch[key];
      if (wanted === undefined) continue;
      // Already as asked: not sent (the backend takes it as a no-op, but it would still judge the lock),
      // so an Emergency change that holds a tick from before the rule is not refused for it.
      if (wanted === stored) {
        delete patch[key];
        continue;
      }
      if (wanted && emergency) return { skip: EMERGENCY_CUSTOMER_STEPS_HELPER };
      const locked = lockedReason(cr.state, { stored, hasProject });
      if (locked) return { skip: locked };
    }
  }

  if (patch.onHold === true && cr.onHold) delete patch.onHold;
  if (patch.onHold === false && !cr.onHold) delete patch.onHold;
  if (cr.onHold && patch.state && patch.onHold !== false) return { skip: ON_HOLD_STATE_SKIPPED };

  if (patch.state === "assess") {
    // Request Approval, judged as the backend does: the team (any state), then in New the Customer Project.
    if (!patch.assignedTeamId && !cr.assignedTeam?.id) return { skip: REQUEST_APPROVAL_NEEDS_TEAM_SKIPPED };
    const needsProject =
      isChangeRequestCreationPhase(cr.state) &&
      customerProjectNeededForApproval(
        patch.type ?? cr.type,
        patch.customerApprovalRequired ?? cr.customerApprovalRequired,
        patch.customerReviewRequired ?? cr.customerReviewRequired,
      );
    if (needsProject && !(patch.projectId ?? cr.project?.id)) return { skip: REQUEST_APPROVAL_NEEDS_PROJECT_SKIPPED };
  }

  if (Object.keys(patch).length === 0) {
    if (approved && bulkEditTouchesContent(edit)) return { skip: APPROVED_FIELDS_SKIPPED };
    if (edit.onHold !== undefined) return { skip: edit.onHold ? "Already on hold." : "Not on hold." };
    return { skip: "Nothing to change." };
  }
  return { patch };
}

/** The words for an approved change request in "Assign to me". */
export const APPROVED_ASSIGNEE_SKIPPED = "Approved, so its assignee can no longer be changed.";

/** "Assign to me" for one change request: the signed-in user as its assignee, unless it has been approved. */
export function assignChangeRequestToMePlan(
  cr: { state?: string | null; assignedEngineer?: { id?: string | null } | null },
  userId: string,
): BulkChangeRequestPlan {
  if (isChangeRequestTerminal(cr.state)) {
    return { skip: "Closed, canceled or rolled back: it can't be edited." };
  }
  if (changeRequestApprovedReadOnlyReason(cr.state)) return { skip: APPROVED_ASSIGNEE_SKIPPED };
  if (cr.assignedEngineer?.id === userId) return { skip: "Already assigned to you." };
  return { patch: { assignedEngineerId: userId } };
}
