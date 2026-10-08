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

//
// In-browser fake of the change-request slice of the backend contract, for
// specs that must walk an approval flow deterministically without creating
// permanent ServiceNow records. Installed with `page.route`, so only the
// change-request endpoints (and the user id returned by `/users/me`) are
// faked -- everything else still reaches the real backend the rest of the
// suite runs against, and the browser session is still the captured one.
//
// The contract encoded here is the one the UI is built against:
//   - `legalNextStates` offers no manual way to `scheduled` at all, nor to `closed`
//     out of `customer_review`: staff never record a customer's approval or review.
//     They are given by the customer, in the Customer Portal (a manual PATCH of
//     either is a 400 whoever sends it, see `customerAnswerRefusal`). Assess lists
//     only Cancel: `authorize` is reached by the peer approval, never offered or taken
//     by a PATCH (it is Re-schedule only, out of Customer Approval);
//   - a manual state change is checked like entity-service's `patchChangeRequestTx` (see
//     `manualStateRefusal`), and a refusal is a 400 that writes nothing (not even the rest of its own
//     request): (1) a FINAL state (closed, canceled, rollback) has no exit at all, Cancel and a Re-schedule
//     included (`finalStateMessage`): a cancelled change cannot be revived with
//     `{state:"implement"}`; (2) the target-specific refusals (Request Approval only from New; `authorize` only
//     as Re-schedule; `customer_approval` and `scheduled` never manual; Roll back only from the review
//     states; Customer Review only when required; Close from Review only when not required); (3) the state
//     machine's own edges: a target that is not a next state of the current one is a jump over a state and
//     every approval gate on the way (`{state:"implement"}` from New, Assess or Authorize with the customer's
//     approval ticked would skip Peer, CAB and the customer) and is refused (`stateJumpMessage`);
//   - Request Approval (`PATCH {state:"assess"}`) on a Normal CR enters Assess
//     with a "Peer Approval" stage; on an Emergency CR it enters Authorize with
//     ONE "CAB Approval" stage in the existing CAB group (there is no ECAB: the previous system
//     has none, and no Peer / Assess stage either); on a Standard CR it goes straight
//     to the post-approval state with no approvals;
//   - approving Peer Approval adds a "CAB Approval" stage and moves to
//     Authorize; approving CAB moves the CR on by itself (an OLDER Emergency change's
//     "ECAB Approval" stage, `startAtState` / `startOlderEmergencyAtAuthorize`, does the same);
//   - an EMERGENCY change acts without the customer's consent: a create that ticks a
//     customer box, a PATCH that turns one on, is a 400 (`emergencyNoCustomerConsentMessage`),
//     and the flow ignores the stored boxes (a legacy row seeded with one ticked still goes
//     CAB -> Scheduled, and its Review closes); a box it already holds is a no-op;
//   - "the post-approval state" is `customer_approval` when the CR has
//     `customerApprovalRequired`, else `scheduled`;
//   - from `customer_approval` legalNextStates = [authorize, canceled], live customer
//     stage or not; "authorize" there is the wire name of the Time Change loop and the
//     state NEVER moves, whatever the type, and no CAB stage is ever opened (the
//     change itself has not changed): PATCH {state:"authorize", plannedStartOn?,
//     plannedEndOn?} is accepted ONLY from `customer_approval`. With NO customer proposal
//     waiting it is a plain Re-schedule: only when the window changes (else a 400 with the
//     backend's wording), refused (a 400, the words of Request Approval, nothing written)
//     when nobody can be asked, and it asks the customer again -- the customer's pending
//     stage is cancelled (kept as a record, reported PENDING with every approver CANCELLED,
//     like the backend) and a fresh customer stage is provisioned. It never writes the
//     stored `customerApprovalRequired` (a migrated row defaults to false and is asked
//     again all the same: the customer was being asked);
//   - a CUSTOMER'S PROPOSED TIME is the previous system's own `customer_updated_on` (the proposed
//     START) with WSO2's answer in `customer_updated_date_confirmation` (agree / disagree),
//     and nothing else: `customerProposes` writes the proposed start and clears the answer;
//     the change STAYS in Customer Approval, the planned window stays what WSO2 planned, no
//     stage or approver row is touched. The detail's `customerProposal` is the backend's
//     allowlist verdict: `answer: "pending"` only in Customer Approval, with a proposed
//     start that differs from the planned start, no answer, and no REQUESTED approver row
//     on anything but a customer stage (an unknown group blocks it too); `proposedBy*` only
//     while the last writer is a registered contact (`proposerKnown`). WSO2 answers it with
//     (1) ACCEPT: PATCH {confirmCustomerUpdatedDate:"agree", expectedCustomerUpdatedOn,
//     expectedPlannedStartOn, expectedPlannedEndOn} (all three required) -- the proposal becomes the planned
//     window (the planned length kept), the answer is agree and the change is `scheduled`
//     in one step: no CAB, no new customer request, `hasCustomerApproved` NOT stamped (no
//     staff action records the customer's approval); or (2) a COUNTER / DECLINE: a
//     Re-schedule that carries `expectedCustomerUpdatedOn` -- a different window asks the
//     customer again (answer disagree), the window as it is declines (answer disagree,
//     the customer keeps their live request, nothing else is written). Every staff answer
//     is a version check: a proposal or window that moved is a 409 in words;
//   - Review offers [customer_review, rollback, canceled] when
//     `customerReviewRequired`, else [closed, rollback, canceled];
//     `customer_review` -> [rollback, canceled] ([canceled] while a customer-group
//     review stage is live: a failed review is the customer's to give).
//     `rollback` ("Roll
//     back", the failed-review off-ramp) is accepted ONLY from those two states
//     (any other state is a 400 `state "rollback" can only be set from review
//     or customer_review`), is refused while a customer-group review stage is
//     live, cancels every still-requested approver row, and is final (a later
//     state change is a 400); the UI posts its reason as an internal note first
//     (`POST /change-requests/{id}/comments`, recorded in `journal()`);
//   - customer group: the CR's Customer Group is READ-ONLY and derived from its
//     Customer Project -- the project's registered contacts (`customerContacts`
//     on the detail and on link-options; FAKE_PROJECT_CONTACTS). When the
//     project has at least one eligible contact (not the creator), entering
//     `customer_approval` / `customer_review` provisions a "Customer Approval" /
//     "Customer Review" stage (approverName "Customer Group", approvers = the
//     contacts), and changing the project replaces a live stage with one for the
//     new project's contacts. `customerGroupId` is no longer accepted (400
//     `customerGroupId is no longer accepted: ...`), nor is `environmentIds`
//     (400 `environmentIds is no longer supported: ...`). While that stage is live
//     (still has REQUESTED approvers) legalNextStates for Customer Review is
//     [canceled] only (Re-schedule stays at Customer Approval), and the manual
//     PATCH {state:"rollback"} out of Customer Review is refused with a 400 whose
//     message is the backend's own (`customerStageManualRefusal` below) -- which
//     is also why the CSM portal's Roll back entry is disabled while a review stage
//     is live. The manual PATCH {state:"scheduled"} out of Customer Approval and
//     {state:"closed"} out of Customer Review are refused ALWAYS, live stage or
//     not (`customerAnswerRefusal`): staff never record the customer's answer. The
//     customer's answer is NOT given
//     in the CSM portal -- customers sign in to the customer portal, and nobody
//     in CSM is an approver of a customer stage -- so it never arrives through
//     this fake's decision route (a decision POSTed there while a customer stage
//     is live is a 403 naming the customer group, like the backend). A spec that
//     needs the customer's answer applies it server-side with
//     `customerDecides(contact, decision)`, which settles the stage like the
//     backend does: the contact's own row APPROVED / REJECTED, the co-contacts'
//     rows CANCELLED; Customer Approval approved -> scheduled, rejected ->
//     canceled; Customer Review approved -> closed, rejected -> rollback
//     (terminal: legalNextStates none); then the spec reloads the CSM page to see
//     it. With no project, or a project with no eligible contact, no stage is
//     provisioned: nobody is asked and nobody can answer, so the change waits at
//     the gate (Re-schedule, Roll back and Cancel are still on offer where they are
//     for any gate). Such a change can no longer be MADE by Request Approval, which is
//     refused (a 400 with the backend's words, `nobodyToAskMessage`; and
//     `REQUEST_APPROVAL_NEEDS_PROJECT` with no project at all) while a customer box is
//     ticked and nobody on the project can be asked, so a spec that needs the dead end
//     that remains for OLDER changes starts at the gate (`startAtState`). Ticking
//     a customer box on after New is refused with the same words in that case.
//     `canDecide` is true only on the signed-in user's own REQUESTED
//     row of a live stage, never for the creator;
//   - `customerApprovalRequired` / `customerReviewRequired` are on the detail
//     response. They are fully editable in New (the creation phase). From the moment
//     the change leaves New they are ADD-ONLY: false -> true is accepted until the
//     gate the box controls has passed (Customer Approval: while the change is in
//     New / Assess / Authorize; Customer Review: until Customer Review) and only
//     when the change already has a Customer Project; true -> false is refused in
//     every state but New. Each refusal is a 400 with the backend's own message
//     (see the constants below);
//   - `hasCustomerApproved` / `hasCustomerReviewed` (the customer's outcome) are
//     on the detail response and stamped true when a customer gate is left by the
//     customer's own answer (`customerDecides`: approved out of Customer Approval /
//     Customer Review), never by a staff action;
//   - the CR's creator can never approve;
//   - an approval is only actionable while the CR is in the state its stage belongs
//     to (Peer Approval: assess, CAB Approval, or an older Emergency change's ECAB one: authorize, Review: review,
//     Customer Approval / Customer Review: the same-named state). Like the
//     backend's reconcileStaleApprovers, every PATCH and every decision ends by
//     cancelling the still-REQUESTED approver rows of every stage the CR has left
//     -- ALL of them once it is closed / canceled / rollback -- so a Review
//     approver can decide while the CR is in review and no longer after it moved
//     on; `canDecide` is false on a REQUESTED row of a stage the CR has left (a
//     legacy row: see `setState`, which moves the CR without that sweep), and a
//     decision on one is a 409 with the backend's wording. Entering `review` on a
//     Normal CR provisions the "Review" stage (the assigned group's internal
//     members: FAKE_PEER and FAKE_PEER_COLLEAGUE). Deciding Review records the
//     answer and cancels the siblings; the CR stays in review;
//   - assignment groups: every internal stage carries `assignmentGroup: {id, name}`
//     -- Peer Approval the CR's assigned group (FAKE_PEER_GROUP, "Example Corp
//     ABT"), CAB Approval the CAB group (FAKE_CAB_GROUP, an Emergency change's one stage
//     included) and an older Emergency change's ECAB one FAKE_ECAB_GROUP -- and `GET /groups/{id}` answers the group page
//     (`{id, name, description, email, manager, members:[{id,name,email,userType,
//     role}], total}`, 404 for an unknown id). The Customer Approval / Customer
//     Review stages carry `assignmentGroup: null` (their approvers are the
//     project's registered contacts, which the page already has as
//     `customerContacts`); `failGroups(status)` makes the group endpoint fail,
//     for the error state;
//   - the customer scope (see FAKE_PROJECTS & co. below): `POST /projects/search`
//     lists the fake projects, `POST /change-requests/link-options` answers the
//     Customer Project -> Deployments -> Deployment products cascade (plus the
//     project's `customerContacts`), and `POST /change-requests` /
//     `PATCH /change-requests/{id}` store `projectId` / `deploymentIds` /
//     `deploymentProductIds` / `category` / `comment` / `workNote` after the
//     backend's own validation -- deployments must belong to the project,
//     deployment products must be exactly the derived set (all else a 400 with a
//     readable message). The Customer Project is editable only in New and FROZEN
//     from the moment the change leaves it (a PATCH that names a different project
//     is a 400; naming the stored one is an accepted no-op); the deployments stay
//     editable until `implement`, within the frozen project. A change can never
//     return to New (`{state: "new"}` after New is a 400), and Request Approval
//     (`{state: "assess"}`) is refused when a customer box is ticked and the change
//     has no Customer Project, or has one with nobody who can be asked (no registered
//     contact other than the creator). The detail response returns `project`, `deployments`,
//     `deploymentProducts`, `customerContacts`, `category` (EntityRef /
//     EntityRef[] / {id,name,email}[] / the category enum value).
//


