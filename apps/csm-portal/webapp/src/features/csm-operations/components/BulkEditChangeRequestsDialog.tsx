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
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  FormHelperText,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useQueries } from "@tanstack/react-query";
import { useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import { useBackendApi } from "@api/backend/client";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchItServices } from "@api/useSearchItServices";
import { useSearchServiceOfferings } from "@api/useSearchServiceOfferings";
import { useSearchInternalUsersByName } from "@api/useSearchUsersByName";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import { changeRequestDetailQuery } from "@features/csm-operations/api/useGetChangeRequest";
import { useSearchServiceRequestsForSelect } from "@features/csm-operations/api/useSearchServiceRequestsForSelect";
import ChangeRequestScopeFields from "@features/csm-operations/components/ChangeRequestScopeFields";
import ExpandableRichTextField from "@features/csm-operations/components/ExpandableRichTextField";
import { useChangeRequestScope, type ChangeRequestScope } from "@features/csm-operations/hooks/useChangeRequestScope";
import { itServiceLabel, userLabel } from "@features/csm-operations/utils/incidentFormOptions";
import {
  bulkChangeRequestTargets,
  CHANGE_REQUEST_CATEGORY_OPTIONS,
  CHANGE_REQUEST_CREATE_TYPE_OPTIONS,
  CHANGE_REQUEST_IMPACT_OPTIONS,
  CHANGE_REQUEST_JOURNAL_MAX,
  CHANGE_REQUEST_PRIORITY_OPTIONS,
  CHANGE_REQUEST_ROLLBACK_DURATION_MAX,
  changeRequestApprovedReadOnlyReason,
  changeRequestStateLabel,
  changeRequestTransitionLabel,
  changeRequestTransitionRequiresReason,
  customerApprovalLockedReason,
  customerProjectNeededForApproval,
  customerReviewLockedReason,
  EMERGENCY_CUSTOMER_STEPS_HELPER,
  isChangeRequestCreationPhase,
  isChangeRequestTerminal,
  isEmergencyChangeRequestType,
  parentRecordLabel,
} from "@features/csm-operations/utils/changeRequests";
import {
  bulkChangeRequestEditProblem,
  type BulkChangeRequestEdit,
} from "@features/csm-operations/utils/bulkChangeRequestEdit";
import { formatDateTimeLocal, parseDateTimeLocal, zonedInputToBackendUtc } from "@utils/dateTime";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import type {
  BeCaseSearchView,
  BeChangeRequestCategory,
  BeChangeRequestDetail,
  BeChangeRequestImpact,
  BeChangeRequestPriority,
  BeChangeRequestSearchView,
  BeChangeRequestType,
  BeGroup,
  BeItService,
  BeServiceOffering,
  BeUser,
} from "@api/backend/types";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface BulkEditChangeRequestsDialogProps {
  /** The selected change requests (none closed, canceled or rolled back: the list does not let those be selected). */
  changeRequests: BeChangeRequestSearchView[];
  isApplying: boolean;
  onClose: () => void;
  /** The one change to apply to every selected change request: only the fields set here. */
  onApply: (edit: BulkChangeRequestEdit) => void;
}

const NO_CHANGE = "";
/** The create form's subject cap. */
const SUBJECT_MAX = 500;

type HoldChoice = "" | "hold" | "release";
type YesNoChoice = "" | "yes" | "no";

/** The rich-text plan fields, in the edit form's order, with its labels. */
const PLAN_FIELDS = [
  { key: "description", label: "Description", placeholder: "What is changing?" },
  { key: "justification", label: "Justification", placeholder: "Why this change is needed." },
  { key: "impactDescription", label: "Risk and impact analysis", placeholder: "What could go wrong, and what is affected if it does." },
  { key: "implementationPlan", label: "Implementation plan", placeholder: "How this change is carried out." },
  { key: "rollbackPlan", label: "Rollback plan", placeholder: "How this change is backed out if it goes wrong." },
  { key: "testPlan", label: "Test plan", placeholder: "How the change is verified once implemented." },
  { key: "affectedServicesText", label: "Affected services", placeholder: "Services impacted by this change." },
  { key: "affectedComponentsText", label: "Affected components", placeholder: "Components impacted by this change." },
] as const;
type PlanFieldKey = (typeof PLAN_FIELDS)[number]["key"];

