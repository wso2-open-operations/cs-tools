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
  AdapterDateFns,
  Alert,
  Box,
  Button,
  Checkbox,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  FormHelperText,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { useCallback, useMemo, useState, type JSX } from "react";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchInternalUsersByName } from "@api/useSearchUsersByName";
import { useSearchServiceRequestsForSelect } from "@features/csm-operations/api/useSearchServiceRequestsForSelect";
import type {
  BeChangeRequestCategory,
  BeChangeRequestDetail,
  BeChangeRequestImpact,
  BeChangeRequestPriority,
  BeChangeRequestType,
  BeGroup,
  BeCaseSearchView,
  BePatchChangeRequestPayload,
  BeUser,
} from "@api/backend/types";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import ChangeRequestScopeFields, {
  ChangeRequestCustomerGroupField,
} from "@features/csm-operations/components/ChangeRequestScopeFields";
import { useChangeRequestScope } from "@features/csm-operations/hooks/useChangeRequestScope";
import CarriedOverDeviationField from "@features/csm-operations/components/CarriedOverDeviationField";
import ExpandableRichTextField from "@features/csm-operations/components/ExpandableRichTextField";
import {
  backendUtcToZonedInput,
  formatDateTimeLocal,
  isPastZonedInput,
  parseDateTimeLocal,
  zonedInputToBackendUtc,
} from "@utils/dateTime";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { userLabel } from "@features/csm-operations/utils/incidentFormOptions";
import {
  CHANGE_REQUEST_CATEGORY_OPTIONS,
  CHANGE_REQUEST_CREATE_TYPE_OPTIONS,
  CHANGE_REQUEST_ROLLBACK_DURATION_MAX,
  CHANGE_REQUEST_IMPACT_OPTIONS,
  CHANGE_REQUEST_PRIORITY_OPTIONS,
  CHANGE_REQUEST_TYPE_FROZEN_REASON,
  changeRequestApprovedNotice,
  changeRequestApprovedReadOnlyReason,
  changeRequestCategoryValue,
  changeRequestImpactValue,
  changeRequestPriorityValue,
  changeRequestScopeLockedReason,
  customerApprovalLockedReason,
  customerProjectLockedReason,
  customerRequirementOnceSavedHelper,
  customerProjectNeededForApproval,
  customerReviewLockedReason,
  EMERGENCY_CUSTOMER_STEPS_HELPER,
  implementationDeviationClearable,
  implementationDeviationLockedReason,
  implementationDeviationOffered,
  isChangeRequestCreationPhase,
  isEmergencyChangeRequestType,
  parentRecordLabel,
  REQUIRED_TO_REQUEST_APPROVAL_HELPER,
  STANDARD_TEMPLATE_FIELD_LOCKED_REASON,
  STANDARD_TEMPLATE_TYPE_LOCKED_REASON,
} from "@features/csm-operations/utils/changeRequests";

const { DateTimePicker, LocalizationProvider } = DatePickers;

/** The one line saying why an Emergency change's two customer boxes are off (their `aria-describedby`). */
const EMERGENCY_NOTE_ID = "cr-edit-customer-steps-emergency-note";

interface EditChangeRequestDialogProps {
  cr: BeChangeRequestDetail;
  /** True while the PATCH is in flight; disables the actions. */
  isSaving: boolean;
  /**
   * User-facing message for the most recent failed save, if any. Rendered
   * inline in the dialog so the rejection is visible even if a page-level
   * error banner is occluded or the dialog is otherwise the only thing the
   * user is looking at.
   */
  saveError?: string | null;
  onClose: () => void;
  /** Submit only the changed fields (`PATCH /change-requests/{id}`). */
  onSave: (patch: BePatchChangeRequestPayload) => void;
  /**
   * `"dialog"` (default): the original modal. `"inline"`: the same form, the
   * same fields and the same edit rules, rendered in the page itself at full
   * width (two columns from `md` up, long-form plans across both) — the change
   * request page uses this so its fields aren't squeezed into a narrow modal.
   */
  variant?: "dialog" | "inline";
}

/** The short hint under each field a standard change template specifies (the full reason is in the notice above them). */
const TEMPLATE_FIELD_HINT = "Set by the standard change template.";

/** The create form's subject cap. */
const SUBJECT_MAX = 500;

/** A detail response's type as one of the create form's three, or "" (a legacy type the form does not offer). */
function creatableTypeValue(type: string | null | undefined): BeChangeRequestType | "" {
  const lower = type?.trim().toLowerCase() ?? "";
  return CHANGE_REQUEST_CREATE_TYPE_OPTIONS.find((o) => o.value === lower)?.value ?? "";
}

/** Grid placement for a field that should span both columns in the inline
 * layout. In the dialog's single flex column it has no effect. */
const FULL_ROW = { gridColumn: "1 / -1" } as const;

/** Same members, order ignored. */
function sameIds(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((id) => b.includes(id));
}

/** One long-form plan field, edited as rich text. */
interface RichTextPlanField {
  /** Frozen seed handed to the editor; never re-sent as the editor changes. */
  initialHtml: string;
  /** Current editor HTML. */
  html: string;
  onChange: (next: string) => void;
  /** True once the user has actually changed the content. */
  isDirty: boolean;
  /** What to put in the patch: `""` for a cleared field, else the HTML. */
  outgoing: string;
}