import type { Page, Route } from "@playwright/test";

export type FakeCrType = "normal" | "standard" | "emergency";

export interface FakeUser {
  id: string;
  name: string;
  email: string;
}

export const FAKE_CREATOR: FakeUser = { id: "00000000-0000-0000-0000-00000000e001", name: "Casey Creator", email: "casey.creator@example.com" };
export const FAKE_PEER: FakeUser = { id: "00000000-0000-0000-0000-00000000e002", name: "Pat Peer", email: "pat.peer@example.com" };
export const FAKE_CAB: FakeUser = { id: "00000000-0000-0000-0000-00000000e003", name: "Cam Cab", email: "cam.cab@example.com" };
/**
 * The approver of the stage an OLDER Emergency change still carries ("ECAB Approval"): ECAB does not exist in the previous system and
 * nothing creates such a stage any more, but the ones already provisioned keep displaying and their approvers keep deciding them.
 */
export const FAKE_ECAB: FakeUser = { id: "00000000-0000-0000-0000-00000000e004", name: "Eli Ecab", email: "eli.ecab@example.com" };

/** Registered contacts of the Acme project (its read-only Customer Group: the customer-side approvers). */
export const FAKE_CUST_ONE: FakeUser = { id: "00000000-0000-0000-0000-00000000e005", name: "Mia Member", email: "mia.member@acme.example" };
export const FAKE_CUST_TWO: FakeUser = { id: "00000000-0000-0000-0000-00000000e006", name: "Max Member", email: "max.member@acme.example" };
/** Someone with no stake in the customer group. */
export const FAKE_OUTSIDER: FakeUser = { id: "00000000-0000-0000-0000-00000000e007", name: "Olive Outsider", email: "olive.outsider@example.com" };
/** The Beta project's only registered contact -- another customer's person. */
export const FAKE_BETA_CONTACT: FakeUser = { id: "00000000-0000-0000-0000-00000000e008", name: "Bea Beta", email: "bea.beta@beta.example" };

export const FAKE_CR_ID = "00000000-0000-0000-0000-00000000c001";

// ---------------------------------------------------------------------------
// Customer scope fixtures: three projects (Gamma has no deployments and no
// registered contacts); each deployment has a type (its environment role) and
// carries deployed products; each project has its own registered contacts.
// ---------------------------------------------------------------------------

export interface FakeRef {
  id: string;
  name: string;
}
export interface FakeDeployment extends FakeRef {
  projectId: string;
  type: string;
}
export interface FakeDeploymentProduct extends FakeRef {
  deploymentId: string;
}

export const FAKE_PROJECTS: FakeRef[] = [
  { id: "00000000-0000-0000-0000-00000000f001", name: "Acme Project" },
  { id: "00000000-0000-0000-0000-00000000f002", name: "Beta Project" },
  { id: "00000000-0000-0000-0000-00000000f003", name: "Gamma Project" },
];
/** Each project's registered contacts: its read-only Customer Group (initial; see setProjectContacts). */
export const FAKE_PROJECT_CONTACTS: Record<string, FakeUser[]> = {
  [FAKE_PROJECTS[0]!.id]: [FAKE_CUST_ONE, FAKE_CUST_TWO],
  [FAKE_PROJECTS[1]!.id]: [FAKE_BETA_CONTACT],
  [FAKE_PROJECTS[2]!.id]: [],
};
export const FAKE_DEPLOYMENTS: FakeDeployment[] = [
  { id: "00000000-0000-0000-0000-00000000d001", name: "Acme Production", projectId: FAKE_PROJECTS[0]!.id, type: "primary_production" },
  { id: "00000000-0000-0000-0000-00000000d002", name: "Acme Staging", projectId: FAKE_PROJECTS[0]!.id, type: "staging" },
  { id: "00000000-0000-0000-0000-00000000d003", name: "Beta Development", projectId: FAKE_PROJECTS[1]!.id, type: "development" },
];
export const FAKE_DEPLOYMENT_PRODUCTS: FakeDeploymentProduct[] = [
  { id: "00000000-0000-0000-0000-00000000b001", name: "API Manager 4.3.0", deploymentId: FAKE_DEPLOYMENTS[0]!.id },
  { id: "00000000-0000-0000-0000-00000000b002", name: "Identity Server 7.0.0", deploymentId: FAKE_DEPLOYMENTS[0]!.id },
  { id: "00000000-0000-0000-0000-00000000b003", name: "API Manager 4.2.0", deploymentId: FAKE_DEPLOYMENTS[1]!.id },
  { id: "00000000-0000-0000-0000-00000000b004", name: "Choreo 1.0.0", deploymentId: FAKE_DEPLOYMENTS[2]!.id },
];
/** Assignment groups the Assignment group picker can search (the Customer Group is not searched: it is derived). */
export const FAKE_GROUPS: FakeRef[] = [
  { id: "00000000-0000-0000-0000-00000000a101", name: "Apollo" },
  { id: "00000000-0000-0000-0000-00000000a102", name: "Artemis" },
];

/** A group's page as `GET /groups/{id}` returns it. */
export interface FakeGroupMember extends FakeUser {
  role: "member" | "lead";
}
export interface FakeApprovalGroup extends FakeRef {
  description: string | null;
  email: string | null;
  manager: FakeRef | null;
  members: FakeGroupMember[];
}

export const FAKE_PEER_COLLEAGUE: FakeUser = { id: "00000000-0000-0000-0000-00000000e011", name: "Quinn Peer", email: "quinn.peer@example.com" };
export const FAKE_CAB_COLLEAGUE: FakeUser = { id: "00000000-0000-0000-0000-00000000e012", name: "Cleo Cab", email: "cleo.cab@example.com" };
export const FAKE_CAB_NO_EMAIL: FakeUser = { id: "00000000-0000-0000-0000-00000000e013", name: "Cyd Cab", email: "" };
export const FAKE_ECAB_COLLEAGUE: FakeUser = { id: "00000000-0000-0000-0000-00000000e014", name: "Eve Ecab", email: "eve.ecab@example.com" };

/** The CR's assigned group: the Peer Approval stage is provisioned from it. */
export const FAKE_PEER_GROUP: FakeApprovalGroup = {
  id: "00000000-0000-0000-0000-00000000a201",
  name: "Example Corp ABT",
  description: "Builds and supports the Example Corp account.",
  email: "example-corp-abt@example.com",
  manager: { id: "00000000-0000-0000-0000-00000000e021", name: "Mona Manager" },
  members: [
    { ...FAKE_PEER, role: "lead" },
    { ...FAKE_PEER_COLLEAGUE, role: "member" },
  ],
};
/** The CAB Approval group (no description, email or manager: only the members show). */
export const FAKE_CAB_GROUP: FakeApprovalGroup = {
  id: "00000000-0000-0000-0000-00000000a202",
  name: "CAB Approval",
  description: null,
  email: null,
  manager: null,
  members: [
    { ...FAKE_CAB, role: "member" },
    { ...FAKE_CAB_COLLEAGUE, role: "member" },
    { ...FAKE_CAB_NO_EMAIL, role: "member" },
  ],
};
/** The unused ECAB group row: only an OLDER Emergency change's "ECAB Approval" stage (`startOlderEmergencyAtAuthorize`) links to it. */
export const FAKE_ECAB_GROUP: FakeApprovalGroup = {
  id: "00000000-0000-0000-0000-00000000a203",
  name: "ECAB Approval",
  description: "Emergency Change Advisory Board.",
  email: null,
  manager: null,
  members: [
    { ...FAKE_ECAB, role: "lead" },
    { ...FAKE_ECAB_COLLEAGUE, role: "member" },
  ],
};
const FAKE_APPROVAL_GROUPS: FakeApprovalGroup[] = [FAKE_PEER_GROUP, FAKE_CAB_GROUP, FAKE_ECAB_GROUP];

/** States from which project / deployments can no longer change. */
const SCOPE_LOCKED = ["implement", "review", "customer_review", "closed", "rollback", "canceled"];

const CATEGORIES = [
  "hardware", "software", "service", "system_software", "applications_software", "network",
  "telecom", "documentation", "other", "regular_release_cloud", "hotfix_release_cloud", "devops", "cloud_computing",
];

/** The customer scope the fake CR currently holds. */
export interface FakeScope {
  projectId: string | null;
  deploymentIds: string[];
  deploymentProductIds: string[];
  category: string | null;
}

/** A request the fake served, with the JSON body it carried (if any). */
export interface FakeRequestBody {
  request: string;
  body: Record<string, unknown> | undefined;
}

const sameSet = (a: string[], b: string[]): boolean => a.length === b.length && a.every((x) => b.includes(x));
const derivedProductIds = (deploymentIds: string[]): string[] =>
  FAKE_DEPLOYMENT_PRODUCTS.filter((p) => deploymentIds.includes(p.deploymentId)).map((p) => p.id);

/** The messages for the two fields the API no longer accepts (same text as the backend and the BFF). */
const CUSTOMER_GROUP_ID_REMOVED =
  "customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts";
const ENVIRONMENT_IDS_REMOVED = "environmentIds is no longer supported: deployments carry the environment";

// The lock on the customer's part of a change request (entity-service
// `patchChangeRequestTx`, "creation-phase gate"). Same text, character for character:
// the state is the change request's CURRENT one, in its API spelling.
export const CANNOT_RETURN_TO_NEW =
  'state "new" cannot be set: a change request that has left New cannot return to it. Cancel it and clone it instead.';
/**
 * The refusal of a state change out of a FINAL state (Closed, Canceled, Rollback): nothing moves a finished change
 * request anywhere (entity-service `changeRequestFinalRefusal`, the same words for the three, and for `{state: "new"}`
 * too, which says more than "cannot return to New"). A Cancel out of Customer Approval followed by
 * `{state: "implement"}` used to answer 200 and revive a change nobody had answered for.
 */
export const finalStateMessage = (target: string, from: string): string =>
  `state "${target}" cannot be set manually from ${from}: a change request that is ${from === "rollback" ? "rolled back" : from} cannot be moved`;

/** The refusal of a `state` that is none of the lifecycle's (entity-service `normalizeRequestedChangeRequestState`). */
export const notAStateMessage = (raw: string): string => `state ${JSON.stringify(raw)} is not a change request state`;

/** What a change waits for in a state staff cannot move it out of (entity-service `changeRequestWaitingOn`). */
const WAITING_ON: Record<string, string> = {
  new: 'approval has not been requested yet (Request Approval is state "assess")',
  assess: "it is waiting for its peer approval, which moves it on by itself",
  authorize:
    "it is waiting for its CAB approval, which moves it on by itself (to Customer Approval first when the customer's approval is required)",
  scheduled: "a change request goes through implement and review in order, one step at a time",
  implement: "a change request goes through implement and review in order, one step at a time",
  review: "a change request cannot go back to an earlier step",
};

/**
 * The refusal of a manual jump (entity-service `changeRequestJumpRefusal`): the state machine has no such edge from the
 * current state, so a step and every approval gate on the way would be skipped (`{state: "implement"}` from New, Assess or
 * Authorize with the customer's approval ticked used to land in Implement, with Peer, CAB and the customer skipped). It
 * says what the change waits for and which moves ARE open to staff from here (`legalNextStates`, with the review branch
 * the box picks).
 */
export const stateJumpMessage = (target: string, from: string, reviewRequired: boolean): string => {
  const open = legalNextStates(from, { customerApprovalRequired: false, customerReviewRequired: reviewRequired });
  const opens = open.length === 0 ? "none" : open.length === 1 ? `only ${open[0]}` : open.join(", ");
  return `state "${target}" cannot be set manually from ${from}: ${
    WAITING_ON[from] ?? "that is not a step of the change request's lifecycle from here"
  }; the moves open to staff from ${from} are: ${opens}`;
};
export const projectFrozenMessage = (state: string): string =>
  `projectId can no longer be changed: the Customer Project is fixed once approval has been requested (current state: ${state}). Cancel this change request and clone it to use another project.`;
export const requirementCannotBeRemovedMessage = (field: string, state: string): string =>
  `${field} can no longer be turned off: once approval has been requested a customer requirement can be added but never removed (current state: ${state}). Cancel and clone to correct it.`;
export const requirementNeedsProjectMessage = (field: string): string =>
  `${field} cannot be turned on: this change request has no Customer Project, and one can no longer be set after approval was requested. Cancel and clone it with a project.`;