/** What a customer requirement can be set to for every one of `records`. */
function customerStepChoices(
  records: BeChangeRequestDetail[],
  step: "approval" | "review",
  type: BeChangeRequestType | "",
  projectId: string,
): { yes: boolean; no: boolean } {
  const lockedReason = step === "approval" ? customerApprovalLockedReason : customerReviewLockedReason;
  /** The requirement as the change request stores it. */
  const storedOf = (cr: BeChangeRequestDetail): boolean =>
    (step === "approval" ? cr.customerApprovalRequired : cr.customerReviewRequired) ?? false;
  return {
    // Ticked already (nothing to send), or one that can still be ticked and is not Emergency.
    yes: records.every(
      (cr) =>
        storedOf(cr) ||
        (!isEmergencyChangeRequestType(type || cr.type) &&
          !lockedReason(cr.state, { stored: false, hasProject: !!(projectId || cr.project?.id) })),
    ),
    // Unticked already, or still in New: after New a requirement can be added, never removed.
    no: records.every((cr) => !storedOf(cr) || isChangeRequestCreationPhase(cr.state)),
  };
}

/**
 * Bulk edit for the change requests selected in the list: the edit form's
 * fields, every one on "No change" until it is set, and only what is set is
 * sent, so each change request keeps its own value for the rest (ServiceNow's
 * list "Update selected"). It offers every field `PATCH /change-requests/{id}`
 * takes from the edit form, plus on hold and the two journals:
 *
 * - The state offers only the moves every selected change request can make,
 *   read from each one's own `legalNextStates` (so a Close, Cancel change or
 *   Roll back is offered when it is legal for all); Roll back and Cancel change
 *   need a reason, added to each as an internal work note in the same request.
 * - Once a change request has been approved its fields are read-only, so for
 *   those only the state, on hold and the notes are sent (the dialog says how
 *   many that is).
 * - The type and the Customer Project (with its deployments) are offered only
 *   when every selected change request is still New: they are fixed once
 *   approval has been requested.
 * - A customer requirement offers "Required" / "Not required" only when every
 *   selected change request can take it (the edit form's add-only rule; never
 *   for an Emergency change).
 *
 * The backend stays the authority: what it refuses for one change request is
 * reported for that one and the rest go through.
 */