/**
 * State for a plan field that is edited as rich text rather than plain text.
 *
 * Dirty-tracking is the whole difficulty here. The editor normalizes markup
 * when it loads a stored value — `<p>A</p>` comes back out as
 * `<p><span style="white-space: pre-wrap;">A</span></p>` — so comparing the
 * editor's HTML against the stored HTML as strings marks every seeded field
 * dirty before the user has touched anything, and the dialog would then patch
 * fields nobody edited.
 *
 * So the baseline is the editor's *own* first emission rather than the stored
 * string: whatever it produces from the seed is, by definition, the unedited
 * state. Two cases, and they differ because the editor only emits on load when
 * there is something to load:
 *
 * - Stored content is non-blank: the editor injects it and emits once before
 *   any user input, so the first emission is the baseline.
 * - Stored content is blank: nothing is injected and nothing is emitted, so
 *   the first emission would be the user's own typing. Baseline is fixed to
 *   "blank" up front instead, and dirtiness is `!isBlankHtml`.
 *
 * A cleared field goes out as `""`, not the editor's `<p><br></p>` — an empty
 * paragraph reads as "the plan says nothing" rather than "there is no plan".
 */
function useRichTextPlanField(storedHtml?: string | null): RichTextPlanField {
  const stored = storedHtml ?? "";
  // Frozen at mount: the editor treats its `value` as an initial value, and
  // feeding the live HTML back in makes it re-seed itself mid-edit.
  const [initialHtml] = useState(stored);
  const [html, setHtml] = useState(stored);
  // `null` means "waiting for the editor's first emission"; `""` means the
  // baseline is known to be blank.
  const [baseline, setBaseline] = useState<string | null>(
    isBlankHtml(stored) ? "" : null,
  );

  const onChange = useCallback((next: string) => {
    setBaseline((current) => (current === null ? next : current));
    setHtml(next);
  }, []);

  const isDirty =
    baseline === null
      ? false
      : baseline === ""
        ? !isBlankHtml(html)
        : html !== baseline;

  // Sanitized on the way out, the same policy the detail page renders it back
  // through, so nothing the editor emits can widen what ends up stored.
  const outgoing = isBlankHtml(html) ? "" : sanitizeRichTextHtml(html);

  return { initialHtml, html, onChange, isDirty, outgoing };
}

/**
 * Edit the change-request fields the BE allows updating -- every field the
 * create form sets: the subject, type (in New only: it decides the approval
 * flow), impact, priority, description, justification and risk and impact
 * analysis, the originating
 * service request (`caseId`; a change request raised without one can be linked
 * afterwards, and a wrong link corrected), the planned window,
 * the assignment group, the individual assignee, requester, customer group,
 * rollback duration, the implementation/rollback/test/affected-services/
 * affected-components plans (the last five added 2026-08-20, see
 * `CHANGES-cr-field-parity.md`), and whether the Implementation Plan is
 * visible to customers.
 * Only changed fields are sent, and the BE requires at least one, so Save is
 * disabled until something differs.
 *
 * Customer Project / Deployments / Deployment products and Category are
 * editable too. The scope fields cascade exactly like the create form
 * (project -> deployments, with deployment products derived and read-only). The
 * Customer Project is editable only in New and is READ-ONLY from the moment
 * approval is requested (the backend refuses the edit in every later state); the
 * deployments stay editable until the change request reaches Implement, within
 * that project. The Customer Group is shown read-only: it is the chosen project's
 * registered contacts, so changing the project changes it and nothing about it is
 * ever sent.
 *
 * The two customer tick boxes are fully editable in New and ADD-ONLY afterwards:
 * a ticked box is read-only, an unticked one can still be ticked until the gate it
 * controls is passed (and only with a Customer Project set), with a note that it
 * cannot be removed once saved. The rule is computed here from the record
 * (`customerApprovalLockedReason` and friends) exactly as the backend decides it;
 * the backend remains the authority and its 400 shows in `saveError`.
 *
 * An EMERGENCY change acts without customer consent, so both boxes are disabled
 * and unticked, in every state, with one line saying so. A change of that type
 * raised before the rule that still has a box ticked in New opens with it off
 * (saving clears it); after New nothing can untick it (add-only), so a ticked box
 * is shown as stored, disabled.
 *
 * ONCE THE CHANGE REQUEST HAS BEEN APPROVED (CAB approval received; a Standard
 * change once Request Approval scheduled it) every field is read-only
 * (`changeRequestApprovedReadOnlyReason`): the form shows them disabled under
 * the reason and offers a work note and the DEVIATION FROM THE IMPLEMENTATION
 * PLAN from Implement on, which is all the backend accepts then (400
 * `change_request_approved_read_only` otherwise). The deviation has a lock of
 * its own (`implementationDeviationLockedReason`, judged on the stored state):
 * editable in implement, review and customer_review; before that read-only (400
 * `change_request_deviation_not_yet`) and final once the change is closed,
 * canceled or rolled back (400 `change_request_deviation_final`), shown only
 * when one is stored. In New a stored one can only have been carried over by a
 * clone: it is shown read-only under the plan with a "Clear deviation" action
 * (`implementationDeviationClearable`), sent as `implementationDeviation: ""`
 * with the save, because Request Approval is refused while it is there.
 * A work note can be added with any save, before the approval too.
 *
 * Deliberately NOT here, even though the backend's write contract accepts
 * them: the customer-visible `comment` (the Comments tab covers it), and
 * `durationInput` (only succeeds against an exact-match
 * validation rule not worth half-implementing here — see
 * `BePatchChangeRequestPayload`'s doc comment for the full reasoning on each).
 *
 * `isCustomerApproved`/`isCustomerReviewed` are deliberately NOT exposed here,
 * and not modeled in `BePatchChangeRequestPayload`: they ARE the customer's
 * answer (the customer's approval moves the change to Scheduled, their
 * rejection cancels it; the customer's review closes it or rolls it back),
 * which only the customer gives, in the Customer Portal. The backend refuses
 * both from staff outright, so a switch labelled "Customer approved" /
 * "Customer reviewed" here would only be a way to answer for the customer.
 * What the customer has confirmed is shown read-only (`hasCustomerApproved` /
 * `hasCustomerReviewed`).
 */