/** The refusal of adding a customer requirement after the gate it controls has passed (entity-service validateCustomerGateEdits). */
export const requirementGatePassedMessage = (field: "customerApprovalRequired" | "customerReviewRequired", state: string): string =>
  field === "customerApprovalRequired"
    ? `customerApprovalRequired can no longer be changed: the change request has already passed the approval stage (current state: ${state})`
    : `customerReviewRequired can no longer be changed: the change request has already left the review stage (current state: ${state})`;
/**
 * The refusal of a customer box on an Emergency change (entity-service `emergencyNoCustomerConsentMsg`, with the box(es) it
 * names): an Emergency change is acted on without the customer's consent, so neither box can be required.
 */
export const emergencyNoCustomerConsentMessage = (fields: string[]): string =>
  `Emergency changes proceed without customer consent, so customer approval and customer review cannot be required (${fields.join(" and ")} must be false for an Emergency change)`;
export const REQUEST_APPROVAL_NEEDS_PROJECT =
  "approval cannot be requested: the customer's approval and/or review is required but no Customer Project is set, so there is nobody to ask. Select a Customer Project first (or clear the requirement).";
/**
 * The refusal of Request Approval (and of ticking a customer box on after New) when the Customer Project is set but nobody on
 * it can be asked: its registered contacts, leaving out the requester and anyone no longer active, are none (entity-service
 * `nobodyToAskMsg`). Such a change would reach Customer Approval / Customer Review with nobody to answer, and staff never answer
 * for the customer, so it could only be cancelled (or rolled back from Review). It names the box (or both) the refusal is about:
 * at Request Approval the boxes that are set (the request's value, else the stored one), after New the box(es) turned on. Same
 * text, character for character, which the real-stack spec asserts against the real API.
 */
export const nobodyToAskMessage = (approval: boolean, review: boolean): string =>
  `${approval && review ? "customer approval and customer review are" : review ? "customer review is" : "customer approval is"} required but nobody on this project can be asked (no registered contact other than the requester): register a contact for the project first`;

/**
 * The refusals of the two staff answers to a customer's proposed time (entity-service `acceptCustomerProposal`, and the
 * `authorize` branch of `patchChangeRequestTx`), character for character, so a spec and the real-stack describe assert the very
 * strings the backend answers.
 */
export const ACCEPT_ONLY_AGREE =
  'confirmCustomerUpdatedDate must be "agree": to decline a proposal, propose a different time (state "authorize" with the new planned window)';
export const ACCEPT_NEEDS_EXPECTED =
  "expectedCustomerUpdatedOn is required with confirmCustomerUpdatedDate: it names the proposed time you are accepting";
export const ACCEPT_NEEDS_EXPECTED_WINDOW =
  "expectedPlannedStartOn and expectedPlannedEndOn are required with confirmCustomerUpdatedDate: they name the planned time the proposal replaces";
export const ACCEPT_CANNOT_COMBINE =
  "confirmCustomerUpdatedDate cannot be combined with other fields; only expectedCustomerUpdatedOn, expectedPlannedStartOn and expectedPlannedEndOn go with it";
export const acceptNotInCustomerApproval = (state: string): string =>
  `a proposed time can only be accepted while the change request is in Customer Approval, but it is in ${stateName(state)}`;
export const NO_PROPOSAL_WAITING = "no new time proposed by the customer is waiting for a response on this change request";
/** An Accept (and the reason the read model gives with canAccept false) for a stored time nobody is recorded as having proposed. */
export const ACCEPT_PROPOSER_NOT_RECORDED =
  'nobody is recorded as having proposed this time (it may have been written by someone at WSO2 or left over from an earlier cycle), so it cannot be accepted: use "Propose a different time" to ask the customer to approve a time';
/** A staff {state: "authorize"} with no window while the stored time is one nobody is recorded as having proposed: nothing to decline. */
export const NO_RECORDED_PROPOSAL_TO_DECLINE =
  "no customer is recorded as having proposed the time stored on this change request, so there is no proposal to decline: send the new planned window to re-schedule it";
/** A staff Re-schedule that names the stored time it was shown (nobody recorded as its proposer) when it is no longer the stored one. */
export const storedTimeChangedMessage = (now: string): string =>
  `the time stored on this change request changed after you opened it (it is now ${now}); read it again before responding`;
export const proposalChangedMessage = (now: string): string =>
  `the customer's proposed time changed after you opened this change request (it is now ${now}); read it again before responding`;
export const windowChangedMessage = (now: string): string =>
  `the planned implementation time of this change request changed after you opened it (it is now ${now}); read it again before responding`;
export const proposalPassedMessage = (proposed: string): string =>
  `the time the customer proposed (${proposed}) has already passed, so it cannot be accepted: use "Propose a different time" to ask the customer to approve another time`;
/**
 * An Accept whose window (the proposed start plus the planned length) would end after the year 2100, the last year every planned window is held
 * to: `customer_updated_on` is a column the previous system writes too, so a date left far ahead can sit there. A customer's own proposal never gets that
 * far (it is refused at the proposal). Entity-service `msgAcceptTooFarAhead`, character for character.
 */
export const acceptTooFarAheadMessage = (proposed: string, end: string): string =>
  `the time the customer proposed (${proposed}) is too far ahead to be accepted: the window would end after the year 2100 (${end}), so use "Propose a different time" to ask the customer to approve another time`;
export const ACCEPT_WINDOW_HAS_NO_LENGTH =
  'the planned window has no length, so the customer\'s proposed start cannot be applied to it: use "Propose a different time"';
export const ON_HOLD_MESSAGE = "change request is on hold; take it off hold (onHold: false) before changing its state";
export const customerProposedWhileOpenMessage = (proposed: string): string =>
  `the customer proposed a new time (${proposed}) after you opened this change request; read it again to accept it or propose a different time`;
export const PROPOSAL_NO_LONGER_WAITING = "the customer's proposed time is no longer waiting for a response; read the change request again";
export const COUNTER_IS_THE_PROPOSAL = 'the time you are proposing is the one the customer proposed: use "Accept proposed time" instead';

/**
 * The 400 the backend answers a manual PATCH of `rollback` out of Customer Review
 * with while the customer group's review request is live (entity-service
 * `customerStageManualRefusal`, a ValidationError the BFF passes through): a failed
 * review is the customer's rejection, which they give in the Customer Portal. Same
 * text, character for character.
 */
export function customerStageManualRefusal(target: "rollback"): string {
  return `state "${target}" cannot be set manually: the customer's review has been requested from the customer group (the registered contacts of the change request's project) and is given by one of them approving or rejecting it in the change request's approvals (POST /change-requests/{id}/approvals/decision)`;
}

/**
 * The 400 the backend answers a manual PATCH out of a customer state with, from ANY caller and whether or not anybody
 * was asked (entity-service `customerOutcomeRefusal` behind `refuseStaffExitFromCustomerState`, a ValidationError the BFF
 * passes through): `scheduled` out of Customer Approval is the customer's approval and `closed` out of Customer Review is
 * the customer's review, which staff never record, and so is every other destination (`{state: "implement"}` out of
 * Customer Approval would skip the customer as surely): they are given in the Customer Portal. The closing words say what
 * staff can do instead, which depends on the state: Re-schedule or Cancel at Customer Approval; Roll back or Cancel at
 * Customer Review while nobody is being asked, and only Cancel (`reviewPending`) while the customer's review request is
 * live. `from` is the gate the change sits in (default: the one `scheduled` / `closed` leave). Same text, character for
 * character.
 */
export function customerAnswerRefusal(target: string, reviewPending = false, from?: "customer_approval" | "customer_review"): string {
  const gate = from ?? (target === "scheduled" ? "customer_approval" : "customer_review");
  if (gate === "customer_approval") {
    return `state "${target}" cannot be set manually from customer_approval: the customer's approval can only be given by the customer in the Customer Portal; cancel the change or re-schedule it instead`;
  }
  const instead = reviewPending ? "cancel the change" : "roll the change back or cancel it";
  return `state "${target}" cannot be set manually from customer_review: the customer's review can only be given by the customer in the Customer Portal; ${instead} instead`;
}

interface Approver {
  id: string;
  name: string;
  status: string;
}
interface Stage {
  stage: string;
  approverType: "STATIC_GROUP";
  approverName: string;
  /** The group the stage was provisioned from; null for the customer stages. */
  assignmentGroup: FakeRef | null;
  status: string;
  approvers: Approver[];
}

/** The two ServiceNow-style creation checkboxes the CR carries. */
export interface FakeCustomerFlags {
  customerApprovalRequired: boolean;
  customerReviewRequired: boolean;
}

/** States from which each checkbox can no longer be changed (backend refuses with 400). */
const APPROVAL_FLAG_LOCKED = ["customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"];
const REVIEW_FLAG_LOCKED = ["customer_review", "closed", "rollback", "canceled"];

export interface FakeChangeRequestApi {
  /** Who the app believes is signed in (applied on the next page load). */
  setViewer(user: FakeUser): void;
  /** The CR's current lifecycle state, as the fake backend holds it. */
  state(): string;
  /** The CR's current checkbox settings, as the fake backend holds them. */
  flags(): FakeCustomerFlags;
  /** Moves the fake CR to `next` out-of-band (e.g. while an edit dialog is
   * still open on a stale copy), without touching its approval stages. */
  setState(next: string): void;
  /** Every request the fake served, as "METHOD /path". */
  requests(): string[];
  /** Every request the fake served that carried a JSON body, in order. */
  requestBodies(): FakeRequestBody[];
  /** The customer scope the fake CR currently holds (as stored, ids only). */
  scope(): FakeScope;
  /** The `comment` / `workNote` journal entries the fake received, in order. */
  journal(): Array<{ kind: "comment" | "workNote"; text: string }>;
  /**
   * Deactivates a deployment server-side, behind the form's back: it drops out
   * of `link-options` and any create / PATCH that still names it is refused
   * with a 400 -- the "stale options" scenario.
   */
  retireDeployment(deploymentId: string): void;
  /**
   * Changes a project's registered contacts out-of-band (someone registers /
   * is deregistered); like the backend, a live customer stage follows on the
   * next write that touches the state or the project.
   */
  setProjectContacts(projectId: string, contacts: FakeUser[]): void;
  /** The planned window as the fake holds it ("YYYY-MM-DD HH:MM:SS", UTC). */
  planned(): { start: string; end: string };
  /** The approval stages as the fake holds them (stage name -> status). */
  stages(): Array<{ stage: string; status: string; approvers: Array<{ name: string; status: string }> }>;
  /**
   * Makes `GET /groups/{id}` fail with this HTTP status (for the dialog's
   * error state), or serve normally again with `null`.
   */
  failGroups(status: number | null): void;
  /**
   * The customer's answer, applied server-side: stands in for the customer
   * portal, where customers decide (they do not sign in to the CSM portal, so no
   * CSM page ever sends this decision). `contact`'s own REQUESTED row of the live
   * Customer Approval / Customer Review stage becomes APPROVED / REJECTED, the
   * co-contacts' rows CANCELLED, and the CR moves on like the backend moves it
   * (Customer Approval: approved -> scheduled, rejected -> canceled; Customer
   * Review: approved -> closed, rejected -> rollback). Throws, like the backend's
   * 403, when `contact` has no pending row on a live customer stage of the CR's
   * current state (a contact of another customer, a superseded stage, ...). The
   * open CSM page is not refreshed: reload it to see the outcome.
   */
  customerDecides(contact: FakeUser, decision: "approved" | "rejected"): void;
  /**
   * An OLDER change request, already in `state` with nobody asked: the internal approvals (Peer / CAB, or an Emergency change's ECAB one) settled the way
   * the flow leaves them, the change in `state`, and NO customer stage and nobody to answer. Request Approval can no longer
   * produce this change when a customer box is ticked (it is refused for a project nobody on which can be asked), so a spec that
   * needs the dead end that remains for changes that reached a customer gate before that rule -- or whose contacts left the
   * project afterwards, or that was migrated from the previous system without a request -- starts here: at `customer_approval` or
   * `customer_review` (the Review stage of a Normal change settled too), or at `review` for a change that is still to go to the
   * customer's review. Nothing is provisioned (like a legacy row: the customer stage appears only on the next write that
   * touches the state or the project, `syncCustomers`); the open page is not refreshed. A Standard change has no internal stage.
   */
  startAtState(state: "review" | "customer_approval" | "customer_review"): void;
  /**
   * An OLDER Emergency change request, as an earlier version of the portal left it: in Authorize with its one approval stage
   * named "ECAB Approval" (the ECAB group, `FAKE_ECAB_GROUP`) still REQUESTED for `FAKE_ECAB`. ECAB does not exist in the previous system and
   * nothing creates such a stage any more, but ones already provisioned keep displaying, and the approvers they asked can still
   * decide them. The open page is not refreshed.
   */
  startOlderEmergencyAtAuthorize(): void;
  /**
   * What the backend does on the next write that touches the change's state or
   * project (someone else's edit, a re-schedule, ...): the project's registered
   * contacts are asked -- a live customer stage is provisioned, or a stale one
   * replaced -- when the CR sits at a customer gate. Lets a spec make a customer
   * request appear behind an open page, so the page still lists Roll back as
   * available while the backend would now refuse it. The open page is not refreshed.
   */
  syncCustomers(): void;
  /**
   * The CUSTOMER'S PROPOSAL, applied server-side (the customer proposes in the customer portal, never in the CSM page): the
   * backend's `proposeCustomerTime`. `contact` must have a pending row on the live Customer Approval stage (else a 403/409 like the
   * backend's), `startOn` (RFC 3339 or "YYYY-MM-DD HH:MM:SS", UTC) must be a future start that is not the planned one. It writes the
   * proposed START to `customer_updated_on` and clears the standing answer -- and NOTHING else: the change stays in Customer
   * Approval, the planned window stays what WSO2 planned, no stage and no approver row is touched. `contact` is on record as the
   * proposer (the change's last writer is a registered contact). The open page is not refreshed: reload it.
   */
  customerProposes(contact: FakeUser, startOn: string): void;
  /**
   * A proposed date WSO2 cannot attribute: the previous system lets WSO2 users write `customer_updated_on` too, and one left over from an old
   * cycle reads the same. Sets the proposal as it stands in the data, with no proposer on record (`proposerKnown: false`, the
   * default) or with one. `confirmation` seeds a standing answer (agree / disagree); none = unanswered.
   */
  seedProposal(proposal: { startOn: string; proposerKnown?: boolean; confirmation?: "agree" | "disagree" | null }): void;
  /** The pair as the fake holds it: the previous system's `customer_updated_on` (RFC 3339) and WSO2's answer. */
  proposal(): { customerUpdatedOn: string | null; confirmation: "agree" | "disagree" | null };
  /** Puts the change on / takes it off hold (a state change is then refused). */
  setOnHold(onHold: boolean): void;
  /**
   * Somebody else re-schedules the change behind the open page: the planned window becomes `start` to `end` ("YYYY-MM-DD HH:MM:SS",
   * UTC) and nothing else is written (a proposal that waits keeps waiting). A request that names the window the page showed is then
   * a 409 `change_request_schedule_changed`. The open page is not refreshed.
   */
  moveWindow(start: string, end: string): void;
  /**
   * An internal approval that is still being asked while the change sits in Customer Approval (an inconsistent row, or an unknown
   * approval group): a REQUESTED approver row on a non-customer stage. The backend's pending allowlist reads it as "not a proposal
   * waiting for WSO2". The open page is not refreshed.
   */
  addInternalRequestedRow(): void;
  /** The customer's approval outcome stamp (`hasCustomerApproved`), as the fake holds it. */
  customerApproved(): boolean;
}

