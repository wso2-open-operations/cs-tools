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
  Select,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { useCallback, useMemo, useState, type JSX } from "react";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchInternalUsersByName } from "@api/useSearchUsersByName";
import type {
  BeChangeRequestCategory,
  BeChangeRequestDetail,
  BeGroup,
  BePatchChangeRequestPayload,
  BeUser,
} from "@api/backend/types";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import ChangeRequestScopeFields, {
  ChangeRequestCustomerGroupField,
} from "@features/csm-operations/components/ChangeRequestScopeFields";
import { useChangeRequestScope } from "@features/csm-operations/hooks/useChangeRequestScope";
import Editor from "@components/rich-text-editor/Editor";
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
  changeRequestCategoryValue,
  changeRequestScopeLockedReason,
  customerApprovalLockedReason,
  customerProjectLockedReason,
  customerRequirementOnceSavedHelper,
  customerReviewLockedReason,
  EMERGENCY_CUSTOMER_STEPS_HELPER,
  isChangeRequestCreationPhase,
  isEmergencyChangeRequestType,
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
}

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
 * Edit the change-request fields the BE allows updating: the planned window,
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
 * Deliberately NOT here, even though the backend's write contract accepts
 * them: `priorityKey` (no metadata endpoint yet for the picker),
 * `comment`/`workNote` (these append journal entries, which the Comments tab
 * already covers), and `durationInput` (only succeeds against an exact-match
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
  const initialRollbackDurationText = cr.rollbackDurationText ?? "";
  const initialIsPlanningVisibleToCustomers = cr.isPlanningVisibleToCustomers ?? false;
  const initialCustomerApprovalRequired = cr.customerApprovalRequired ?? false;
  const initialCustomerReviewRequired = cr.customerReviewRequired ?? false;
  // Emergency: no customer steps (see the doc comment above). In New the form seeds
  // them OFF, so a leftover tick from before the rule is cleared by saving.
  const isEmergency = isEmergencyChangeRequestType(cr.type);
  const emergencyClearsCustomerSteps = isEmergency && isChangeRequestCreationPhase(cr.state);
  // After New the customer's part is add-only (see the rules in utils/changeRequests.ts):
  // a ticked box is read-only, an unticked one may still be ticked until the gate it
  // controls has passed and only when a Customer Project is set. The backend refuses
  // anything else (400) and stays the authority; the control is disabled up front
  // with the reason, computed here from the same inputs (state, stored value, project).
  const hasStoredProject = !!cr.project?.id;
  // An Emergency change's unticked box is off for the Emergency rule (the line under
  // the boxes), not for one of these reasons; a box a change of that type still has
  // ticked from before the rule keeps its add-only one.
  const customerApprovalLocked =
    isEmergency && !initialCustomerApprovalRequired
      ? null
      : customerApprovalLockedReason(cr.state, {
          stored: initialCustomerApprovalRequired,
          hasProject: hasStoredProject,
        });
  const customerReviewLocked =
    isEmergency && !initialCustomerReviewRequired
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
  const scope = useChangeRequestScope({
    projectId: cr.project?.id,
    projectLabel: cr.project?.name,
    deployments: cr.deployments?.map((d) => ({ id: d.id, label: d.name })),
    deploymentProducts: cr.deploymentProducts?.map((p) => ({ id: p.id, label: p.name })),
  });
  const [requestedById, setRequestedById] = useState(initialRequestedById);
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
  const rollbackPlan = useRichTextPlanField(cr.rollbackPlan);
  const testPlan = useRichTextPlanField(cr.testPlan);
  const implementationPlan = useRichTextPlanField(cr.implementationPlan);
  const affectedServicesText = useRichTextPlanField(cr.affectedServicesText);
  const affectedComponentsText = useRichTextPlanField(cr.affectedComponentsText);

  // Client-side only, and only when both ends are set: the backing system
  // does its own validation and this must not become the thing that blocks a
  // legitimate save, so it surfaces inline rather than being enforced
  // server-side.
  const startDate = parseDateTimeLocal(plannedStart);
  const endDate = parseDateTimeLocal(plannedEnd);
  const plannedEndBeforeStart =
    !!startDate && !!endDate && endDate.getTime() <= startDate.getTime();

  const patch = useMemo<BePatchChangeRequestPayload>(() => {
    const next: BePatchChangeRequestPayload = {};
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
    if (rollbackPlan.isDirty) next.rollbackPlan = rollbackPlan.outgoing;
    if (testPlan.isDirty) next.testPlan = testPlan.outgoing;
    if (implementationPlan.isDirty) next.implementationPlan = implementationPlan.outgoing;
    if (affectedServicesText.isDirty) next.affectedServicesText = affectedServicesText.outgoing;
    if (affectedComponentsText.isDirty) next.affectedComponentsText = affectedComponentsText.outgoing;
    if (rollbackDurationText !== initialRollbackDurationText) {
      next.rollbackDurationText = rollbackDurationText;
    }
    if (category !== initialCategory && category) {
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
    if (isPlanningVisibleToCustomers !== initialIsPlanningVisibleToCustomers) {
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
  ): JSX.Element => (
    <Box>
      <Typography
        id={`${id}-label`}
        variant="caption"
        color="text.secondary"
        sx={{ display: "block", mb: 0.5 }}
      >
        {label}
      </Typography>
      <Box role="group" aria-labelledby={`${id}-label`} aria-describedby={`${id}-help`}>
        <Editor
          value={field.initialHtml}
          onChange={field.onChange}
          minHeight={100}
          maxHeight={300}
          toolbarVariant="full"
          disabled={isSaving}
        />
      </Box>
      <FormHelperText id={`${id}-help`}>{helperText}</FormHelperText>
    </Box>
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
        disabled={isSaving || !!lockedReason || isEmergency}
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

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit change request</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {saveError && (
            <Alert severity="error" sx={{ width: "100%" }}>
              {saveError}
            </Alert>
          )}
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
            disabled={isSaving}
            useSearch={useSearchGroups}
            getId={(g) => g.id}
            getLabel={(g) => g.name}
            knownLabel={cr.assignedTeam?.name}
            helperText="Required before moving to Assess — its members become the Assess-stage approvers."
          />
          <AsyncEntitySelect<BeUser>
            id="cr-edit-assigned-engineer"
            label="Assigned to"
            placeholder="Search people…"
            value={assignedEngineerId}
            onChange={setAssignedEngineerId}
            disabled={isSaving}
            useSearch={useSearchInternalUsersByName}
            getId={(u) => u.id!}
            getLabel={userLabel}
            knownLabel={cr.assignedEngineer?.name}
          />
          <AsyncEntitySelect<BeUser>
            id="cr-edit-requested-by"
            label="Requested by"
            placeholder="Search people…"
            value={requestedById}
            onChange={setRequestedById}
            disabled={isSaving}
            useSearch={useSearchInternalUsersByName}
            getId={(u) => u.id!}
            getLabel={userLabel}
            knownLabel={cr.requestedBy?.name}
          />
          <Box
            role="group"
            aria-label="Customer project and deployments"
            sx={{ display: "flex", flexDirection: "column", gap: 1 }}
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
              disabled={isSaving || !!scopeLocked}
              projectDisabled={!!projectLocked}
              idPrefix="cr-edit"
              // A saved project can be swapped for another but not removed —
              // the patch has no way to express "no project".
              projectClearable={!initialProjectId}
            />
            <ChangeRequestCustomerGroupField scope={scope} idPrefix="cr-edit" />
          </Box>
          <FormControl fullWidth size="small" disabled={isSaving}>
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
          </FormControl>
          {renderCustomerStepCheckbox(
            "cr-edit-customer-approval",
            "Customer Approval",
            "Adds a customer approval step after internal approval, before scheduling.",
            customerApprovalRequired,
            setCustomerApprovalRequired,
            customerApprovalLocked,
            isEmergency ? null : customerRequirementOnceSavedHelper(cr.state, initialCustomerApprovalRequired),
          )}
          {renderCustomerStepCheckbox(
            "cr-edit-customer-review",
            "Customer Review",
            "Adds a customer review step after Review, before closing.",
            customerReviewRequired,
            setCustomerReviewRequired,
            customerReviewLocked,
            isEmergency ? null : customerRequirementOnceSavedHelper(cr.state, initialCustomerReviewRequired),
          )}
          {isEmergency && (
            <FormHelperText id={EMERGENCY_NOTE_ID} sx={{ mx: 0 }}>
              {EMERGENCY_CUSTOMER_STEPS_HELPER}
            </FormHelperText>
          )}
          <TextField
            label="Rollback duration"
            value={rollbackDurationText}
            onChange={(e) => setRollbackDurationText(e.target.value)}
            fullWidth
            size="small"
            disabled={isSaving}
            placeholder="e.g. 30 mins"
            helperText="Free text — ServiceNow does not parse this into a structured duration."
          />
          <FormControlLabel
            sx={{ ml: 0, justifyContent: "space-between", width: "100%" }}
            labelPlacement="start"
            control={
              <Switch
                size="small"
                checked={isPlanningVisibleToCustomers}
                onChange={(e) => setIsPlanningVisibleToCustomers(e.target.checked)}
                disabled={isSaving}
                inputProps={{ "aria-label": "Implementation Plan visible to customers" }}
              />
            }
            label={
              <Typography variant="body2" color="text.secondary">
                Implementation Plan visible to customers
              </Typography>
            }
          />
          {renderPlanField(
            "cr-edit-implementation-plan",
            "Implementation plan",
            implementationPlan,
            "How this change is carried out.",
          )}
          {renderPlanField(
            "cr-edit-rollback-plan",
            "Rollback plan",
            rollbackPlan,
            "How this change is backed out if it goes wrong.",
          )}
          {renderPlanField(
            "cr-edit-test-plan",
            "Test plan",
            testPlan,
            "How the change is verified once implemented.",
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
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onSave(patch)}
          disabled={isSaving || !hasChanges || plannedEndBeforeStart}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