export default function BulkEditChangeRequestsDialog({
  changeRequests,
  isApplying,
  onClose,
  onApply,
}: BulkEditChangeRequestsDialogProps): JSX.Element {
  const api = useBackendApi();
  const [title, setTitle] = useState("");
  const [state, setState] = useState(NO_CHANGE);
  const [type, setType] = useState<BeChangeRequestType | "">(NO_CHANGE);
  const [category, setCategory] = useState<BeChangeRequestCategory | "">(NO_CHANGE);
  const [impact, setImpact] = useState<BeChangeRequestImpact | "">(NO_CHANGE);
  const [priority, setPriority] = useState<BeChangeRequestPriority | "">(NO_CHANGE);
  const [caseId, setCaseId] = useState("");
  const [hold, setHold] = useState<HoldChoice>(NO_CHANGE);
  const [holdReason, setHoldReason] = useState("");
  const [serviceId, setServiceId] = useState("");
  const [serviceOfferingId, setServiceOfferingId] = useState("");
  const [assignedTeamId, setAssignedTeamId] = useState("");
  const [assignedEngineerId, setAssignedEngineerId] = useState("");
  const [requestedById, setRequestedById] = useState("");
  const [plannedStart, setPlannedStart] = useState("");
  const [plannedEnd, setPlannedEnd] = useState("");
  const [rollbackDuration, setRollbackDuration] = useState("");
  const [customerApproval, setCustomerApproval] = useState<YesNoChoice>(NO_CHANGE);
  const [customerReview, setCustomerReview] = useState<YesNoChoice>(NO_CHANGE);
  const [planVisible, setPlanVisible] = useState<YesNoChoice>(NO_CHANGE);
  const [plans, setPlans] = useState<Record<PlanFieldKey, string>>(() => {
    const empty = {} as Record<PlanFieldKey, string>;
    for (const f of PLAN_FIELDS) empty[f.key] = "";
    return empty;
  });
  const [serviceOutage, setServiceOutage] = useState("");
  const [communicationPlan, setCommunicationPlan] = useState("");
  const [workNote, setWorkNote] = useState("");
  const [comment, setComment] = useState("");
  const scope = useChangeRequestScope();
  // Whether the Customer Project or the deployments were changed here: only then are they sent.
  const [scopeTouched, setScopeTouched] = useState(false);

  // Each change request's own legal moves (and its type, hold, customer steps) come with its detail, not the list row.
  const details = useQueries({
    queries: changeRequests.map((cr) => changeRequestDetailQuery(api, cr.id)),
  });
  const detailsLoading = details.some((q) => q.isPending);
  const detailsFailed = details.some((q) => q.isError);
  const detailsReady = !detailsLoading && !detailsFailed;
  // Each change request as it is now: its detail once loaded, else the row as it was picked.
  const current = changeRequests.map((cr, i) => (details[i]?.data ?? cr) as BeChangeRequestDetail);
  // One that has finished since it was picked is skipped by the run, so it neither narrows the moves nor counts.
  const editable = current.filter((cr) => !isChangeRequestTerminal(cr.state));
  const finishedCount = current.length - editable.length;
  const targets = !detailsReady || editable.length === 0 ? [] : bulkChangeRequestTargets(editable);
  // The ones whose own fields can change: not yet approved.
  const open = editable.filter((cr) => !changeRequestApprovedReadOnlyReason(cr.state));
  const approvedCount = editable.length - open.length;
  const onHoldCount = editable.filter((cr) => cr.onHold).length;
  const needsReason = !!state && changeRequestTransitionRequiresReason(state);

  const fieldsDisabled = isApplying || (editable.length > 0 && open.length === 0);
  // Type and Customer Project: only while every one whose fields change is still New.
  const allNew = detailsReady && open.length > 0 && open.every((cr) => isChangeRequestCreationPhase(cr.state));
  const detailsNotReadyHelper = detailsFailed
    ? "Couldn't load the selected change requests, so this can't be changed here. Try again later."
    : "Loading the selected change requests…";
  const newOnlyHelper = !detailsReady
    ? detailsNotReadyHelper
    : allNew
      ? undefined
      : "Only when every selected change request is New: it is fixed once approval has been requested.";
  // Past New the deployments still change until approval, within the project: offered when all share one.
  const sharedProject = useMemo(() => {
    if (!detailsReady || allNew || open.length === 0) return null;
    const first = open[0].project;
    return first?.id && open.every((cr) => cr.project?.id === first.id) ? first : null;
  }, [detailsReady, allNew, open]);
  const scopeMode: "project" | "deployments" | "none" = allNew ? "project" : sharedProject ? "deployments" : "none";
  const sharedProjectId = sharedProject?.id;
  const sharedProjectName = sharedProject?.name;
  const { setProject: seedScopeProject } = scope;
  useEffect(() => {
    // The project shown, fixed, above the deployments: never sent changed.
    if (scopeMode === "deployments" && sharedProjectId) seedScopeProject(sharedProjectId, sharedProjectName);
  }, [scopeMode, sharedProjectId, sharedProjectName, seedScopeProject]);
  const scopeForFields = useMemo<ChangeRequestScope>(
    () => ({
      ...scope,
      setProject: (id: string, label?: string) => {
        setScopeTouched(true);
        scope.setProject(id, label);
      },
      setDeployments: (ids: string[]) => {
        setScopeTouched(true);
        scope.setDeployments(ids);
      },
    }),
    [scope],
  );
  const scopeHelper =
    scopeMode === "project"
      ? undefined
      : scopeMode === "deployments"
        ? "The Customer Project is fixed once approval has been requested; the deployments can still change, within it, until approval."
        : !detailsReady
          ? `Customer Project and deployments: ${detailsNotReadyHelper.charAt(0).toLowerCase()}${detailsNotReadyHelper.slice(1)}`
          : "The Customer Project changes only while every selected change request is New, and the deployments only while they share one project.";
  // What is sent: a project picked in New (with its deployments, even none), or past New new deployments in the shared project.
  const sendScope =
    scopeTouched &&
    !!scope.projectId &&
    (scopeMode === "project" || (scopeMode === "deployments" && scope.deploymentIds.length > 0));
  const emergency = isEmergencyChangeRequestType(type);
  const approvalChoices = customerStepChoices(open, "approval", type, scope.projectId);
  const reviewChoices = customerStepChoices(open, "review", type, scope.projectId);
  // A choice is kept only while it is offered (a type change can withdraw it): never sent unseen.
  const offered = (value: YesNoChoice, choices: { yes: boolean; no: boolean }): YesNoChoice =>
    detailsReady && !emergency && ((value === "yes" && choices.yes) || (value === "no" && choices.no)) ? value : NO_CHANGE;
  const effectiveApproval = offered(customerApproval, approvalChoices);
  const effectiveReview = offered(customerReview, reviewChoices);

  // Request Approval needs an assignment group, and (in New) a Customer Project when a customer step is ticked on
  // a non-Emergency change. A change request missing one is skipped by the run, so the field is marked required
  // while any selected one lacks it and the field is still empty: the plan takes a value set here first, so it
  // counts for all of them (for the project, one is set when it is sent: sendScope).
  const requestingApproval = state === "assess";
  const lackingTeamCount =
    requestingApproval && !assignedTeamId ? open.filter((cr) => !cr.assignedTeam?.id).length : 0;
  const lackingProjectCount = requestingApproval && !sendScope
    ? open.filter(
        (cr) =>
          isChangeRequestCreationPhase(cr.state) &&
          !cr.project?.id &&
          customerProjectNeededForApproval(
            type || cr.type,
            effectiveApproval ? effectiveApproval === "yes" : cr.customerApprovalRequired,
            effectiveReview ? effectiveReview === "yes" : cr.customerReviewRequired,
          ),
      ).length
    : 0;
  // The Customer Project can be set here only while every selected change request is New ("project"), so only then
  // is it marked required. Otherwise the picker is disabled, and the ones lacking one are counted in a plain note
  // under it instead.
  const projectRequired = scopeMode === "project" && lackingProjectCount > 0;
  /** "N of the selected change requests have no X": why a field is required to request approval. */
  const lackingHelper = (n: number, what: string): string =>
    `Required to request approval: ${n} of the selected change requests ${n === 1 ? "has" : "have"} ${what}, and without one ${n === 1 ? "it is" : "they are"} skipped.`;
  /** The same count, said plainly where the field can't be set here. */
  const lackingNote = (n: number, what: string): string =>
    `${n} of the selected change requests ${n === 1 ? "has" : "have"} ${what}, so ${n === 1 ? "it is" : "they are"} skipped.`;

  const startDate = parseDateTimeLocal(plannedStart);
  const endDate = parseDateTimeLocal(plannedEnd);

  const edit = useMemo<BulkChangeRequestEdit>(() => {
    const next: BulkChangeRequestEdit = {};
    if (state) next.state = state;
    if (title.trim()) next.title = title.trim();
    if (type) next.type = type;
    if (category) next.category = category;
    if (impact) next.impact = impact;
    if (priority) next.priority = priority;
    if (caseId) next.caseId = caseId;
    if (hold === "hold") {
      next.onHold = true;
      if (holdReason.trim()) next.onHoldReason = holdReason.trim();
    } else if (hold === "release") {
      next.onHold = false;
    }
    if (serviceId) next.serviceId = serviceId;
    if (serviceOfferingId) next.serviceOfferingId = serviceOfferingId;
    if (assignedTeamId) next.assignedTeamId = assignedTeamId;
    if (assignedEngineerId) next.assignedEngineerId = assignedEngineerId;
    if (requestedById) next.requestedById = requestedById;
    // The form holds wall-clock values in the user's timezone; the backend takes UTC.
    const startUtc = plannedStart ? zonedInputToBackendUtc(plannedStart) : null;
    if (startUtc) next.plannedStartOn = startUtc;
    const endUtc = plannedEnd ? zonedInputToBackendUtc(plannedEnd) : null;
    if (endUtc) next.plannedEndOn = endUtc;
    if (rollbackDuration.trim()) next.rollbackDurationText = rollbackDuration.trim();
    if (sendScope) {
      // Validated by the backend as a unit: the project, its deployments and their products go together.
      next.projectId = scope.projectId;
      next.deploymentIds = scope.deploymentIds;
      if (scope.productsReady) next.deploymentProductIds = scope.deploymentProductIds;
      // Past New the project is only the frame of the deployments: never moved to.
      if (scopeMode === "deployments") next.projectFixed = true;
    }
    if (effectiveApproval) next.customerApprovalRequired = effectiveApproval === "yes";
    if (effectiveReview) next.customerReviewRequired = effectiveReview === "yes";
    if (planVisible) next.isPlanningVisibleToCustomers = planVisible === "yes";
    // Sanitized on the way out, the same policy the detail page renders it back through.
    for (const f of PLAN_FIELDS) {
      if (!isBlankHtml(plans[f.key])) next[f.key] = sanitizeRichTextHtml(plans[f.key]);
    }
    if (serviceOutage.trim()) next.serviceOutage = serviceOutage.trim();
    if (communicationPlan.trim()) next.communicationPlan = communicationPlan.trim();
    if (workNote.trim()) next.workNote = workNote.trim();
    if (comment.trim()) next.comment = comment.trim();
    return next;
  }, [
    state, title, type, category, impact, priority, caseId, hold, holdReason, serviceId, serviceOfferingId,
    assignedTeamId, assignedEngineerId, requestedById, plannedStart, plannedEnd, rollbackDuration,
    sendScope, scopeMode, scope.projectId, scope.deploymentIds, scope.deploymentProductIds, scope.productsReady,
    effectiveApproval, effectiveReview, planVisible, plans, serviceOutage, communicationPlan, workNote, comment,
  ]);

  const problem = bulkChangeRequestEditProblem(edit);
  const productsPending = sendScope && !scope.productsReady;
  const canApply = Object.keys(edit).length > 0 && !problem && !productsPending && !isApplying;
  const count = changeRequests.length;
  const noun = `change request${count === 1 ? "" : "s"}`;

  const stateHelper = detailsLoading
    ? "Loading the moves each selected change request can make…"
    : detailsFailed
      ? "Couldn't load the selected change requests' moves, so the state can't be changed here. Try again later."
      : targets.length === 0
        ? "The selected change requests have no state they can all move to."
        : "Only the moves every selected change request can make.";

  /** A select whose first option is "No change" (the empty value), so an untouched field is never sent. */
  const select = <V extends string>(
    id: string,
    label: string,
    value: V | "",
    onChange: (v: V | "") => void,
    options: ReadonlyArray<{ value: V; label: string }>,
    opts: { helper?: string; disabled?: boolean } = {},
  ): JSX.Element => (
    <FormControl fullWidth size="small" disabled={isApplying || opts.disabled}>
      <InputLabel id={`${id}-label`} shrink>
        {label}
      </InputLabel>
      <Select
        labelId={`${id}-label`}
        label={label}
        value={value}
        displayEmpty
        onChange={(e) => onChange(String(e.target.value) as V | "")}
      >
        <MenuItem value={NO_CHANGE}>
          <Typography component="span" color="text.secondary">
            No change
          </Typography>
        </MenuItem>
        {options.map((o) => (
          <MenuItem key={o.value} value={o.value}>
            {o.label}
          </MenuItem>
        ))}
      </Select>
      {opts.helper && <FormHelperText>{opts.helper}</FormHelperText>}
    </FormControl>
  );

  /** One of the dialog's sections: a heading over its fields, set off from the one above by a divider. */
  const section = (heading: string, children: ReactNode, first = false): JSX.Element => (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {!first && <Divider />}
      <Typography variant="subtitle2" component="h3">
        {heading}
      </Typography>
      {children}
    </Box>
  );
  // Columns follow the dialog's own width (wide), so a field never squeezes below a readable size.
  const twoColumns = { display: "grid", gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" }, gap: 2 } as const;
  const threeColumns = { display: "grid", gridTemplateColumns: { xs: "1fr", md: "repeat(3, minmax(0, 1fr))" }, gap: 2 } as const;

  /** A customer requirement's options: only the values every selected change request can take. */
  const stepChoiceOptions = (choices: { yes: boolean; no: boolean }) =>
    [
      ...(choices.yes ? [{ value: "yes" as const, label: "Required" }] : []),
      ...(choices.no ? [{ value: "no" as const, label: "Not required" }] : []),
    ];
  /** Why a customer requirement offers less than both values (or nothing), in words. */
  const stepHelper = (choices: { yes: boolean; no: boolean }): string | undefined => {
    if (!detailsReady) return detailsNotReadyHelper;
    if (emergency) return EMERGENCY_CUSTOMER_STEPS_HELPER;
    if (!choices.yes && !choices.no) return "Can't be changed for all the selected change requests.";
    if (!choices.no) return "Can be added but not removed once approval has been requested.";
    if (!choices.yes) return "Can't be added for all of them: an Emergency change, a step already passed or no Customer Project.";
    return undefined;
  };

  return (
    <Dialog open onClose={isApplying ? undefined : onClose} maxWidth="lg" fullWidth>
      <DialogTitle>
        Edit {count} {noun}
      </DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
        <Typography variant="body2" color="text.secondary">
          Only the fields you change are applied; the rest keep each change request's own value.
        </Typography>
        {finishedCount > 0 && (
          <Alert severity="warning">
            {finishedCount} of the selected change requests {finishedCount === 1 ? "has" : "have"} been closed,
            canceled or rolled back since {finishedCount === 1 ? "it was" : "they were"} selected and will be skipped.
          </Alert>
        )}
        {approvedCount > 0 && approvedCount < editable.length && (
          <Alert severity="info">
            {approvedCount} of the selected change requests {approvedCount === 1 ? "has" : "have"} been approved: only the
            state, on hold, the work note and the additional comment apply to {approvedCount === 1 ? "it" : "them"},
            since an approved change request's fields are read-only.
          </Alert>
        )}
        {fieldsDisabled && !isApplying && (
          <Alert severity="info">
            All the selected change requests have been approved, so their fields are read-only: only the state, on hold
            and the notes can be changed.
          </Alert>
        )}

        {section(
          "Change request",
          <>
            <TextField
              id="bulk-cr-title"
              label="Short description"
              placeholder="No change"
              size="small"
              value={title}
              onChange={(e) => setTitle(e.target.value.slice(0, SUBJECT_MAX))}
              disabled={fieldsDisabled}
              slotProps={{ inputLabel: { shrink: true } }}
              helperText={
                title
                  ? `Sets the same short description on every selected change request. ${SUBJECT_MAX - title.length} characters left.`
                  : undefined
              }
            />
            <Box sx={threeColumns}>
              {select(
                "bulk-cr-state",
                "State",
                state,
                setState,
                targets.map((t) => ({ value: t, label: `${changeRequestTransitionLabel(t)} (${changeRequestStateLabel(t)})` })),
                { helper: stateHelper, disabled: !detailsReady || targets.length === 0 },
              )}
              {select(
                "bulk-cr-type",
                "Type",
                type,
                (v) => {
                  setType(v);
                  // An Emergency change has no customer steps: the run turns both off.
                  if (isEmergencyChangeRequestType(v)) {
                    setCustomerApproval(NO_CHANGE);
                    setCustomerReview(NO_CHANGE);
                  }
                },
                // Never Standard: a change becomes one only by being raised from a template.
                CHANGE_REQUEST_CREATE_TYPE_OPTIONS.filter((o) => o.value !== "standard"),
                {
                  disabled: fieldsDisabled || !allNew,
                  helper: emergency
                    ? "Emergency: both customer steps are turned off on every selected change request."
                    : newOnlyHelper,
                },
              )}
              {select("bulk-cr-category", "Category", category, setCategory, CHANGE_REQUEST_CATEGORY_OPTIONS, {
                disabled: fieldsDisabled,
              })}
              {select("bulk-cr-impact", "Impact", impact, setImpact, CHANGE_REQUEST_IMPACT_OPTIONS, {
                disabled: fieldsDisabled,
              })}
              {select("bulk-cr-priority", "Priority", priority, setPriority, CHANGE_REQUEST_PRIORITY_OPTIONS, {
                disabled: fieldsDisabled,
              })}
              <AsyncEntitySelect<BeCaseSearchView>
                id="bulk-cr-originating-service-request"
                label="Originating service request"
                placeholder="No change"
                shrinkLabel
                value={caseId}
                onChange={setCaseId}
                disabled={fieldsDisabled}
                useSearch={useSearchServiceRequestsForSelect}
                searchExtra={scope.projectId || undefined}
                getId={(c) => c.id}
                getLabel={(c) => parentRecordLabel({ id: c.id, number: c.number, subject: c.subject })}
              />
            </Box>
          </>,
          true,
        )}

        {section(
          "On hold",
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "minmax(0, 1fr) minmax(0, 2fr)" }, gap: 2 }}>
            {select(
              "bulk-cr-on-hold",
              "On hold",
              hold,
              (v) => {
                setHold(v);
                if (v !== "hold") setHoldReason("");
              },
              [
                { value: "hold", label: "Put on hold" },
                { value: "release", label: "Take off hold" },
              ],
              {
                helper:
                  onHoldCount > 0
                    ? `${onHoldCount} of the selected change requests ${onHoldCount === 1 ? "is" : "are"} on hold: a state change applies to ${onHoldCount === 1 ? "it" : "them"} only if this edit also takes ${onHoldCount === 1 ? "it" : "them"} off hold.`
                    : "While on hold a change request's state can't change.",
              },
            )}
            <TextField
              id="bulk-cr-on-hold-reason"
              label="On hold reason"
              placeholder={hold === "hold" ? "Why they are on hold" : "Put on hold first"}
              size="small"
              value={holdReason}
              onChange={(e) => setHoldReason(e.target.value)}
              disabled={isApplying || hold !== "hold"}
              slotProps={{ inputLabel: { shrink: true } }}
              helperText={
                hold === "hold"
                  ? "Replaces the reason on every selected change request; left empty, a new hold has no reason."
                  : undefined
              }
            />
          </Box>,
        )}

        {section(
          "Service",
          <Box sx={twoColumns}>
            <AsyncEntitySelect<BeItService>
              id="bulk-cr-service"
              label="Service"
              placeholder="No change"
              shrinkLabel
              value={serviceId}
              onChange={(v) => {
                setServiceId(v);
                setServiceOfferingId("");
              }}
              disabled={fieldsDisabled}
              useSearch={useSearchItServices}
              getId={(s) => s.id}
              getLabel={itServiceLabel}
            />
            <AsyncEntitySelect<BeServiceOffering>
              id="bulk-cr-service-offering"
              label="Service offering"
              placeholder="No change"
              shrinkLabel
              value={serviceOfferingId}
              onChange={setServiceOfferingId}
              disabled={fieldsDisabled || !serviceId}
              useSearch={useSearchServiceOfferings}
              searchExtra={serviceId || undefined}
              getId={(o) => o.id}
              getLabel={(o) => o.name}
              helperText={
                serviceId ? undefined : "Set a Service first: an offering must belong to the change requests' service."
              }
            />
          </Box>,
        )}

        {section(
          "Assignment",
          <Box sx={threeColumns}>
            <AsyncEntitySelect<BeGroup>
              id="bulk-cr-assigned-team"
              label="Assignment group"
              placeholder="No change"
              shrinkLabel
              value={assignedTeamId}
              onChange={setAssignedTeamId}
              disabled={fieldsDisabled}
              required={lackingTeamCount > 0}
              helperText={lackingTeamCount > 0 ? lackingHelper(lackingTeamCount, "no assignment group") : undefined}
              useSearch={useSearchGroups}
              getId={(g) => g.id}
              getLabel={(g) => g.name}
            />
            <AsyncEntitySelect<BeUser>
              id="bulk-cr-assigned-engineer"
              label="Assigned to"
              placeholder="No change"
              shrinkLabel
              value={assignedEngineerId}
              onChange={setAssignedEngineerId}
              disabled={fieldsDisabled}
              useSearch={useSearchInternalUsersByName}
              getId={(u) => u.id!}
              getLabel={userLabel}
            />
            <AsyncEntitySelect<BeUser>
              id="bulk-cr-requested-by"
              label="Requested by"
              placeholder="No change"
              shrinkLabel
              value={requestedById}
              onChange={setRequestedById}
              disabled={fieldsDisabled}
              useSearch={useSearchInternalUsersByName}
              getId={(u) => u.id!}
              getLabel={userLabel}
            />
          </Box>,
        )}

        {section(
          "Schedule",
          <Box sx={threeColumns}>
            <LocalizationProvider dateAdapter={AdapterDateFns}>
              <DateTimePicker
                label="Planned start"
                disabled={fieldsDisabled}
                value={startDate}
                onChange={(next) =>
                  setPlannedStart(next instanceof Date && !Number.isNaN(next.getTime()) ? formatDateTimeLocal(next) : "")
                }
                slotProps={{
                  field: { clearable: true },
                  textField: {
                    id: "bulk-cr-planned-start",
                    size: "small",
                    fullWidth: true,
                    placeholder: "No change",
                    // One date set alone: the other is each change request's own.
                    helperText: (plannedStart === "") !== (plannedEnd === "")
                      ? "Checked against each change request's own other planned date: one it wouldn't fit is skipped."
                      : undefined,
                  },
                }}
              />
              <DateTimePicker
                label="Planned end"
                disabled={fieldsDisabled}
                value={endDate}
                onChange={(next) =>
                  setPlannedEnd(next instanceof Date && !Number.isNaN(next.getTime()) ? formatDateTimeLocal(next) : "")
                }
                slotProps={{
                  field: { clearable: true },
                  textField: {
                    id: "bulk-cr-planned-end",
                    size: "small",
                    fullWidth: true,
                    placeholder: "No change",
                    error: !!startDate && !!endDate && endDate.getTime() <= startDate.getTime(),
                  },
                }}
              />
            </LocalizationProvider>
            <TextField
              id="bulk-cr-rollback-duration"
              label="Rollback duration"
              placeholder="No change"
              size="small"
              value={rollbackDuration}
              onChange={(e) => setRollbackDuration(e.target.value.slice(0, CHANGE_REQUEST_ROLLBACK_DURATION_MAX))}
              disabled={fieldsDisabled}
              slotProps={{ inputLabel: { shrink: true } }}
              helperText={
                rollbackDuration
                  ? `${CHANGE_REQUEST_ROLLBACK_DURATION_MAX - rollbackDuration.length} characters left.`
                  : "Free text, e.g. 30 mins."
              }
            />
          </Box>,
        )}

        {section(
          "Customer",
          <>
            <Box role="group" aria-label="Customer project and deployments" sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              {scopeHelper && (
                <Typography variant="caption" color="text.secondary">
                  {scopeHelper}
                </Typography>
              )}
              <ChangeRequestScopeFields
                scope={scopeForFields}
                disabled={fieldsDisabled || scopeMode === "none"}
                projectDisabled={scopeMode !== "project"}
                idPrefix="bulk-cr"
                projectClearable={scopeMode === "project"}
                projectRequired={projectRequired}
                projectHelperText={
                  projectRequired
                    ? lackingHelper(lackingProjectCount, "a customer step ticked and no Customer Project")
                    : undefined
                }
              />
              {lackingProjectCount > 0 && !projectRequired && (
                <Typography variant="caption" color="text.secondary">
                  {lackingNote(lackingProjectCount, "a customer step ticked and no Customer Project")}
                </Typography>
              )}
              {sendScope && (
                <Typography variant="caption" color="text.secondary">
                  {scopeMode === "project"
                    ? "Replaces each selected change request's Customer Project and deployments with these."
                    : "Replaces each selected change request's deployments with these."}
                </Typography>
              )}
            </Box>
            <Box sx={threeColumns}>
              {select(
                "bulk-cr-customer-approval",
                "Customer approval",
                effectiveApproval,
                setCustomerApproval,
                emergency ? [] : stepChoiceOptions(approvalChoices),
                {
                  disabled: fieldsDisabled || !detailsReady || emergency || (!approvalChoices.yes && !approvalChoices.no),
                  helper: stepHelper(approvalChoices),
                },
              )}
              {select(
                "bulk-cr-customer-review",
                "Customer review",
                effectiveReview,
                setCustomerReview,
                emergency ? [] : stepChoiceOptions(reviewChoices),
                {
                  disabled: fieldsDisabled || !detailsReady || emergency || (!reviewChoices.yes && !reviewChoices.no),
                  helper: stepHelper(reviewChoices),
                },
              )}
              {select(
                "bulk-cr-plan-visible",
                "Implementation Plan visible to customers",
                planVisible,
                setPlanVisible,
                [
                  { value: "yes", label: "Visible" },
                  { value: "no", label: "Not visible" },
                ],
                { disabled: fieldsDisabled },
              )}
            </Box>
          </>,
        )}

        {section(
          "Planning",
          <>
            <Typography variant="caption" color="text.secondary">
              Text written here replaces that field on every selected change request; an empty field is left as it is.
            </Typography>
            {PLAN_FIELDS.map((f) => (
              <ExpandableRichTextField
                key={f.key}
                id={`bulk-cr-${f.key}`}
                label={f.label}
                value={plans[f.key]}
                onChange={(html) => setPlans((p) => (p[f.key] === html ? p : { ...p, [f.key]: html }))}
                disabled={fieldsDisabled}
                placeholder={`No change. ${f.placeholder}`}
                minHeight={80}
              />
            ))}
            <Box sx={twoColumns}>
              <TextField
                id="bulk-cr-service-outage"
                label="Service outage / downtime"
                placeholder="No change"
                multiline
                minRows={2}
                size="small"
                value={serviceOutage}
                onChange={(e) => setServiceOutage(e.target.value)}
                disabled={fieldsDisabled}
                slotProps={{ inputLabel: { shrink: true } }}
              />
              <TextField
                id="bulk-cr-communication-plan"
                label="Communication plan"
                placeholder="No change"
                multiline
                minRows={2}
                size="small"
                value={communicationPlan}
                onChange={(e) => setCommunicationPlan(e.target.value)}
                disabled={fieldsDisabled}
                slotProps={{ inputLabel: { shrink: true } }}
              />
            </Box>
          </>,
        )}

        {section(
          "Notes",
          <Box sx={twoColumns}>
            <TextField
              id="bulk-cr-work-note"
              label={needsReason ? "Reason (internal work note)" : "Work note (internal)"}
              required={needsReason}
              multiline
              minRows={2}
              size="small"
              value={workNote}
              onChange={(e) => setWorkNote(e.target.value.slice(0, CHANGE_REQUEST_JOURNAL_MAX))}
              disabled={isApplying}
              helperText={
                needsReason
                  ? `Required to ${changeRequestTransitionLabel(state).toLowerCase()}: added to every selected change request as an internal work note, never shown to the customer.`
                  : "Added to every selected change request; never shown to the customer."
              }
            />
            <TextField
              id="bulk-cr-comment"
              label="Additional comment (customer-visible)"
              multiline
              minRows={2}
              size="small"
              value={comment}
              onChange={(e) => setComment(e.target.value.slice(0, CHANGE_REQUEST_JOURNAL_MAX))}
              disabled={isApplying}
              helperText="Added to every selected change request, and visible to the customer."
            />
          </Box>,
        )}

        {(problem || productsPending) && (
          <Alert severity="info" role="status">
            {problem ?? "Looking up the chosen deployments' products…"}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isApplying}>
          Cancel
        </Button>
        <Button variant="contained" onClick={() => onApply(edit)} disabled={!canApply}>
          {isApplying ? "Applying…" : `Apply to ${count} ${noun}`}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