const CUSTOMER_STAGES = ["Customer Approval", "Customer Review"];

/** The one state in which each stage can be decided (the backend's approvalStageDecidableState). */
const STAGE_STATE: Record<string, string> = {
  "Peer Approval": "assess",
  "CAB Approval": "authorize",
  "ECAB Approval": "authorize",
  Review: "review",
  "Customer Approval": "customer_approval",
  "Customer Review": "customer_review",
};
/** States nothing can be approved in, or moved out of, any more. */
const FINAL_STATES = ["closed", "canceled", "rollback"];
/** Every state of the lifecycle: what a `state` in a PATCH may name. */
const ALL_STATES = ["new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"];
/** "customer_review" -> "Customer Review", for the refusal message. */
const stateName = (s: string): string => s.split("_").map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(" ");

/**
 * The forward moves a staff PATCH `{state}` is accepted for out of each non-final state (entity-service
 * `changeRequestForwardNextStates`). ONE table: `legalNextStates` renders it (what the page offers) and the fake's
 * `manualStateRefusal` enforces it (what the backend accepts), so the two cannot drift. Assess and Authorize are the approval
 * waits (the peer / CAB approval moves them on, never a PATCH: only Cancel is left), Customer Approval is the customer's step
 * (the customer's own answer moves it on; staff keep Re-schedule, `authorize`, and Cancel), Review lists both Closed and
 * Customer Review (the box picks one) and Customer Review has no forward move at all.
 */
const FORWARD_NEXT_STATES: Record<string, string[]> = {
  new: ["assess"],
  assess: [],
  authorize: [],
  customer_approval: ["authorize"],
  scheduled: ["implement"],
  implement: ["review"],
  review: ["closed", "customer_review"],
  customer_review: [],
};
/** The states a change can be rolled back from (the failed-review off-ramp). */
const ROLLBACK_FROM = ["review", "customer_review"];

/** Every state a staff PATCH may name from `state`, whatever the review box says; none for a final state. */
const staffTargets = (state: string): string[] => {
  const forward = FORWARD_NEXT_STATES[state];
  if (!forward) return [];
  return [...forward, ...(ROLLBACK_FROM.includes(state) ? ["rollback"] : []), "canceled"];
};

function legalNextStates(
  state: string,
  flags: Pick<FakeCustomerFlags, "customerReviewRequired"> & Partial<FakeCustomerFlags>,
  liveCustomerStage = false,
): string[] {
  return staffTargets(state).filter((target) => {
    // Review lists Closed and Customer Review: the box picks one.
    if (state === "review" && target === "customer_review" && !flags.customerReviewRequired) return false;
    if (state === "review" && target === "closed" && flags.customerReviewRequired) return false;
    // A failed review is the customer's rejection too, so Roll back is withheld while the group's review is pending.
    if (state === "customer_review" && target === "rollback" && liveCustomerStage) return false;
    return true;
  });
}

const nextStage = (name: string, group: FakeApprovalGroup, who: FakeUser): Stage => ({
  stage: name,
  approverType: "STATIC_GROUP",
  approverName: group.name,
  assignmentGroup: { id: group.id, name: group.name },
  status: "REQUESTED",
  approvers: [{ id: who.id, name: who.name, status: "REQUESTED" }],
});