export default function EditChangeRequestDialog({
  cr,
  isSaving,
  saveError,
  onClose,
  onSave,
  variant = "dialog",
}: EditChangeRequestDialogProps): JSX.Element {
  const initialPlannedStart = useMemo(
    () => backendUtcToZonedInput(cr.plannedStartOn),
    [cr.plannedStartOn],
  );
  const initialPlannedEnd = useMemo(
    () => backendUtcToZonedInput(cr.plannedEndOn),
    [cr.plannedEndOn],
  );
  const initialAssignedTeamId = cr.assignedTeam?.id ?? "";
  const initialAssignedEngineerId = cr.assignedEngineer?.id ?? "";
  const initialRequestedById = cr.requestedBy?.id ?? "";
  const initialCaseId = cr.case?.id ?? "";
  const initialRollbackDurationText = cr.rollbackDurationText ?? "";
  const initialIsPlanningVisibleToCustomers = cr.isPlanningVisibleToCustomers ?? false;
  const initialCustomerApprovalRequired = cr.customerApprovalRequired ?? false;
  const initialCustomerReviewRequired = cr.customerReviewRequired ?? false;
  // Once approved, every field is read-only: only a work note can be added, and,
  // from Implement until the change is closed, canceled or rolled back, the
  // deviation from the plan.
  const approvedLock = changeRequestApprovedReadOnlyReason(cr.state);
  const approvedNotice = changeRequestApprovedNotice(cr.state);
  const fieldsDisabled = isSaving || !!approvedLock;
  // Raised from a standard change template: what the template specifies (the
  // subject, the plans, impact, category, rollback duration, planning visibility
  // and the two customer boxes) is frozen in every state; the backend refuses a
  // change to any of them (400 `change_request_standard_template_field`). The
  // parameters' values cannot be edited either. Everything else stays editable.
  const templateLock = cr.standardChange ? STANDARD_TEMPLATE_FIELD_LOCKED_REASON : null;
  const templateDisabled = fieldsDisabled || !!templateLock;
  // The deviation is not under that lock: it is recorded in implement, review
  // and customer_review, and elsewhere shown read-only when one is stored. In New
  // a stored one (a clone carried it) can be cleared, and has to be before
  // Request Approval.
  const deviationLocked = implementationDeviationLockedReason(cr.state, cr.implementationDeviation);
  const showDeviation = implementationDeviationOffered(cr.state, cr.implementationDeviation);
  const deviationClearable = implementationDeviationClearable(cr.state, cr.implementationDeviation);
  const [deviationCleared, setDeviationCleared] = useState(false);
  const initialTitle = cr.subject ?? "";
  const [title, setTitle] = useState(initialTitle);
  const initialImpact = changeRequestImpactValue(cr.impact);
  const [impact, setImpact] = useState<BeChangeRequestImpact | "">(initialImpact);
  const initialPriority = changeRequestPriorityValue(cr.priority);
  const [priority, setPriority] = useState<BeChangeRequestPriority | "">(initialPriority);
  // The type decides the approval flow: editable in New only (the backend's rule).
  const initialType = creatableTypeValue(cr.type);
  const [type, setType] = useState<BeChangeRequestType | "">(initialType);
  const typeLocked = cr.standardChange
    ? STANDARD_TEMPLATE_TYPE_LOCKED_REASON
    : isChangeRequestCreationPhase(cr.state)
      ? null
      : CHANGE_REQUEST_TYPE_FROZEN_REASON;
  const [workNote, setWorkNote] = useState("");
  // Emergency: no customer steps (see the doc comment above). In New the form seeds
  // them OFF, so a leftover tick from before the rule is cleared by saving. The
  // stored type decides the locks; the chosen one (it can change in New) decides
  // whether the boxes are offered.
  const storedIsEmergency = isEmergencyChangeRequestType(cr.type);
  const isEmergency = isEmergencyChangeRequestType(type || cr.type);
  const emergencyClearsCustomerSteps = storedIsEmergency && isChangeRequestCreationPhase(cr.state);
  // After New the customer's part is add-only (see the rules in utils/changeRequests.ts):
  // a ticked box is read-only, an unticked one may still be ticked until the gate it
  // controls has passed and only when a Customer Project is set. The backend refuses
  // anything else (400) and stays the authority; the control is disabled up front
  // with the reason, computed here from the same inputs (state, stored value, project).
  const hasStoredProject = !!cr.project?.id;
  // An Emergency change's unticked box is off for the Emergency rule (the line under
  // the boxes), not for one of these reasons; a box a change of that type still has
  // ticked from before the rule keeps its add-only one.
  const customerApprovalLocked = templateLock
    ? TEMPLATE_FIELD_HINT
    : storedIsEmergency && !initialCustomerApprovalRequired
      ? null
      : customerApprovalLockedReason(cr.state, {
          stored: initialCustomerApprovalRequired,
          hasProject: hasStoredProject,
        });
  const customerReviewLocked = templateLock
    ? TEMPLATE_FIELD_HINT
    : storedIsEmergency && !initialCustomerReviewRequired
      ? null
      : customerReviewLockedReason(cr.state, {
          stored: initialCustomerReviewRequired,
          hasProject: hasStoredProject,
        });
  const [plannedStart, setPlannedStart] = useState(initialPlannedStart);
  const [plannedEnd, setPlannedEnd] = useState(initialPlannedEnd);
  const [assignedTeamId, setAssignedTeamId] = useState(initialAssignedTeamId);
  const [assignedEngineerId, setAssignedEngineerId] = useState(initialAssignedEngineerId);
  const initialProjectId = cr.project?.id ?? "";
  const initialDeploymentIds = useMemo(() => cr.deployments?.map((d) => d.id) ?? [], [cr.deployments]);
  const initialCategory = changeRequestCategoryValue(cr.category);
  const [category, setCategory] = useState<string>(initialCategory);
  // Customer Project / Deployments / Deployment products, seeded from the record.
  // The project is fixed the moment the change request leaves New; the deployments
  // stay editable (within that project) until Implement. The backend refuses
  // anything else, so the controls are disabled up front with the reason.
  const projectLocked = customerProjectLockedReason(cr.state);
  const scopeLocked = changeRequestScopeLockedReason(cr.state);
  // Request Approval (New -> Assess) is still ahead: it needs an Assignment group,
  // and a Customer Project when a customer step is ticked on a non-Emergency change.
  // Both are marked required while it is, though saving needs neither (the action
  // bar's Request Approval is the gate). After New neither can be cleared.
  const approvalAhead = isChangeRequestCreationPhase(cr.state);
  const scope = useChangeRequestScope({
    projectId: cr.project?.id,
    projectLabel: cr.project?.name,
    deployments: cr.deployments?.map((d) => ({ id: d.id, label: d.name })),
    deploymentProducts: cr.deploymentProducts?.map((p) => ({ id: p.id, label: p.name })),
  });
  const [requestedById, setRequestedById] = useState(initialRequestedById);
  const [caseId, setCaseId] = useState(initialCaseId);
  const [rollbackDurationText, setRollbackDurationText] = useState(initialRollbackDurationText);
  const [isPlanningVisibleToCustomers, setIsPlanningVisibleToCustomers] = useState(
    initialIsPlanningVisibleToCustomers,
  );
  const [customerApprovalRequired, setCustomerApprovalRequired] = useState(
    !emergencyClearsCustomerSteps && initialCustomerApprovalRequired,
  );
  const [customerReviewRequired, setCustomerReviewRequired] = useState(
    !emergencyClearsCustomerSteps && initialCustomerReviewRequired,
  );
  const projectNeededForApproval =
    approvalAhead &&
    customerProjectNeededForApproval(type || cr.type, customerApprovalRequired, customerReviewRequired);
  const rollbackPlan = useRichTextPlanField(cr.rollbackPlan);
  const testPlan = useRichTextPlanField(cr.testPlan);
  const implementationPlan = useRichTextPlanField(cr.implementationPlan);
  const implementationDeviation = useRichTextPlanField(cr.implementationDeviation);
  // Only an edit the backend can take is sent (or changes the button's label).
  const deviationEdited = implementationDeviation.isDirty && !deviationLocked;
  const affectedServicesText = useRichTextPlanField(cr.affectedServicesText);
  const affectedComponentsText = useRichTextPlanField(cr.affectedComponentsText);
  const description = useRichTextPlanField(cr.description);
  const justification = useRichTextPlanField(cr.justification);
  const impactDescription = useRichTextPlanField(cr.impactDescription);

  // Client-side only, and only when both ends are set: the backing system
  // does its own validation and this must not become the thing that blocks a
  // legitimate save, so it surfaces inline rather than being enforced
  // server-side.
  const startDate = parseDateTimeLocal(plannedStart);
  const endDate = parseDateTimeLocal(plannedEnd);
  const plannedEndBeforeStart =
    !!startDate && !!endDate && endDate.getTime() <= startDate.getTime();

  const titleMissing = !title.trim() && !templateLock;

  const patch = useMemo<BePatchChangeRequestPayload>(() => {
    const next: BePatchChangeRequestPayload = {};
    // The template's fields are never sent (their controls are disabled; an
    // editor's own normalisation must not look like an edit either).
    const editable = !templateLock;
    if (editable && title.trim() && title.trim() !== initialTitle.trim()) next.title = title.trim();
    if (editable && description.isDirty) next.description = description.outgoing;
    if (editable && justification.isDirty) next.justification = justification.outgoing;
    if (editable && impactDescription.isDirty) next.impactDescription = impactDescription.outgoing;
    if (editable && impact && impact !== initialImpact) next.impact = impact;
    if (priority && priority !== initialPriority) next.priority = priority;
    if (!typeLocked && type && type !== initialType) next.type = type;
    if (workNote.trim()) next.workNote = workNote.trim();
    // The form holds wall-clock values in the user's timezone (seeded from the
    // record's UTC value above); the BE takes UTC.
    if (plannedStart !== initialPlannedStart && plannedStart) {
      const utc = zonedInputToBackendUtc(plannedStart);
      if (utc) next.plannedStartOn = utc;
    }
    if (plannedEnd !== initialPlannedEnd && plannedEnd) {
      const utc = zonedInputToBackendUtc(plannedEnd);
      if (utc) next.plannedEndOn = utc;
    }
    if (assignedTeamId !== initialAssignedTeamId && assignedTeamId) {
      next.assignedTeamId = assignedTeamId;
    }
    if (assignedEngineerId !== initialAssignedEngineerId && assignedEngineerId) {
      next.assignedEngineerId = assignedEngineerId;
    }
    // Unlike the pickers above, an emptied plan field is a real edit the BE
    // can accept, so "" is sent rather than skipped. Both plans are rich text
    // on both sides now — see `useRichTextPlanField` for why "changed" is not
    // a comparison against the stored string.
    if (editable && rollbackPlan.isDirty) next.rollbackPlan = rollbackPlan.outgoing;
    if (editable && testPlan.isDirty) next.testPlan = testPlan.outgoing;
    if (editable && implementationPlan.isDirty) next.implementationPlan = implementationPlan.outgoing;
    if (deviationEdited) {
      next.implementationDeviation = implementationDeviation.outgoing;
    } else if (deviationClearable && deviationCleared) {
      next.implementationDeviation = "";
    }
    if (affectedServicesText.isDirty) next.affectedServicesText = affectedServicesText.outgoing;
    if (affectedComponentsText.isDirty) next.affectedComponentsText = affectedComponentsText.outgoing;
    if (editable && rollbackDurationText !== initialRollbackDurationText) {
      next.rollbackDurationText = rollbackDurationText;
    }
    if (editable && category !== initialCategory && category) {
      next.category = category as BeChangeRequestCategory;
    }
    // The scope fields are validated by the backend as a unit, so when any of
    // them changed the whole set goes out together (see BePatchChangeRequestPayload).
    // Deployment products are derived: sent only once the lookup has settled.
    // A frozen project is never sent changed (its picker is disabled); sending the
    // stored one along with changed deployments is accepted as a no-op.
    const scopeChanged =
      !scopeLocked &&
      !!scope.projectId &&
      (scope.projectId !== initialProjectId || !sameIds(scope.deploymentIds, initialDeploymentIds));
    if (scopeChanged) {
      next.projectId = scope.projectId;
      next.deploymentIds = scope.deploymentIds;
      if (scope.productsReady) next.deploymentProductIds = scope.deploymentProductIds;
    }
    if (requestedById !== initialRequestedById && requestedById) {
      next.requestedById = requestedById;
    }
    // Like the pickers above, a cleared link is not sent: the patch has no way
    // to express "no originating service request".
    if (caseId !== initialCaseId && caseId) {
      next.caseId = caseId;
    }
    if (editable && isPlanningVisibleToCustomers !== initialIsPlanningVisibleToCustomers) {
      next.isPlanningVisibleToCustomers = isPlanningVisibleToCustomers;
    }
    if (!customerApprovalLocked && customerApprovalRequired !== initialCustomerApprovalRequired) {
      next.customerApprovalRequired = customerApprovalRequired;
    }
    if (!customerReviewLocked && customerReviewRequired !== initialCustomerReviewRequired) {
      next.customerReviewRequired = customerReviewRequired;
    }
    return next;
  }, [
    templateLock,
    title,
    initialTitle,
    description.isDirty,
    description.outgoing,
    justification.isDirty,
    justification.outgoing,
    impactDescription.isDirty,
    impactDescription.outgoing,
    impact,
    initialImpact,
    priority,
    initialPriority,
    typeLocked,
    type,
    initialType,
    workNote,
    plannedStart,
    initialPlannedStart,
    plannedEnd,
    initialPlannedEnd,
    assignedTeamId,
    initialAssignedTeamId,
    assignedEngineerId,
    initialAssignedEngineerId,
    rollbackPlan.isDirty,
    rollbackPlan.outgoing,
    testPlan.isDirty,
    testPlan.outgoing,
    implementationPlan.isDirty,
    implementationPlan.outgoing,
    deviationEdited,
    implementationDeviation.outgoing,
    deviationClearable,
    deviationCleared,
    affectedServicesText.isDirty,
    affectedServicesText.outgoing,
    affectedComponentsText.isDirty,
    affectedComponentsText.outgoing,
    rollbackDurationText,
    initialRollbackDurationText,
    category,
    initialCategory,
    scopeLocked,
    scope.projectId,
    scope.deploymentIds,
    scope.deploymentProductIds,
    scope.productsReady,
    initialProjectId,
    initialDeploymentIds,
    requestedById,
    initialRequestedById,
    caseId,
    initialCaseId,
    isPlanningVisibleToCustomers,
    initialIsPlanningVisibleToCustomers,
    customerApprovalRequired,
    initialCustomerApprovalRequired,
    customerApprovalLocked,
    customerReviewRequired,
    initialCustomerReviewRequired,
    customerReviewLocked,
  ]);

  const hasChanges = Object.keys(patch).length > 0;
  // Non-blocking: editing a CR's planned start to a past instant is unusual
  // but not forbidden (e.g. recording when it actually started), so this
  // only warns.
  const plannedStartIsPast = isPastZonedInput(plannedStart);

  // Rich-text plan field. The editor takes no `id`/native label, so the
  // visible label is a separate Typography tied to the control via
  // role="group" + aria-labelledby, and the helper text is referenced by
  // aria-describedby — same convention as the create page's Planning fields.
  const renderPlanField = (
    id: string,
    label: string,
    field: RichTextPlanField,
    helperText: string,
    fromTemplate = false,
  ): JSX.Element => (
    // Resizable, and Expand opens it full screen (ExpandableRichTextField); the
    // editor is seeded with the frozen `initialHtml` so dirty tracking holds.
    <ExpandableRichTextField
      id={id}
      label={label}
      value={field.html}
      seedHtml={field.initialHtml}
      onChange={field.onChange}
      disabled={fromTemplate ? templateDisabled : fieldsDisabled}
      helperText={fromTemplate && templateLock ? TEMPLATE_FIELD_HINT : helperText}
      sx={FULL_ROW}
    />
  );

  // One of the two customer-step checkboxes. When locked it is disabled and
  // the lock reason replaces the helper line (which the input references via
  // aria-describedby, so assistive tech announces it) and also rides on a
  // tooltip for pointer users.
  const renderCustomerStepCheckbox = (
    id: string,
    label: string,
    helperText: string,
    checked: boolean,
    onChange: (next: boolean) => void,
    lockedReason: string | null,
    onceSavedHelper: string | null,
  ): JSX.Element => {
    const control = (
      <FormControlLabel
        sx={{ alignItems: "flex-start", m: 0 }}
        disabled={fieldsDisabled || !!lockedReason || isEmergency}
        control={
          <Checkbox
            size="small"
            checked={checked}
            onChange={(e) => onChange(e.target.checked)}
            inputProps={{
              "aria-label": label,
              "aria-describedby": isEmergency ? `${id}-desc ${EMERGENCY_NOTE_ID}` : `${id}-desc`,
            }}
          />
        }
        label={
          <Box>
            <Typography variant="body1">{label}</Typography>
            <Typography id={`${id}-desc`} variant="body2" color="text.secondary">
              {lockedReason ?? (onceSavedHelper ? `${helperText} ${onceSavedHelper}` : helperText)}
            </Typography>
          </Box>
        }
      />
    );
    return lockedReason ? (
      <Tooltip title={lockedReason}>
        <Box component="span" sx={{ display: "block" }}>
          {control}
        </Box>
      </Tooltip>
    ) : (
      control
    );
  };

  // The deviation from the implementation plan: its own lock (recorded from
  // Implement on, final once the change is over), never the approved one. Once
  // approved it sits at the top with the work note, as the inputs left; before
  // that, under the plan -- in New, where a clone carried it over, read-only with
  // the action that clears it.
  const deviationField = !showDeviation ? null : deviationClearable ? (
    <CarriedOverDeviationField
      id="cr-edit-implementation-deviation"
      html={cr.implementationDeviation ?? ""}
      cleared={deviationCleared}
      onClearedChange={setDeviationCleared}
      clearedText="It is cleared when you save."
      disabled={isSaving}
      sx={FULL_ROW}
    />
  ) : (
    <ExpandableRichTextField
      id="cr-edit-implementation-deviation"
      label="Deviation from the implementation plan"
      value={implementationDeviation.html}
      seedHtml={implementationDeviation.initialHtml}
      onChange={implementationDeviation.onChange}
      disabled={isSaving || !!deviationLocked}
      helperText={
        deviationLocked ??
        "What the implementation or the review did differently from the implementation plan. Internal: never shown to the customer."
      }
      sx={FULL_ROW}
    />
  );

  // Internal only: a work note is journaled with the save and never shown to the
  // customer. The one input left once the change has been approved, so then it is
  // required: "Add work note" stays disabled until it has text.
  const workNoteField = (
    <TextField
      id="cr-edit-work-note"
      label="Work note (internal)"
      required={!!approvedLock}
      value={workNote}
      onChange={(e) => setWorkNote(e.target.value)}
      fullWidth
      multiline
      minRows={2}
      size="small"
      disabled={isSaving}
      helperText="Added to the activity as an internal work note; never shown to the customer."
      sx={FULL_ROW}
    />
  );

  const inline = variant === "inline";
  const fields = (
        <Box
          sx={
            inline
              ? {
                  display: "grid",
                  gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
                  columnGap: 3,
                  rowGap: 2,
                  alignItems: "start",
                }
              : { display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }
          }
        >
          {saveError && (
            <Alert severity="error" sx={{ width: "100%", ...FULL_ROW }}>
              {saveError}
            </Alert>
          )}
          {approvedNotice && (
            // An explanation, not an error: a status, so a save error stays the form's one alert.
            <Alert severity="info" role="status" sx={FULL_ROW}>
              {approvedNotice}
            </Alert>
          )}
          {templateLock && cr.standardChange && (
            <Alert severity="info" role="status" sx={FULL_ROW}>
              {`Raised from standard change template ${cr.standardChange.templateNumber} (version ${cr.standardChange.version}). `}
              {templateLock}
            </Alert>
          )}
          {approvedLock && deviationField}
          {approvedLock && workNoteField}
          <TextField
            id="cr-edit-subject"
            label="Subject"
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            fullWidth
            size="small"
            disabled={templateDisabled}
            error={titleMissing && !approvedLock}
            helperText={
              titleMissing && !approvedLock
                ? "A change request needs a subject."
                : templateLock
                  ? TEMPLATE_FIELD_HINT
                  : undefined
            }
            slotProps={{ htmlInput: { maxLength: SUBJECT_MAX } }}
            sx={FULL_ROW}
          />
          <FormControl fullWidth size="small" disabled={fieldsDisabled || !!typeLocked}>
            <InputLabel id="cr-edit-type-label" shrink>
              Type
            </InputLabel>
            <Select
              labelId="cr-edit-type-label"
              label="Type"
              value={type}
              displayEmpty
              inputProps={{ "aria-describedby": typeLocked ? "cr-edit-type-help" : undefined }}
              onChange={(e) => {
                const next = String(e.target.value) as BeChangeRequestType | "";
                setType(next);
                // An Emergency change takes no customer step: the backend requires
                // both boxes off in the request that re-types it.
                if (isEmergencyChangeRequestType(next)) {
                  setCustomerApprovalRequired(false);
                  setCustomerReviewRequired(false);
                }
              }}
            >
              {!initialType && (
                <MenuItem value="">
                  <Typography component="span" color="text.secondary">
                    {cr.type ? cr.type : "-- Select --"}
                  </Typography>
                </MenuItem>
              )}
              {/* Standard only for a change that already is one: a change becomes a standard
                  change by being raised from a template, never by an edit (the server refuses it). */}
              {CHANGE_REQUEST_CREATE_TYPE_OPTIONS.filter((o) => o.value !== "standard" || initialType === "standard").map((o) => (
                <MenuItem key={o.value} value={o.value}>
                  {o.label}
                </MenuItem>
              ))}
            </Select>
            {typeLocked && !approvedLock && <FormHelperText id="cr-edit-type-help">{typeLocked}</FormHelperText>}
          </FormControl>
          <FormControl fullWidth size="small" disabled={templateDisabled}>
            <InputLabel id="cr-edit-impact-label" shrink>
              Impact
            </InputLabel>
            <Select
              labelId="cr-edit-impact-label"
              label="Impact"
              value={impact}
              displayEmpty
              onChange={(e) => setImpact(String(e.target.value) as BeChangeRequestImpact | "")}
            >
              {!initialImpact && (
                <MenuItem value="">
                  <Typography component="span" color="text.secondary">
                    -- Select --
                  </Typography>
                </MenuItem>
              )}
              {CHANGE_REQUEST_IMPACT_OPTIONS.map((o) => (
                <MenuItem key={o.value} value={o.value}>
                  {o.label}
                </MenuItem>
              ))}
            </Select>
            {templateLock && <FormHelperText>{TEMPLATE_FIELD_HINT}</FormHelperText>}
          </FormControl>
          <FormControl fullWidth size="small" disabled={fieldsDisabled}>
            <InputLabel id="cr-edit-priority-label" shrink>
              Priority
            </InputLabel>
            <Select
              labelId="cr-edit-priority-label"
              label="Priority"
              value={priority}
              displayEmpty
              onChange={(e) => setPriority(String(e.target.value) as BeChangeRequestPriority | "")}
            >
              {!initialPriority && (
                <MenuItem value="">
                  <Typography component="span" color="text.secondary">
                    -- Select --
                  </Typography>
                </MenuItem>
              )}
              {CHANGE_REQUEST_PRIORITY_OPTIONS.map((o) => (
                <MenuItem key={o.value} value={o.value}>
                  {o.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          {/*
            No `clearable` on either picker. The patch payload has no way to
            express "remove the planned date" — `plannedStartOn`/`plannedEndOn`
            are `string | undefined`, and an omitted key means "leave it
            alone" — so a clear affordance would appear to work and then
            silently save nothing. Widening the payload to express a null
            clear is the fix if this is ever actually needed.
          */}
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label="Planned start"
              disabled={fieldsDisabled}
              value={startDate}
              onChange={(next) =>
                setPlannedStart(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateTimeLocal(next)
                    : "",
                )
              }
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  helperText: plannedStartIsPast
                    ? "This date is in the past."
                    : undefined,
                },
              }}
            />
            <DateTimePicker
              label="Planned end"
              disabled={fieldsDisabled}
              value={endDate}
              onChange={(next) =>
                setPlannedEnd(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateTimeLocal(next)
                    : "",
                )
              }
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  error: plannedEndBeforeStart,
                  helperText: plannedEndBeforeStart
                    ? "Planned end must be after planned start."
                    : undefined,
                },
              }}
            />
          </LocalizationProvider>
          <AsyncEntitySelect<BeGroup>
            id="cr-edit-assigned-team"
            label="Assignment group"
            placeholder="Search groups…"
            value={assignedTeamId}
            onChange={setAssignedTeamId}
            disabled={fieldsDisabled}
            useSearch={useSearchGroups}
            getId={(g) => g.id}
            getLabel={(g) => g.name}
            knownLabel={cr.assignedTeam?.name}
            required={approvalAhead}
            helperText="Required before moving to Assess — its members become the Assess-stage approvers."
          />
          <AsyncEntitySelect<BeUser>
            id="cr-edit-assigned-engineer"
            label="Assigned to"
            placeholder="Search people…"
            value={assignedEngineerId}
            onChange={setAssignedEngineerId}
            disabled={fieldsDisabled}
            useSearch={useSearchInternalUsersByName}
            getId={(u) => u.id!}
            getLabel={userLabel}
            knownLabel={cr.assignedEngineer?.name}
          />
          {/*
            Service requests only: the PATCH resolves `caseId` against the case
            table, so an incident (which the create form's picker can find but
            never submit) has no place here.
          */}
          <AsyncEntitySelect<BeCaseSearchView>
            id="cr-edit-originating-service-request"
            label="Originating service request"
            placeholder="Search service requests by CS number or subject…"
            value={caseId}
            onChange={setCaseId}
            disabled={fieldsDisabled}
            useSearch={useSearchServiceRequestsForSelect}
            searchExtra={cr.project?.id}
            getId={(c) => c.id}
            getLabel={(c) => parentRecordLabel({ id: c.id, number: c.number, subject: c.subject })}
            knownLabel={cr.case?.name}
            helperText={
              initialCaseId
                ? "The service request this change request was raised from."
                : "Link the service request this change request was raised from, if any."
            }
          />
          <AsyncEntitySelect<BeUser>
            id="cr-edit-requested-by"
            label="Requested by"
            placeholder="Search people…"
            value={requestedById}
            onChange={setRequestedById}
            disabled={fieldsDisabled}
            useSearch={useSearchInternalUsersByName}
            getId={(u) => u.id!}
            getLabel={userLabel}
            knownLabel={cr.requestedBy?.name}
          />
          <Box
            role="group"
            aria-label="Customer project and deployments"
            sx={{ display: "flex", flexDirection: "column", gap: 1, ...FULL_ROW }}
          >
            {(projectLocked || scopeLocked) && (
              // An explanation, not an error: a status, so a save error stays the dialog's one alert.
              <Alert severity="info" role="status">
                {projectLocked && <Typography variant="body2">{projectLocked}</Typography>}
                {scopeLocked && <Typography variant="body2">{scopeLocked}</Typography>}
              </Alert>
            )}
            <ChangeRequestScopeFields
              scope={scope}
              disabled={fieldsDisabled || !!scopeLocked}
              projectDisabled={!!projectLocked}
              idPrefix="cr-edit"
              // A saved project can be swapped for another but not removed —
              // the patch has no way to express "no project".
              projectClearable={!initialProjectId}
              projectRequired={projectNeededForApproval}
              projectHelperText={projectNeededForApproval ? REQUIRED_TO_REQUEST_APPROVAL_HELPER : undefined}
            />
            <ChangeRequestCustomerGroupField scope={scope} idPrefix="cr-edit" />
          </Box>
          <FormControl fullWidth size="small" disabled={templateDisabled}>
            <InputLabel id="cr-edit-category-label" shrink>
              Category
            </InputLabel>
            <Select
              labelId="cr-edit-category-label"
              label="Category"
              value={category}
              displayEmpty
              onChange={(e) => setCategory(String(e.target.value))}
            >
              <MenuItem value="">
                <Typography component="span" color="text.secondary">
                  -- Select --
                </Typography>
              </MenuItem>
              {CHANGE_REQUEST_CATEGORY_OPTIONS.map((o) => (
                <MenuItem key={o.value} value={o.value}>
                  {o.label}
                </MenuItem>
              ))}
            </Select>
            {templateLock && <FormHelperText>{TEMPLATE_FIELD_HINT}</FormHelperText>}
          </FormControl>
          {renderCustomerStepCheckbox(
            "cr-edit-customer-approval",
            "Customer Approval",
            "Adds a customer approval step after internal approval, before scheduling.",
            customerApprovalRequired,
            setCustomerApprovalRequired,
            customerApprovalLocked,
            isEmergency || approvedLock ? null : customerRequirementOnceSavedHelper(cr.state, initialCustomerApprovalRequired),
          )}
          {renderCustomerStepCheckbox(
            "cr-edit-customer-review",
            "Customer Review",
            "Adds a customer review step after Review, before closing.",
            customerReviewRequired,
            setCustomerReviewRequired,
            customerReviewLocked,
            isEmergency || approvedLock ? null : customerRequirementOnceSavedHelper(cr.state, initialCustomerReviewRequired),
          )}
          {isEmergency && (
            <FormHelperText id={EMERGENCY_NOTE_ID} sx={{ mx: 0, ...FULL_ROW }}>
              {EMERGENCY_CUSTOMER_STEPS_HELPER}
            </FormHelperText>
          )}
          <TextField
            label="Rollback duration"
            value={rollbackDurationText}
            onChange={(e) => setRollbackDurationText(e.target.value.slice(0, CHANGE_REQUEST_ROLLBACK_DURATION_MAX))}
            fullWidth
            size="small"
            disabled={templateDisabled}
            placeholder="e.g. 30 mins"
            helperText={templateLock ? TEMPLATE_FIELD_HINT : "Free text — ServiceNow does not parse this into a structured duration."}
          />
          <FormControlLabel
            sx={{ ml: 0, justifyContent: "space-between", width: "100%", ...FULL_ROW }}
            labelPlacement="start"
            control={
              <Switch
                size="small"
                checked={isPlanningVisibleToCustomers}
                onChange={(e) => setIsPlanningVisibleToCustomers(e.target.checked)}
                disabled={templateDisabled}
                inputProps={{ "aria-label": "Implementation Plan visible to customers" }}
              />
            }
            label={
              <Typography variant="body2" color="text.secondary">
                Implementation Plan visible to customers
              </Typography>
            }
          />
          {renderPlanField("cr-edit-description", "Description", description, "What is changing?", true)}
          {renderPlanField(
            "cr-edit-justification",
            "Justification",
            justification,
            "Why this change is needed.",
            true,
          )}
          {renderPlanField(
            "cr-edit-risk-impact",
            "Risk and impact analysis",
            impactDescription,
            "What could go wrong, and what is affected if it does.",
            true,
          )}
          {renderPlanField(
            "cr-edit-implementation-plan",
            "Implementation plan",
            implementationPlan,
            "How this change is carried out.",
            true,
          )}
          {!approvedLock && deviationField}
          {renderPlanField(
            "cr-edit-rollback-plan",
            "Rollback plan",
            rollbackPlan,
            "How this change is backed out if it goes wrong.",
            true,
          )}
          {renderPlanField(
            "cr-edit-test-plan",
            "Test plan",
            testPlan,
            "How the change is verified once implemented.",
            true,
          )}
          {renderPlanField(
            "cr-edit-affected-services",
            "Affected services",
            affectedServicesText,
            "Services impacted by this change.",
          )}
          {renderPlanField(
            "cr-edit-affected-components",
            "Affected components",
            affectedComponentsText,
            "Components impacted by this change.",
          )}
          {!approvedLock && workNoteField}
        </Box>
  );

  const actions = (
    <>
      <Button color="inherit" onClick={onClose} disabled={isSaving}>
        Cancel
      </Button>
      <Button
        variant="contained"
        onClick={() => onSave(patch)}
        disabled={isSaving || !hasChanges || plannedEndBeforeStart || (titleMissing && !approvedLock)}
      >
        {isSaving ? "Saving…" : approvedLock && !deviationEdited ? "Add work note" : "Save"}
      </Button>
    </>
  );

  if (inline) {
    return (
      <Paper
        component="section"
        aria-labelledby="cr-inline-edit-title"
        sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}
      >
        <Typography id="cr-inline-edit-title" variant="h6" component="h2">
          Edit change request
        </Typography>
        {fields}
        <Box
          sx={{
            display: "flex",
            justifyContent: "flex-end",
            gap: 1,
            pt: 1.5,
            borderTop: 1,
            borderColor: "divider",
          }}
        >
          {actions}
        </Box>
      </Paper>
    );
  }

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit change request</DialogTitle>
      <DialogContent dividers>{fields}</DialogContent>
      <DialogActions>{actions}</DialogActions>
    </Dialog>
  );
}