export async function installFakeChangeRequestApi(
  page: Page,
  initialType: FakeCrType,
  viewer: FakeUser = FAKE_CREATOR,
  initialFlags: Partial<FakeCustomerFlags> = {},
  /** The customer scope the CR already holds. */
  initialScope: Partial<FakeScope> = {},
): Promise<FakeChangeRequestApi> {
  let type = initialType;
  let currentViewer = viewer;
  let state = "new";
  // The customer's outcome, stamped like the backend does when a customer gate is left by the customer's
  // own answer: `hasCustomerApproved` / `hasCustomerReviewed`.
  let customerApproved = false;
  let customerReviewed = false;
  let subject = "[E2E] approval flow (mocked)";
  const scope: FakeScope = {
    projectId: null,
    deploymentIds: [],
    deploymentProductIds: [],
    category: null,
  };
  if (initialScope.deploymentIds?.length) {
    scope.projectId = initialScope.projectId ?? null;
    scope.deploymentIds = [...initialScope.deploymentIds];
    scope.deploymentProductIds = derivedProductIds(scope.deploymentIds);
  } else if (initialScope.projectId) {
    scope.projectId = initialScope.projectId;
  }
  scope.category = initialScope.category ?? null;
  const journal: Array<{ kind: "comment" | "workNote"; text: string }> = [];
  const retired = new Set<string>();
  const bodies: FakeRequestBody[] = [];
  const flags: FakeCustomerFlags = {
    customerApprovalRequired: initialFlags.customerApprovalRequired ?? false,
    customerReviewRequired: initialFlags.customerReviewRequired ?? false,
  };
  /**
   * The customer's part the FLOW acts on: an Emergency change acts without the customer's consent, so whatever its stored
   * boxes hold (a legacy row can still carry one) it has neither -- CAB approval schedules it and Review closes it.
   */
  const effectiveGates = (): FakeCustomerFlags =>
    type === "emergency" ? { customerApprovalRequired: false, customerReviewRequired: false } : flags;
  /** Where a CR lands once its internal approval is granted. */
  const afterInternalApproval = (): string => (effectiveGates().customerApprovalRequired ? "customer_approval" : "scheduled");
  let stages: Stage[] = [];
  /** Registered contacts per project (the read-only Customer Group). */
  const contacts = new Map<string, FakeUser[]>(Object.entries(FAKE_PROJECT_CONTACTS).map(([id, users]) => [id, [...users]]));
  /** The CR's customer group as the fake derives it: its project's registered contacts. */
  const currentContacts = (): FakeUser[] => (scope.projectId ? (contacts.get(scope.projectId) ?? []) : []);
  /**
   * Who the backend could ask at a customer gate for `projectId`: its registered contacts leaving out the requester (the one
   * rule `syncCustomerStage` and the two refusals below share, like the backend's one provisioning function).
   */
  const askableContacts = (projectId: string | null | undefined): FakeUser[] =>
    projectId ? (contacts.get(projectId) ?? []).filter((u) => u.id !== FAKE_CREATOR.id) : [];
  /** When set, GET /groups/{id} fails with this status. */
  let groupFailure: number | null = null;
  let plannedStartOn = "2030-03-01 09:00:00";
  let plannedEndOn = "2030-03-01 11:00:00";
  // The previous system's own proposal pair (`customer_updated_on`, as an RFC 3339 instant, and WSO2's answer), and whether the change's last
  // writer is still a registered contact of the project (then the backend can name the proposer).
  let customerUpdatedOn: string | null = null;
  let confirmation: "agree" | "disagree" | null = null;
  let proposer: FakeUser | null = null;
  let onHold = false;
  /** "2030-03-01 09:00:00" (UTC, as the planned window is held and sent) or an RFC 3339 instant, as epoch ms. */
  const instantOf = (value: string | null | undefined): number | null => {
    if (!value) return null;
    const ms = Date.parse(/[zZ]|[+-]\d\d:?\d\d$/.test(value) ? value : `${value.replace(" ", "T")}Z`);
    return Number.isNaN(ms) ? null : ms;
  };
  /** An instant as the planned window is held and sent. */
  const plannedOf = (ms: number): string => new Date(ms).toISOString().slice(0, 19).replace("T", " ");
  /** An instant as the backend sends the proposal (RFC 3339). */
  const rfc3339Of = (ms: number): string => new Date(ms).toISOString().replace(".000Z", "Z");
  const log: string[] = [];
  const hasLiveCustomerStage = (): boolean =>
    stages.some((s) => CUSTOMER_STAGES.includes(s.stage) && s.status === "REQUESTED");
  const legal = (): string[] => legalNextStates(state, effectiveGates(), hasLiveCustomerStage());
  /** Moves the CR to `next`; entering a customer gate provisions the group's stage. */
  const enter = (next: string): void => {
    if (state === "customer_approval" && next === "scheduled") customerApproved = true;
    if (state === "customer_review" && next === "closed") customerReviewed = true;
    state = next;
    provisionReview();
    syncCustomerStage();
  };
  /**
   * Entering Review on a Normal change provisions the "Review" stage from the
   * assigned group (its internal members, the creator excluded); Emergency and
   * Standard changes never get one (the backend only provisions it once exactly
   * two internal stages exist).
   */
  function provisionReview(): void {
    if (state !== "review" || type !== "normal" || stages.some((s) => s.stage === "Review")) return;
    stages = [
      ...stages,
      {
        stage: "Review",
        approverType: "STATIC_GROUP",
        approverName: FAKE_PEER_GROUP.name,
        assignmentGroup: { id: FAKE_PEER_GROUP.id, name: FAKE_PEER_GROUP.name },
        status: "REQUESTED",
        approvers: FAKE_PEER_GROUP.members.filter((m) => m.id !== FAKE_CREATOR.id).map((m) => ({ id: m.id, name: m.name, status: "REQUESTED" })),
      },
    ];
  }
  /** Whether the CR has left (or can never be in) the state the stage can be decided in. */
  const stageOutOfState = (st: Stage): boolean => {
    const decidable = STAGE_STATE[st.stage];
    return decidable !== undefined && decidable !== state;
  };
  /**
   * The backend's `reconcileStaleApprovers`, run at the end of every PATCH and
   * every decision: the still-REQUESTED rows of every stage the CR has left are
   * cancelled -- all of them once it is closed / canceled / rollback. The stage
   * stays as a record, reported PENDING (nothing was approved or rejected on it).
   */
  function reconcile(): void {
    const final = FINAL_STATES.includes(state);
    for (const st of stages) {
      if (!final && !stageOutOfState(st)) continue;
      let cancelled = false;
      for (const a of st.approvers) {
        if (a.status === "REQUESTED") {
          a.status = "CANCELLED";
          cancelled = true;
        }
      }
      if (cancelled && st.status === "REQUESTED") st.status = "PENDING";
    }
  }
  /**
   * The backend's idempotent `provisionCustomerStage`: at a customer gate the
   * project's eligible registered contacts (everyone but the creator) get
   * exactly one live stage. Entered with contacts -> provisioned; project (or
   * its contacts) changed while a stage is live -> the old stage's REQUESTED
   * rows are cancelled and a new one is provisioned; no contacts -> pending
   * rows cancelled and nobody is asked (nobody can answer either: staff never
   * record the customer's answer); a stage already approved/rejected is never
   * re-provisioned.
   */
  function syncCustomerStage(): void {
    const kind = state === "customer_approval" ? "Customer Approval" : state === "customer_review" ? "Customer Review" : null;
    if (!kind) return;
    const members = askableContacts(scope.projectId);
    const live = stages.find((s) => s.stage === kind && s.status === "REQUESTED");
    if (live) {
      const have = live.approvers.map((a) => a.id);
      if (members.length > 0 && sameSet(have, members.map((m) => m.id))) return;
      for (const a of live.approvers) if (a.status === "REQUESTED") a.status = "CANCELLED";
      live.status = "CANCELLED";
    }
    const settled = stages.some((s) => s.stage === kind && (s.status === "APPROVED" || s.status === "REJECTED"));
    if (settled || members.length === 0) return;
    stages = [
      ...stages,
      {
        stage: kind,
        approverType: "STATIC_GROUP",
        approverName: "Customer Group",
        assignmentGroup: null, // the project's registered contacts, not a group
        status: "REQUESTED",
        approvers: members.map((m) => ({ id: m.id, name: m.name, status: "REQUESTED" })),
      },
    ];
  }

  /**
   * The customer's answer (see `customerDecides` on the returned API): the contact's
   * own live row is settled, the co-contacts' rows are cancelled, and the CR moves
   * on -- Customer Approval approved -> scheduled, rejected -> canceled; Customer
   * Review approved -> closed, rejected -> rollback.
   */
  function settleCustomerStage(contact: FakeUser, decision: "approved" | "rejected"): void {
    const current = stages.find(
      (s) =>
        CUSTOMER_STAGES.includes(s.stage) &&
        s.status === "REQUESTED" &&
        !stageOutOfState(s) &&
        s.approvers.some((a) => a.id === contact.id && a.status === "REQUESTED"),
    );
    const row = current?.approvers.find((a) => a.id === contact.id && a.status === "REQUESTED");
    if (!current || !row) {
      throw new Error(`${contact.name} has no pending customer approval or review on this change request`);
    }
    row.status = decision === "approved" ? "APPROVED" : "REJECTED";
    current.status = row.status;
    for (const a of current.approvers) if (a !== row && a.status === "REQUESTED") a.status = "CANCELLED";
    if (decision === "approved") enter(current.stage === "Customer Approval" ? "scheduled" : "closed");
    else enter(current.stage === "Customer Approval" ? "canceled" : "rollback");
    reconcile();
  }

  /**
   * The backend's allowlist for "a customer proposal is waiting for WSO2": in Customer Approval, a proposed start that differs from the
   * planned one, no answer yet, and the ONLY approver rows still REQUESTED are on a customer stage (any other REQUESTED row, an unknown
   * group included, blocks it). A change in Authorize with a live CAB stage, a closed or a scheduled one, or a date WSO2 itself wrote
   * the plan from never matches.
   */
  const proposalPending = (): boolean => {
    if (state !== "customer_approval" || !customerUpdatedOn || confirmation) return false;
    if (instantOf(customerUpdatedOn) === instantOf(plannedStartOn)) return false;
    return !stages.some((st) => !CUSTOMER_STAGES.includes(st.stage) && st.approvers.some((a) => a.status === "REQUESTED"));
  };
  /** The detail's `customerProposal` read model (omitted when nobody proposed anything). */
  const proposalView = (): Record<string, unknown> | undefined => {
    if (!customerUpdatedOn) return undefined;
    const pending = proposalPending();
    const answer = pending ? "pending" : confirmation === "agree" ? "agreed" : confirmation === "disagree" ? "disagreed" : "unanswered";
    const start = instantOf(customerUpdatedOn)!;
    const ps = instantOf(plannedStartOn);
    const pe = instantOf(plannedEndOn);
    // What the backend says of Accept while the proposal waits: the words of the refusal the PATCH would give.
    const blocked = !proposer
      ? ACCEPT_PROPOSER_NOT_RECORDED
      : onHold
      ? ON_HOLD_MESSAGE
      : start <= Date.now()
        ? proposalPassedMessage(customerUpdatedOn)
        : ps === null || pe === null || pe <= ps
          ? ACCEPT_WINDOW_HAS_NO_LENGTH
          : new Date(start + (pe - ps)).getUTCFullYear() > 2100
            ? acceptTooFarAheadMessage(customerUpdatedOn, rfc3339Of(start + (pe - ps)))
            : "";
    return {
      startOn: customerUpdatedOn,
      ...(pending && ps !== null && pe !== null && pe > ps ? { endOn: rfc3339Of(start + (pe - ps)) } : {}),
      answer,
      ...(pending ? { proposerRecorded: !!proposer } : {}),
      ...(pending && proposer ? { proposedByName: proposer.name, proposedByEmail: proposer.email, proposedOn: "2030-02-01T10:00:00Z" } : {}),
      ...(pending ? { canAccept: blocked === "", ...(blocked ? { acceptBlockedReason: blocked } : {}) } : {}),
    };
  };
  /** The customer is asked again: the live request is superseded (rows cancelled, the stage kept as a record) and a fresh one provisioned. */
  const askCustomersAgain = (): void => {
    for (const st of stages) {
      if (CUSTOMER_STAGES.includes(st.stage) && st.status === "REQUESTED") {
        for (const a of st.approvers) if (a.status === "REQUESTED") a.status = "CANCELLED";
        st.status = "PENDING";
      }
    }
    syncCustomerStage();
  };

  const detail = (): Record<string, unknown> => ({
    id: FAKE_CR_ID,
    number: "CHG0099001",
    subject,
    createdOn: "2026-01-01T00:00:00Z",
    createdBy: FAKE_CREATOR.email,
    state,
    type,
    assignedTeam: { id: "00000000-0000-0000-0000-00000000a001", name: "Platform" },
    requestedBy: { id: FAKE_CREATOR.id, name: FAKE_CREATOR.name },
    customerApprovalRequired: flags.customerApprovalRequired,
    customerReviewRequired: flags.customerReviewRequired,
    hasCustomerApproved: customerApproved,
    hasCustomerReviewed: customerReviewed,
    legalNextStates: legal(),
    plannedStartOn,
    plannedEndOn,
    customerUpdatedOn,
    confirmCustomerUpdatedDate: confirmation,
    ...(customerUpdatedOn ? { customerProposal: proposalView() } : {}),
    onHold,
    project: FAKE_PROJECTS.find((p) => p.id === scope.projectId),
    deployments: FAKE_DEPLOYMENTS.filter((d) => scope.deploymentIds.includes(d.id)).map(({ id, name }) => ({ id, name })),
    deploymentProducts: FAKE_DEPLOYMENT_PRODUCTS.filter((p) => scope.deploymentProductIds.includes(p.id)).map(({ id, name }) => ({ id, name })),
    customerContacts: currentContacts().map(({ id, name, email }) => ({ id, name, email })),
    category: scope.category,
  });

  /**
   * The backend's validation of a create / PATCH body's customer scope, applied to
   * `next` (the scope as it would be after the write). Returns the 400 message,
   * or null when the combination is consistent.
   */
  const validateScope = (
    body: Record<string, unknown>,
    next: FakeScope,
    isPatch: boolean,
  ): string | null => {
    // The removed fields are refused outright (any value, null included).
    if (body.customerGroupId !== undefined) return CUSTOMER_GROUP_ID_REMOVED;
    if (body.environmentIds !== undefined) return ENVIRONMENT_IDS_REMOVED;
    if (typeof body.category === "string" && !CATEGORIES.includes(body.category)) {
      return `category must be one of ${CATEGORIES.join(", ")}`;
    }
    if (isPatch) {
      // The Customer Project part is frozen far earlier (creationPhaseProblem); what
      // is left here is the deployments' own window, which closes at Implement.
      const touchesScope =
        (body.projectId !== undefined && body.projectId !== scope.projectId) ||
        (body.deploymentIds !== undefined && !sameSet(body.deploymentIds as string[], scope.deploymentIds));
      if (touchesScope && SCOPE_LOCKED.includes(state)) {
        return `projectId and deploymentIds can no longer be changed once the change request is ${state}`;
      }
      if (body.projectId !== undefined && body.projectId !== scope.projectId && scope.deploymentIds.length > 0 && body.deploymentIds === undefined) {
        return "changing projectId while deployments are stored requires deploymentIds in the same request";
      }
    }
    if (next.deploymentIds.length > 0 && !next.projectId) {
      return "deploymentIds requires projectId";
    }
    if (next.projectId && !FAKE_PROJECTS.some((p) => p.id === next.projectId)) {
      return `projectId: project ${next.projectId} not found`;
    }
    for (const id of next.deploymentIds) {
      const d = FAKE_DEPLOYMENTS.find((x) => x.id === id);
      if (!d) return `deploymentIds: deployment ${id} not found`;
      if (d.projectId !== next.projectId || retired.has(d.id)) {
        return `deploymentIds: deployment ${d.name} is not an active deployment of the selected project`;
      }
    }
    if (body.deploymentProductIds !== undefined && !sameSet(body.deploymentProductIds as string[], derivedProductIds(next.deploymentIds))) {
      return "deploymentProductIds: deployment products are derived from the selected deployments and must be exactly that set";
    }
    return null;
  };

  /** The scope after applying the scope fields of `body` on top of the stored one. */
  const applyScope = (body: Record<string, unknown>, base: FakeScope): FakeScope => {
    const next: FakeScope = { ...base, deploymentIds: [...base.deploymentIds] };
    if (body.projectId !== undefined) next.projectId = body.projectId as string;
    if (body.deploymentIds !== undefined) next.deploymentIds = body.deploymentIds as string[];
    next.deploymentProductIds = derivedProductIds(next.deploymentIds);
    if (body.category !== undefined) next.category = body.category as string | null;
    return next;
  };

  /**
   * The backend's creation-phase gate on a PATCH body (the customer's part of a
   * change request is fully editable only in New): the first refusal wins, in the
   * backend's order, and nothing is written when one applies. Resending a value
   * that is already stored is never a refusal.
   */
  const creationPhaseProblem = (body: Record<string, unknown>): string | null => {
    const inNew = state === "new";
    // 1. A change request that has left New cannot return to it.
    // (a FINAL change is refused as every other request to move it is, which says more than "cannot return to New")
    if (body.state === "new" && !inNew) return FINAL_STATES.includes(state) ? finalStateMessage("new", state) : CANNOT_RETURN_TO_NEW;
    // 2. The Customer Project is frozen once the change leaves New.
    if (!inNew && body.projectId !== undefined && body.projectId !== scope.projectId) {
      return projectFrozenMessage(state);
    }
    // 2b. An Emergency change cannot have a customer box turned on (a value it already holds is a no-op, accepted).
    if (type === "emergency") {
      const turnedOn = (["customerApprovalRequired", "customerReviewRequired"] as const).filter((field) => body[field] === true && !flags[field]);
      if (turnedOn.length > 0) return emergencyNoCustomerConsentMessage(turnedOn);
    }
    const boxes = [
      { field: "customerApprovalRequired", stored: flags.customerApprovalRequired, gateLocked: APPROVAL_FLAG_LOCKED },
      { field: "customerReviewRequired", stored: flags.customerReviewRequired, gateLocked: REVIEW_FLAG_LOCKED },
    ] as const;
    // 3. After New a customer requirement can be added but never removed.
    for (const box of boxes) {
      if (!inNew && box.stored && body[box.field] === false) return requirementCannotBeRemovedMessage(box.field, state);
    }
    // 4. ... and added only before its gate, and only with a Customer Project to ask.
    for (const box of boxes) {
      if (!inNew && !box.stored && body[box.field] === true) {
        if (box.gateLocked.includes(state)) return requirementGatePassedMessage(box.field, state);
        if (!scope.projectId) return requirementNeedsProjectMessage(box.field);
      }
    }
    // 4b. ... and a box turned on needs somebody on that project who can be asked (the refusal names the box(es) turned on).
    if (!inNew) {
      const approvalOn = !flags.customerApprovalRequired && body.customerApprovalRequired === true;
      const reviewOn = !flags.customerReviewRequired && body.customerReviewRequired === true;
      if ((approvalOn || reviewOn) && askableContacts(scope.projectId).length === 0) return nobodyToAskMessage(approvalOn, reviewOn);
    }
    // 6. Request Approval needs somebody to ask when the customer's part is required: a Customer Project, with at least one
    // registered contact other than the requester (the same people the customer stage would ask).
    if (body.state === "assess" && inNew) {
      const gates = effectiveGates();
      const approval = typeof body.customerApprovalRequired === "boolean" ? body.customerApprovalRequired : gates.customerApprovalRequired;
      const review = typeof body.customerReviewRequired === "boolean" ? body.customerReviewRequired : gates.customerReviewRequired;
      const project = typeof body.projectId === "string" && body.projectId ? body.projectId : scope.projectId;
      if ((approval || review) && !project) return REQUEST_APPROVAL_NEEDS_PROJECT;
      if ((approval || review) && askableContacts(project).length === 0) return nobodyToAskMessage(approval, review);
    }
    return null;
  };

  /**
   * The backend's manual state change (a PATCH body's `state`, already trimmed and lower-cased), in its order; returns the
   * 400 message, or null when it is allowed. Nothing is written when one applies.
   *
   *  1. The graph (`checkStaffStateRequest`): naming the state the change is in is a resend, no move; a FINAL state
   *     (Closed, Canceled, Rollback) has no exit, whatever the target, Cancel and a Re-schedule included; a customer state
   *     is left to 3.; the targets that have a refusal of their own in 2. are left to it; any other target must be a
   *     move staff may make from here (`staffTargets`), else it is a jump over a state and every approval gate on the way.
   *  2. The target-specific refusals: Request Approval only from New; `authorize` only as Re-schedule from Customer
   *     Approval, with a changed window; `customer_approval` and `scheduled` are never a manual choice; Roll back only from
   *     the two review states and not while the customer's review is live; Customer Review only when it is required, Close
   *     from Review only when it is not.
   *  3. A customer state is left only by the exits that answer nothing for the customer (`refuseStaffExitFromCustomerState`):
   *     Cancel, Re-schedule out of Customer Approval, Roll back out of Customer Review, or staying where it is. Every other
   *     destination is the customer's own answer (`customerAnswerRefusal`).
   */
  const OWN_REFUSAL = ["new", "assess", "authorize", "customer_approval", "scheduled", "rollback"];
  const manualStateRefusal = (target: string, body: Record<string, unknown>): string | null => {
    // The gate flags in effect are the ones this very request carries, else the stored ones.
    const gates = effectiveGates();
    const reviewRequired = typeof body.customerReviewRequired === "boolean" ? body.customerReviewRequired : gates.customerReviewRequired;
    const approvalRequired = typeof body.customerApprovalRequired === "boolean" ? body.customerApprovalRequired : gates.customerApprovalRequired;
    const inCustomerGate = state === "customer_approval" || state === "customer_review";
    // 1.
    if (target !== state) {
      if (FINAL_STATES.includes(state)) return finalStateMessage(target, state);
      if (!inCustomerGate && !OWN_REFUSAL.includes(target) && !staffTargets(state).includes(target)) {
        return stateJumpMessage(target, state, reviewRequired);
      }
    }
    // 2.
    switch (target) {
      case "assess": {
        // Where Request Approval lands, by the change's type: a resend of it from there is no move.
        const destination = type === "standard" ? (approvalRequired ? "customer_approval" : "scheduled") : type === "emergency" ? "authorize" : "assess";
        if (state !== "new" && state !== destination) return "approval can only be requested for a change request in the New state";
        break;
      }
      case "authorize": {
        // Out of Customer Approval only: the wire name of the Time Change loop (the state never moves). What it asks is judged by
        // `timeChangeRefusal`, which says more than "not an edge" (the proposal's version, the window, who can be asked).
        if (state !== "customer_approval") {
          return 'state "authorize" cannot be set manually: it is reached automatically through the approval flow (Request Approval, then peer approval); it can only be set by hand to re-schedule a change from customer_approval';
        }
        break;
      }
      case "customer_approval":
        return 'state "customer_approval" cannot be set manually: it is reached automatically through the approval flow when customerApprovalRequired is set';
      case "scheduled":
        // The customer's approval is theirs to give, in the Customer Portal (3.); from anywhere else it is reached by the flow.
        if (state === "customer_approval") break;
        return 'state "scheduled" cannot be set manually: it is reached automatically through the approval flow (Request Approval, then peer/CAB approval), or by the customer\'s own approval from customer_approval';
      case "rollback":
        if (state !== "review" && state !== "customer_review") return 'state "rollback" can only be set from review or customer_review';
        if (state === "customer_review" && hasLiveCustomerStage()) return customerStageManualRefusal("rollback");
        break;
      case "customer_review":
        if (!reviewRequired) {
          return 'state "customer_review" cannot be set: customer review is not required for this change request (customerReviewRequired is false); close it from review instead';
        }
        break;
      case "closed":
        if (state === "review" && reviewRequired) {
          return 'state "closed" cannot be set from review: customer review is required for this change request (customerReviewRequired is true); move it to customer_review first';
        }
        break;
    }
    // 3.
    if ((state === "customer_approval" || state === "customer_review") && target !== state && target !== "canceled") {
      const exit = state === "customer_approval" ? "authorize" : "rollback";
      if (target !== exit) return customerAnswerRefusal(target, hasLiveCustomerStage(), state);
    }
    return null;
  };

  /** A refusal with its HTTP status (the answers to a customer's proposed time are 400s and 409s). */
  interface Refusal {
    status: 400 | 409;
    message: string;
    /** The stable `errorCode` the backend names the refusal by, when it names one (entity-service `apierror`). */
    code?: string;
  }
  /** A refusal's body: the words, and the code when there is one. */
  const refusalBody = (refused: Refusal): { message: string; errorCode?: string } => ({
    message: refused.message,
    ...(refused.code ? { errorCode: refused.code } : {}),
  });
  /** The planned window as the backend words it in a stale-window refusal (RFC 3339 bounds). */
  const plannedNow = (): string => {
    const s = instantOf(plannedStartOn);
    const e = instantOf(plannedEndOn);
    return s === null && e === null ? "no planned time is set" : `${s === null ? "not set" : rfc3339Of(s)} to ${e === null ? "not set" : rfc3339Of(e)}`;
  };
  /** The window the staff request says it was shown must still be the planned one (checkExpectedSchedule). */
  const staleWindow = (body: Record<string, unknown>): Refusal | null => {
    const es = typeof body.expectedPlannedStartOn === "string" ? instantOf(body.expectedPlannedStartOn) : null;
    const ee = typeof body.expectedPlannedEndOn === "string" ? instantOf(body.expectedPlannedEndOn) : null;
    if ((es !== null && es !== instantOf(plannedStartOn)) || (ee !== null && ee !== instantOf(plannedEndOn))) {
      return { status: 409, message: windowChangedMessage(plannedNow()), code: "change_request_schedule_changed" };
    }
    return null;
  };
  /**
   * WSO2's ACCEPT (`PATCH {confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn, expectedPlannedStartOn,
   * expectedPlannedEndOn}`: all three expectations required), refusals in the backend's order: the shape, the state, the allowlisted pending proposal, the version of the proposal, the window it was shown, on hold, a proposed
   * time that has passed, a planned window with no length.
   */
  const acceptRefusal = (body: Record<string, unknown>): Refusal | null => {
    if (body.confirmCustomerUpdatedDate !== "agree") return { status: 400, message: ACCEPT_ONLY_AGREE };
    const allowed = ["confirmCustomerUpdatedDate", "expectedCustomerUpdatedOn", "expectedPlannedStartOn", "expectedPlannedEndOn"];
    if (Object.keys(body).some((k) => !allowed.includes(k))) return { status: 400, message: ACCEPT_CANNOT_COMBINE };
    if (typeof body.expectedCustomerUpdatedOn !== "string") return { status: 400, message: ACCEPT_NEEDS_EXPECTED };
    // The planned window the page showed is REQUIRED (the CSM page always has it): a stale one is a 409, never a blind accept.
    if (typeof body.expectedPlannedStartOn !== "string" || typeof body.expectedPlannedEndOn !== "string") {
      return { status: 400, message: ACCEPT_NEEDS_EXPECTED_WINDOW };
    }
    if (state !== "customer_approval") return { status: 409, message: acceptNotInCustomerApproval(state), code: "change_request_not_proposable" };
    if (!proposalPending()) return { status: 409, message: NO_PROPOSAL_WAITING };
    if (instantOf(body.expectedCustomerUpdatedOn) !== instantOf(customerUpdatedOn)) {
      return { status: 409, message: proposalChangedMessage(customerUpdatedOn!) };
    }
    const stale = staleWindow(body);
    if (stale) return stale;
    // No staff action stands in for the customer's answer: a registered contact must be recorded as the proposer.
    if (!proposer) return { status: 409, message: ACCEPT_PROPOSER_NOT_RECORDED, code: "change_request_proposer_not_recorded" };
    if (onHold) return { status: 400, message: ON_HOLD_MESSAGE };
    const start = instantOf(customerUpdatedOn)!;
    if (start <= Date.now()) return { status: 409, message: proposalPassedMessage(customerUpdatedOn!) };
    const ps = instantOf(plannedStartOn);
    const pe = instantOf(plannedEndOn);
    if (ps === null || pe === null || pe <= ps) return { status: 409, message: ACCEPT_WINDOW_HAS_NO_LENGTH };
    if (new Date(start + (pe - ps)).getUTCFullYear() > 2100) {
      return { status: 409, message: acceptTooFarAheadMessage(rfc3339Of(start), rfc3339Of(start + (pe - ps))) };
    }
    return null;
  };
  /**
   * A time is stored and unanswered (`proposalPending`) AND a registered contact is recorded as having proposed it: a customer's
   * PROPOSAL waiting for WSO2's answer. A stored time nobody is recorded as having proposed (a WSO2 user's, or one left over from an
   * earlier cycle) is never answered: a staff request is a plain Re-schedule about it.
   */
  const proposalWaits = (): boolean => proposalPending() && !!proposer;
  /**
   * `{state: "authorize"}` out of Customer Approval, after the graph accepted it: a plain Re-schedule when no proposal waits (the
   * window must change), WSO2's COUNTER or DECLINE when one does (it must carry the proposal it answers; the window may equal the
   * plan, which declines, but never the customer's own time, which is Accept). A stored time nobody is recorded as having proposed
   * is no proposal: the request is a plain Re-schedule (it may name the stored time it was shown; with no window it is refused, there
   * is nothing to decline, never a silent Disagree). Nobody to ask is refused before anything is written, with the words of Request
   * Approval -- except a decline, which touches no request.
   */
  const timeChangeRefusal = (body: Record<string, unknown>): Refusal | null => {
    const stored = proposalPending();
    const pending = proposalWaits();
    const expected = typeof body.expectedCustomerUpdatedOn === "string" ? body.expectedCustomerUpdatedOn : null;
    if (pending && expected === null) return { status: 409, message: customerProposedWhileOpenMessage(customerUpdatedOn!) };
    if (expected !== null && !stored) return { status: 409, message: PROPOSAL_NO_LONGER_WAITING };
    if (expected !== null && instantOf(expected) !== instantOf(customerUpdatedOn)) {
      return { status: 409, message: pending ? proposalChangedMessage(customerUpdatedOn!) : storedTimeChangedMessage(customerUpdatedOn!) };
    }
    const stale = staleWindow(body);
    if (stale) return stale;
    const newStart = typeof body.plannedStartOn === "string" ? body.plannedStartOn : undefined;
    const newEnd = typeof body.plannedEndOn === "string" ? body.plannedEndOn : undefined;
    const changed =
      (newStart !== undefined && instantOf(newStart) !== instantOf(plannedStartOn)) || (newEnd !== undefined && instantOf(newEnd) !== instantOf(plannedEndOn));
    const effStart = instantOf(newStart ?? plannedStartOn) ?? 0;
    const effEnd = instantOf(newEnd ?? plannedEndOn) ?? 0;
    if (stored && !pending && newStart === undefined && newEnd === undefined) {
      return { status: 400, message: NO_RECORDED_PROPOSAL_TO_DECLINE };
    }
    if (!pending && !changed) {
      return {
        status: 400,
        message: "re-scheduling requires a changed planned start or end: send plannedStartOn and/or plannedEndOn with a value different from the stored one",
      };
    }
    // Pending with no window at all is a decline (nothing to judge); a window is judged whole: usable first, then not the customer's own.
    if (!pending || newStart !== undefined || newEnd !== undefined) {
      if (effStart > effEnd) return { status: 400, message: "the planned start must not be after the planned end" };
      if (effStart === effEnd) return { status: 400, message: "the planned start must not be the same as the planned end: the window must have a duration" };
    }
    if (pending && (newStart !== undefined || newEnd !== undefined)) {
      const proposedEnd = instantOf(proposalView()?.endOn as string | undefined);
      if (effStart === instantOf(customerUpdatedOn) && (proposedEnd === null || effEnd === proposedEnd)) {
        return { status: 400, message: COUNTER_IS_THE_PROPOSAL };
      }
    }
    if ((changed || !pending) && askableContacts(scope.projectId).length === 0) return { status: 400, message: nobodyToAskMessage(true, false) };
    return null;
  };

  const cors = (route: Route): Record<string, string> => ({
    "access-control-allow-origin": route.request().headers()["origin"] ?? "*",
    "access-control-allow-headers": "*",
    "access-control-allow-methods": "GET,POST,PATCH,PUT,DELETE,OPTIONS",
    "access-control-allow-credentials": "true",
  });
  const json = (route: Route, body: unknown, status = 200): Promise<void> =>
    route.fulfill({
      status,
      contentType: "application/json",
      headers: cors(route),
      body: JSON.stringify(body),
    });

  // Same identity swap for /users/me: keep the real profile (roles, time
  // zone, ...) but make the signed-in user one of the fake people above.
  await page.route(
    (url) => url.pathname.endsWith("/users/me"),
    async (route) => {
      const type = route.request().resourceType();
      if (route.request().method() !== "GET" || (type !== "fetch" && type !== "xhr")) return route.fallback();
      try {
        const real = await route.fetch();
        const profile = (await real.json()) as Record<string, unknown>;
        const [firstName, ...rest] = currentViewer.name.split(" ");
        await route.fulfill({
          response: real,
          json: { ...profile, id: currentViewer.id, email: currentViewer.email, firstName, lastName: rest.join(" ") },
        });
      } catch {
        // The test can end (the page and its context close, and the pending response is disposed)
        // while this is still fetching the live profile from the BFF, which is not an error of the
        // test. There is nobody left to answer then; if the page is still there, hand the request on
        // to the network as it is.
        await route.fallback().catch(() => undefined);
      }
    },
  );

  /** Common prologue for the endpoints below: only XHR/fetch, answers CORS preflights. */
  const isApiCall = async (route: Route): Promise<boolean> => {
    const req = route.request();
    const rtype = req.resourceType();
    if (rtype !== "fetch" && rtype !== "xhr") {
      await route.fallback();
      return false;
    }
    if (req.method() === "OPTIONS") {
      await route.fulfill({ status: 204, headers: cors(route) });
      return false;
    }
    return true;
  };
  const bodyOf = (route: Route): Record<string, unknown> | undefined => {
    try {
      return (route.request().postDataJSON() as Record<string, unknown> | null) ?? undefined;
    } catch {
      return undefined;
    }
  };
  const record = (route: Route, label: string): Record<string, unknown> | undefined => {
    log.push(label);
    const body = bodyOf(route);
    bodies.push({ request: label, body });
    return body;
  };

  // Customer Project picker.
  await page.route(
    (url) => url.pathname.endsWith("/projects/search"),
    async (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      if (!(await isApiCall(route))) return;
      const body = record(route, "POST /projects/search");
      const q = String((body?.searchQuery as string | undefined) ?? "").toLowerCase();
      const projects = FAKE_PROJECTS.filter((p) => p.name.toLowerCase().includes(q));
      return json(route, { projects, hasMore: false, totalRecords: projects.length });
    },
  );

  // Assignment group picker (the Customer Group is derived, so it is not searched).
  await page.route(
    (url) => url.pathname.endsWith("/groups/search"),
    async (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      if (!(await isApiCall(route))) return;
      const body = record(route, "POST /groups/search");
      const q = String(((body?.filters as { searchQuery?: string } | undefined)?.searchQuery) ?? "").toLowerCase();
      const groups = FAKE_GROUPS.filter((g) => g.name.toLowerCase().includes(q)).map((g) => ({ ...g, active: true }));
      return json(route, { groups, total: groups.length, limit: 20, offset: 0 });
    },
  );

  // One group and its members: what opens from a stage's Assignment group.
  await page.route(
    (url) => /\/groups\/[0-9a-f-]{36}$/.test(url.pathname),
    async (route) => {
      if (route.request().method() !== "GET") return route.fallback();
      if (!(await isApiCall(route))) return;
      const id = new URL(route.request().url()).pathname.split("/").pop()!;
      record(route, `GET /groups/${id}`);
      if (groupFailure !== null) return json(route, { message: "Failed to retrieve group." }, groupFailure);
      const group = FAKE_APPROVAL_GROUPS.find((g) => g.id === id);
      if (!group) return json(route, { message: "Not found." }, 404);
      return json(route, {
        id: group.id,
        name: group.name,
        description: group.description,
        email: group.email,
        manager: group.manager,
        members: group.members.map((m) => ({ id: m.id, name: m.name, email: m.email || null, userType: "INTERNAL", role: m.role })),
        total: group.members.length,
      });
    },
  );

  // The Customer Project -> Deployments -> Deployment products cascade, plus the project's contacts.
  await page.route(
    (url) => url.pathname.endsWith("/change-requests/link-options"),
    async (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      if (!(await isApiCall(route))) return;
      const body = record(route, "POST /change-requests/link-options") ?? {};
      const projectId = body.projectId as string | undefined;
      if (!projectId) return json(route, { message: "projectId is required" }, 400);
      if (!FAKE_PROJECTS.some((p) => p.id === projectId)) {
        return json(route, { message: `projectId does not refer to an existing project: ${projectId}` }, 400);
      }
      const chosen = (body.deploymentIds as string[] | undefined) ?? [];
      const projectDeployments = FAKE_DEPLOYMENTS.filter((d) => d.projectId === projectId && !retired.has(d.id));
      const stray = chosen.find((id) => !projectDeployments.some((d) => d.id === id));
      if (stray) return json(route, { message: `deployment ${stray} does not belong to the selected project` }, 400);
      return json(route, {
        deployments: projectDeployments.map((d) => ({
          id: d.id,
          name: d.name,
          type: d.type,
        })),
        customerContacts: (contacts.get(projectId) ?? []).map(({ id, name, email }) => ({ id, name, email })),
        deploymentProducts: FAKE_DEPLOYMENT_PRODUCTS.filter((p) => chosen.includes(p.deploymentId)).map((p) => ({
          id: p.id,
          name: p.name,
          deployment: FAKE_DEPLOYMENTS.filter((d) => d.id === p.deploymentId).map(({ id, name }) => ({ id, name }))[0],
        })),
      });
    },
  );

  // Create. The fake holds exactly one CR (FAKE_CR_ID); creating "creates" it.
  await page.route(
    (url) => url.pathname.endsWith("/change-requests"),
    async (route) => {
      if (route.request().method() !== "POST") return route.fallback();
      if (!(await isApiCall(route))) return;
      const body = record(route, "POST /change-requests") ?? {};
      if (!["normal", "standard", "emergency"].includes(body.type as string)) {
        return json(route, { message: "type is required: a change request must be one of standard, normal or emergency" }, 400);
      }
      const next = applyScope(body, scope);
      const problem = validateScope(body, next, false);
      if (problem) return json(route, { message: problem }, 400);
      // An Emergency change cannot be created with a customer box ticked.
      if (body.type === "emergency") {
        const ticked = (["customerApprovalRequired", "customerReviewRequired"] as const).filter((field) => body[field] === true);
        if (ticked.length > 0) return json(route, { message: emergencyNoCustomerConsentMessage(ticked) }, 400);
      }
      Object.assign(scope, next);
      type = body.type as FakeCrType;
      state = "new";
      subject = String(body.subject ?? subject);
      if (body.customerApprovalRequired !== undefined) flags.customerApprovalRequired = body.customerApprovalRequired as boolean;
      if (body.customerReviewRequired !== undefined) flags.customerReviewRequired = body.customerReviewRequired as boolean;
      for (const kind of ["comment", "workNote"] as const) {
        const text = body[kind];
        if (typeof text === "string" && text.trim()) journal.push({ kind, text });
      }
      return json(
        route,
        { message: "Change request created.", changeRequest: { id: FAKE_CR_ID, number: "CHG0099001", createdOn: "2026-01-01T00:00:00Z", createdBy: currentViewer.email } },
        201,
      );
    },
  );

  await page.route(
    (url) => new RegExp(`/change-requests/${FAKE_CR_ID}(/.*)?$`).test(url.pathname),
    async (route) => {
      const req = route.request();
      const rtype = req.resourceType();
      if (rtype !== "fetch" && rtype !== "xhr") return route.fallback();
      if (req.method() === "OPTIONS") return route.fulfill({ status: 204, headers: cors(route) });

      const path = new URL(req.url()).pathname.replace(/^.*\/change-requests\//, "/change-requests/");
      record(route, `${req.method()} ${path}`);

      if (path.endsWith("/approvals/decision") && req.method() === "POST") {
        const { decision } = req.postDataJSON() as { decision: "approved" | "rejected" };
        // The caller's pending stage: their row on a stage decidable in the CR's
        // current state, else (all of theirs are stale) the first one. The customer
        // stages are not the CSM portal's to decide (see customerDecides).
        const mine = stages.filter(
          (s) =>
            !CUSTOMER_STAGES.includes(s.stage) &&
            s.status === "REQUESTED" &&
            s.approvers.some((a) => a.id === currentViewer.id && a.status === "REQUESTED"),
        );
        const current = mine.find((s) => !stageOutOfState(s)) ?? mine[0];
        const row = current?.approvers.find((a) => a.id === currentViewer.id && a.status === "REQUESTED");
        if (!current || !row || currentViewer.id === FAKE_CREATOR.id) {
          // A change waiting on its live customer stage says so, like the backend.
          const waiting = stages.find((s) => CUSTOMER_STAGES.includes(s.stage) && s.status === "REQUESTED" && !stageOutOfState(s));
          return json(
            route,
            {
              message: waiting
                ? `only members of the customer group (the registered contacts of this change request's project) can approve or reject the customer's ${waiting.stage === "Customer Approval" ? "approval" : "review"} of this change request`
                : "Access to the requested resource is forbidden!",
            },
            403,
          );
        }
        if (stageOutOfState(current)) {
          // Nothing is changed: the approval is no longer pending.
          return json(
            route,
            {
              message: `this approval is no longer pending: the change request is in ${stateName(state)}, but the ${current.stage} stage can only be decided while it is in ${stateName(STAGE_STATE[current.stage]!)}`,
            },
            409,
          );
        }
        row.status = decision === "approved" ? "APPROVED" : "REJECTED";
        current.status = row.status;
        // Like the backend, a resolving decision cancels the stage's other pending approvers.
        for (const a of current.approvers) if (a !== row && a.status === "REQUESTED") a.status = "CANCELLED";
        if (decision === "approved") {
          if (current.stage === "Peer Approval") {
            state = "authorize";
            stages = [...stages, nextStage("CAB Approval", FAKE_CAB_GROUP, FAKE_CAB)];
          } else if (current.stage === "CAB Approval" || current.stage === "ECAB Approval") {
            enter(afterInternalApproval()); // CAB approval (an older Emergency change's ECAB one too) moves the CR on itself
          }
          // Review: the answer is recorded, the CR stays in review (a human moves it on).
        }
        reconcile();
        return json(route, { id: FAKE_CR_ID, state });
      }
      if (path.endsWith("/approvals") && req.method() === "GET") {
        // Like the Postgres-backed API: `canDecide` is true only on the
        // caller's own REQUESTED row, and never for the CR's creator.
        return json(route, {
          approvals: stages.map((st) => ({
            ...st,
            approvers: st.approvers.map((a) => ({
              ...a,
              canDecide:
                st.status === "REQUESTED" &&
                a.id === currentViewer.id &&
                a.status === "REQUESTED" &&
                currentViewer.id !== FAKE_CREATOR.id &&
                !stageOutOfState(st),
            })),
          })),
        });
      }
      if (path.endsWith("/comments") && req.method() === "POST") {
        const { content } = req.postDataJSON() as { content?: string };
        if (typeof content === "string" && content.trim()) journal.push({ kind: "comment", text: content });
        return json(route, { id: "00000000-0000-0000-0000-00000000d001", content, type: "comment", createdOn: "2026-01-01T00:00:00Z", createdBy: null }, 201);
      }
      if (path.endsWith("/comments/search")) {
        return json(route, { comments: [], hasMore: false, totalRecords: 0 });
      }
      if (req.method() === "PATCH") {
        const body = req.postDataJSON() as {
          state?: string;
          customerApprovalRequired?: boolean;
          customerReviewRequired?: boolean;
        } & Record<string, unknown>;
        // The requested state is read the way the table of moves is written -- trimmed, lower case -- and a value that is not
        // a state of the lifecycle is refused before anything else (entity-service `normalizeRequestedChangeRequestState`).
        if (body.state !== undefined && body.state !== null) {
          const asked = typeof body.state === "string" ? body.state.trim().toLowerCase() : "";
          if (!ALL_STATES.includes(asked)) return json(route, { message: notAStateMessage(String(body.state)) }, 400);
          body.state = asked;
        }
        // WSO2's ACCEPT of the customer's proposed time (the previous system's "Agree"): a PATCH of its own, never a `state`. One step: the
        // proposal becomes the planned window (the planned length kept), the answer is agree and the change is Scheduled. No CAB, no new
        // customer request (the customer's own request is closed like any state change closes it), and the customer's outcome
        // (`hasCustomerApproved`) is NOT stamped: no staff action records the customer's approval.
        if (body.confirmCustomerUpdatedDate !== undefined) {
          const refused = acceptRefusal(body);
          if (refused) return json(route, refusalBody(refused), refused.status);
          const start = instantOf(customerUpdatedOn)!;
          const length = instantOf(plannedEndOn)! - instantOf(plannedStartOn)!;
          plannedStartOn = plannedOf(start);
          plannedEndOn = plannedOf(start + length);
          confirmation = "agree";
          state = "scheduled"; // not `enter`: that would stamp the customer's approval
          reconcile();
          return json(route, { id: FAKE_CR_ID, state, message: "Change request updated.", changeRequest: detail() });
        }
        // The on-hold gate: a state change is refused while the change is on hold (unless the same request takes it off hold).
        if (typeof body.state === "string" && onHold && body.onHold !== false) return json(route, { message: ON_HOLD_MESSAGE }, 400);
        if (typeof body.onHold === "boolean") onHold = body.onHold;
        // The creation-phase gate: nothing below is written when it refuses.
        const gateProblem = creationPhaseProblem(body);
        if (gateProblem) return json(route, { message: gateProblem }, 400);
        // A refused state change writes nothing either, not even the rest of its own request (a comment, a scope field,
        // a tick box): the backend refuses inside the one transaction.
        const refusal = typeof body.state === "string" ? manualStateRefusal(body.state, body) : null;
        if (refusal) return json(route, { message: refusal }, 400);
        // The Time Change loop out of Customer Approval (a Re-schedule, or WSO2's counter / decline of a proposal).
        if (body.state === "authorize" && state === "customer_approval") {
          const refused = timeChangeRefusal(body);
          if (refused) return json(route, refusalBody(refused), refused.status);
        }
        // Customer scope / category (and the removed customerGroupId / environmentIds,
        // which are refused), validated like the backend.
        const touchesScopeFields = ["projectId", "deploymentIds", "environmentIds", "deploymentProductIds", "customerGroupId", "category"].some(
          (k) => body[k] !== undefined,
        );
        if (touchesScopeFields) {
          const next = applyScope(body, scope);
          const problem = validateScope(body, next, true);
          if (problem) return json(route, { message: problem }, 400);
          Object.assign(scope, next);
          // A project written (even the stored one again: an accepted no-op) while the
          // CR already sits at a customer gate (re)provisions the stage for that
          // project's contacts, like the backend.
          if (body.projectId !== undefined) syncCustomerStage();
        }
        for (const kind of ["comment", "workNote"] as const) {
          const text = body[kind];
          if (typeof text === "string" && text.trim()) journal.push({ kind, text });
        }
        // Checkbox edits: the gate above has refused every one that is not allowed.
        if (typeof body.customerApprovalRequired === "boolean") flags.customerApprovalRequired = body.customerApprovalRequired;
        if (typeof body.customerReviewRequired === "boolean") flags.customerReviewRequired = body.customerReviewRequired;
        const target = body.state;
        if (target === undefined) {
          return json(route, { id: FAKE_CR_ID, state, message: "Change request updated.", changeRequest: detail() });
        }
        // (Already accepted by `manualStateRefusal` above.) Naming the state the change is in is a resend: no move.
        if (target === state) {
          // nothing to do
        } else if (target === "rollback") {
          // Rolling back cancels every still-requested approver row (the closing reconcile).
          state = "rollback";
        } else if (target === "authorize") {
          // The Time Change loop out of Customer Approval (all refusals are above): the state NEVER moves and no CAB stage is
          // opened, whatever the type -- the change itself has not changed. A changed window asks the customer again (their pending
          // request is superseded: rows cancelled, the stage kept as a record and reported PENDING, and a fresh stage provisioned). A
          // proposal that was waiting is answered Disagree; with the window as it is, that is all that is written (a decline: the
          // customer keeps their live request). Never the stored `customerApprovalRequired`.
          const answeringProposal = proposalWaits(); // a stored time nobody proposed is never answered
          const newStart = typeof body.plannedStartOn === "string" ? body.plannedStartOn : undefined;
          const newEnd = typeof body.plannedEndOn === "string" ? body.plannedEndOn : undefined;
          const changed =
            (newStart !== undefined && instantOf(newStart) !== instantOf(plannedStartOn)) || (newEnd !== undefined && instantOf(newEnd) !== instantOf(plannedEndOn));
          if (changed) {
            plannedStartOn = newStart ?? plannedStartOn;
            plannedEndOn = newEnd ?? plannedEndOn;
            askCustomersAgain();
          }
          if (answeringProposal) confirmation = "disagree";
        } else if (target === "assess") {
          if (type === "standard") enter(afterInternalApproval());
          else if (type === "emergency") {
            // One stage, in the existing CAB group: there is no ECAB, no Peer stage and no Assess.
            state = "authorize";
            stages = [nextStage("CAB Approval", FAKE_CAB_GROUP, FAKE_CAB)];
          } else {
            state = "assess";
            stages = [nextStage("Peer Approval", FAKE_PEER_GROUP, FAKE_PEER)];
          }
        } else {
          enter(target); // implement, review, customer_review, closed, canceled: each an edge of the state machine
        }
        reconcile();
        return json(route, { id: FAKE_CR_ID, state });
      }
      if (req.method() === "GET" && path === `/change-requests/${FAKE_CR_ID}`) {
        return json(route, detail());
      }
      return route.fallback();
    },
  );

  return {
    setViewer: (user) => {
      currentViewer = user;
    },
    state: () => state,
    setState: (next) => {
      state = next;
    },
    flags: () => ({ ...flags }),
    requests: () => [...log],
    requestBodies: () => [...bodies],
    scope: () => ({ ...scope, deploymentIds: [...scope.deploymentIds], deploymentProductIds: [...scope.deploymentProductIds] }),
    journal: () => [...journal],
    planned: () => ({ start: plannedStartOn, end: plannedEndOn }),
    retireDeployment: (deploymentId) => {
      retired.add(deploymentId);
    },
    setProjectContacts: (projectId, users) => {
      contacts.set(projectId, [...users]);
    },
    failGroups: (status) => {
      groupFailure = status;
    },
    customerDecides: (contact, decision) => settleCustomerStage(contact, decision),
    customerProposes: (contact, startOn) => {
      const live = stages.find(
        (st) =>
          CUSTOMER_STAGES.includes(st.stage) &&
          st.status === "REQUESTED" &&
          !stageOutOfState(st) &&
          st.approvers.some((a) => a.id === contact.id && a.status === "REQUESTED"),
      );
      if (state !== "customer_approval") throw new Error(`${contact.name} cannot propose: the change request is no longer in Customer Approval`);
      if (!live) throw new Error(`${contact.name} has no pending customer approval on this change request`);
      const start = instantOf(startOn);
      if (start === null || start <= Date.now()) throw new Error("a proposed time must be a start in the future");
      if (start === instantOf(plannedStartOn)) throw new Error("plannedStartOn is the planned start already: propose a different start");
      customerUpdatedOn = rfc3339Of(start);
      confirmation = null;
      proposer = contact;
    },
    seedProposal: ({ startOn, proposerKnown = false, confirmation: answer = null }) => {
      const start = instantOf(startOn);
      if (start === null) throw new Error(`not a time: ${startOn}`);
      customerUpdatedOn = rfc3339Of(start);
      confirmation = answer;
      proposer = proposerKnown ? FAKE_CUST_ONE : null;
    },
    proposal: () => ({ customerUpdatedOn, confirmation }),
    setOnHold: (next) => {
      onHold = next;
    },
    moveWindow: (start, end) => {
      plannedStartOn = start;
      plannedEndOn = end;
    },
    addInternalRequestedRow: () => {
      // An internal approval still being asked, in a group that is not the customer's: not one of ours, never reconciled away.
      stages = [
        ...stages,
        { stage: "Devops Approval", approverType: "STATIC_GROUP", approverName: "Devops Approval", assignmentGroup: null, status: "REQUESTED", approvers: [{ id: FAKE_CAB.id, name: FAKE_CAB.name, status: "REQUESTED" }] },
      ];
    },
    customerApproved: () => customerApproved,
    syncCustomers: () => {
      syncCustomerStage();
      reconcile();
    },
    startAtState: (next) => {
      const settled = (name: string, group: FakeApprovalGroup, who: FakeUser): Stage => ({
        ...nextStage(name, group, who),
        status: "APPROVED",
        approvers: [{ id: who.id, name: who.name, status: "APPROVED" }],
      });
      const internal: Stage[] =
        type === "normal"
          ? [settled("Peer Approval", FAKE_PEER_GROUP, FAKE_PEER), settled("CAB Approval", FAKE_CAB_GROUP, FAKE_CAB)]
          : type === "emergency"
            ? // An OLDER Emergency change: its one internal stage was still named ECAB (see `startOlderEmergencyAtAuthorize`).
              [settled("ECAB Approval", FAKE_ECAB_GROUP, FAKE_ECAB)]
            : [];
      // The Review stage a Normal change had before it was sent to the customer's review.
      if (next === "customer_review" && type === "normal") internal.push(settled("Review", FAKE_PEER_GROUP, FAKE_PEER));
      stages = internal;
      state = next;
      provisionReview(); // the Review stage of a Normal change that sits in Review (still to be decided)
    },
    startOlderEmergencyAtAuthorize: () => {
      type = "emergency";
      stages = [nextStage("ECAB Approval", FAKE_ECAB_GROUP, FAKE_ECAB)];
      state = "authorize";
    },
    stages: () =>
      stages.map((st) => ({
        stage: st.stage,
        status: st.status,
        approvers: st.approvers.map((a) => ({ name: a.name, status: a.status })),
      })),
  };
}
