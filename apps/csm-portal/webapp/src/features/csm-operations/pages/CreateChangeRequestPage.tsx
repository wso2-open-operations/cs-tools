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
  alpha,
  Box,
  Button,
  Card,
  DatePickers,
  FormControl,
  FormControlLabel,
  InputLabel,
  MenuItem,
  Select,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";

const { DateTimePicker, LocalizationProvider } = DatePickers;
import { ArrowLeft, Link2 } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useRef, useState, type JSX } from "react";
import { useLocation, useNavigate } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import Editor from "@components/rich-text-editor/Editor";
import { isBlankHtml } from "@utils/sanitizeHtml";
import { isPastDateTime } from "@utils/dateTime";
import { usePostChangeRequest } from "@features/csm-operations/api/usePostChangeRequest";
import { usePatchChangeRequest } from "@features/csm-operations/api/usePatchChangeRequest";
import { useGetUsersMe } from "@features/settings/api/useGetUsersMe";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchInternalUsersByName } from "@api/useSearchUsersByName";
import { useSearchParentRecordsForSelect } from "@features/csm-operations/api/useSearchParentRecordsForSelect";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import {
  changeRequestDraftKey,
  changeRequestStateLabel,
  clearChangeRequestDraft,
  CLONE_SOURCE_GAP_MESSAGE,
  decodeParentRecordValue,
  encodeParentRecordValue,
  loadChangeRequestDraft,
  parentRecordLabel,
  saveChangeRequestDraft,
  type ChangeRequestDraftContext,
  type CloneChangeRequestNavState,
  type CreateChangeRequestFromIncidentNavState,
  type ParentRecordOption,
} from "@features/csm-operations/utils/changeRequests";
import type { CreateChangeRequestFromCaseNavState } from "@features/csm-cases/types/csmCases";
import type {
  BeChangeRequestImpact,
  BeChangeRequestPriority,
  BeChangeRequestState,
  BeChangeRequestType,
  BeCreateChangeRequestPayload,
  BeGroup,
  BeUser,
} from "@api/backend/types";

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

// Field limits and defaults below mirror the legacy ServiceNow "Create New
// Change Request" form (Short description: 500 chars; Type/Category/Impact/
// Risk pre-selected rather than left blank). The Planning journal fields
// (Description onward) are ServiceNow rich-text fields — no character cap
// here, since naively truncating HTML at a fixed offset risks cutting a tag
// in half and corrupting the markup; the backend rejects an overlong
// submission with a real error instead (see handleSubmit's onError).
const SUBJECT_MAX = 500;

const TYPE_OPTIONS: Array<{ value: BeChangeRequestType; label: string }> = [
  { value: "standard", label: "Standard" },
  { value: "normal", label: "Normal" },
  { value: "emergency", label: "Emergency" },
  { value: "model", label: "Model" },
  { value: "site_reliability_ops", label: "Site reliability ops" },
  { value: "azure", label: "Azure" },
];

const IMPACT_OPTIONS: Array<{ value: BeChangeRequestImpact; label: string }> = [
  { value: "high", label: "High" },
  { value: "medium", label: "Medium" },
  { value: "low", label: "Low" },
];

const PRIORITY_OPTIONS: Array<{ value: BeChangeRequestPriority; label: string }> = [
  { value: "critical", label: "Critical" },
  { value: "high", label: "High" },
  { value: "moderate", label: "Moderate" },
  { value: "low", label: "Low" },
];

// Only the pre-workflow states are selectable at creation. A new change
// request must enter its lifecycle at the start (new/assess/authorize) and
// move forward from there — creating one already Closed/Cancelled, or straight
// into Implement, would skip its own assess → authorize → approval workflow.
// Defaults to "new" — the state SN itself defaults a fresh CR to. Labels reuse
// the same map the list/detail pages show, so they read consistently.
const CREATE_STATE_VALUES: BeChangeRequestState[] = ["new", "assess", "authorize"];
const STATE_OPTIONS: Array<{ value: BeChangeRequestState; label: string }> =
  CREATE_STATE_VALUES.map((s) => ({ value: s, label: changeRequestStateLabel(s) }));

// Option labels for this form's pickers. Each falls back down to the record id
// rather than rendering blank, so an option is always selectable even when the
// backing record carries none of the friendlier fields.

/** Display label for a user option: full name, else email, else id. */
function userLabel(u: BeUser): string {
  return [u.firstName, u.lastName].filter(Boolean).join(" ").trim() || u.email || u.id || "";
}

/** `datetime-local` input value ("YYYY-MM-DDTHH:MM") to the BE's expected
 * "YYYY-MM-DD HH:MM:SS" string. */
function toBackendDateTime(localValue: string): string {
  return `${localValue.replace("T", " ")}:00`;
}

/** "YYYY-MM-DDTHH:MM" (the wire format this form's state still uses) to a
 * local Date, avoiding the UTC-parse day/hour shift a plain `new Date(value)`
 * risks depending on the viewer's timezone. */
function parseDateTimeLocal(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(
    Number(match[1]),
    Number(match[2]) - 1,
    Number(match[3]),
    Number(match[4]),
    Number(match[5]),
  );
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Local Date back to "YYYY-MM-DDTHH:MM", matching toBackendDateTime's input. */
function formatDateTimeLocal(date: Date): string {
  const y = date.getFullYear();
  const mo = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  const h = String(date.getHours()).padStart(2, "0");
  const mi = String(date.getMinutes()).padStart(2, "0");
  return `${y}-${mo}-${d}T${h}:${mi}`;
}

/** "Characters left: N" once a field is more than half full, matching the
 * legacy form's live counter — not shown for an empty/lightly-used field. */
function charsLeftHelper(value: string, max: number): string | undefined {
  return value.length >= max / 2 ? `Characters left: ${max - value.length}` : undefined;
}

const OPERATIONS_CHANGE_REQUESTS_PATH = "/operations?tab=change_requests";

export default function CreateChangeRequestPage(): JSX.Element {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();
  const postChangeRequest = usePostChangeRequest();
  const patchChangeRequest = usePatchChangeRequest();

  // This form can be opened three ways, each carrying its own router state
  // (not query params) — read once: this form's state is what the user edits
  // from here on, so a later change to the *source* record must not reach
  // back in and overwrite what they've typed. The three shapes are mutually
  // exclusive; narrow on `caseId`/`incidentId`, the fields only the latter two
  // carry.
  //   - Clone, from a change request's "Clone" action — see
  //     CsmChangeRequestDetailPage.tsx's cloneChangeRequest and
  //     buildCloneChangeRequestNavState's doc comment for exactly which
  //     fields this can and can't carry over.
  //   - A service request's own "Create change request…" action — see
  //     CsmCaseDetailPage.tsx's create_change_request handler and
  //     CreateChangeRequestFromCaseNavState's doc comment. Carries the
  //     originating service request (and its project, for scoping the
  //     picker's search) so the "Originating service request" field below
  //     starts pre-selected rather than blank.
  //   - An incident's own "Create change request…" action — see
  //     CsmIncidentDetailPage.tsx and CreateChangeRequestFromIncidentNavState's
  //     doc comment. Pre-selects that incident as the intended parent, but see
  //     `isIncidentParentSelected` below: submitting with an incident selected
  //     is gated until the backend accepts one.
  const location = useLocation();
  const locationState = location.state as
    | CloneChangeRequestNavState
    | CreateChangeRequestFromCaseNavState
    | CreateChangeRequestFromIncidentNavState
    | undefined;
  const cloneState =
    locationState && !("caseId" in locationState) && !("incidentId" in locationState)
      ? locationState
      : undefined;
  const fromCaseState =
    locationState && "caseId" in locationState ? locationState : undefined;
  const fromIncidentState =
    locationState && "incidentId" in locationState ? locationState : undefined;

  // Set when opened from a list/detail page's own "Create change request"
  // action with `state: { from: ... }` (same convention as the 4 case-type
  // create pages), so Back/Cancel return there instead of the hardcoded
  // change-requests tab, and the newly created change request's own Back
  // button (reading this same convention) returns there too.
  const backState = location.state as { from?: string } | undefined;
  const backTarget = backState?.from ?? OPERATIONS_CHANGE_REQUESTS_PATH;

  // This route unmounts (losing all local state) whenever the user navigates
  // to another operations tab, and remounts fresh on the way back — without
  // this, that remount re-seeds every field from the *original*
  // cloneState/fromCaseState/fromIncidentState again, discarding anything
  // typed in between. `draftKey` scopes a sessionStorage draft to this exact
  // entry context (see `changeRequestDraftKey`'s doc comment for why); `draft`
  // is read once, at mount, via a lazy `useState` initializer — every field
  // below seeds from it in preference to the nav-state source when present.
  const draftContext: ChangeRequestDraftContext = cloneState
    ? { kind: "clone", sourceNumber: cloneState.sourceNumber }
    : fromCaseState
      ? { kind: "case", caseId: fromCaseState.caseId }
      : fromIncidentState
        ? { kind: "incident", incidentId: fromIncidentState.incidentId }
        : { kind: "new" };
  const draftKey = changeRequestDraftKey(draftContext);
  const [draft] = useState(() => loadChangeRequestDraft(draftKey));

  // Slice on seed as well as on change: a source record at or beyond the cap
  // would otherwise load untrimmed, show a negative characters-left count, and
  // submit over-length if the user never edits the field.
  const [subject, setSubject] = useState(
    draft?.subject ?? (cloneState?.subject ?? "").slice(0, SUBJECT_MAX),
  );
  // Pre-selected to match the legacy ServiceNow form's own defaults, rather
  // than leaving every dropdown blank — most change requests are Normal
  // type, Low impact. Priority has no default there either ("-- None --"),
  // so it stays unset here too. A clone carries over `type`/`impact` from
  // the source record when present; priority has no source value to carry
  // (see buildCloneChangeRequestNavState), so it keeps the same default a
  // from-scratch change request gets. `category` and `risk` are not
  // editable here at all — see BeCreateChangeRequestPayload's doc comment:
  // `category` is 99.9% left at its default on real records and `risk`
  // isn't a field on the real ServiceNow CR form.
  const [type, setType] = useState<string>(draft?.type ?? cloneState?.type ?? "normal");
  const [impact, setImpact] = useState<string>(draft?.impact ?? cloneState?.impact ?? "low");
  const [priority, setPriority] = useState<string>(draft?.priority ?? UNSET);
  // Always "new" regardless of the source record's own state/schedule/
  // approval — cloning must never carry an approval or a stale window
  // across into the new change request. A restored draft is the one
  // exception: it reflects wherever the user's own in-progress edit left this
  // field (still just "new"/"assess"/"authorize" — the same options remain
  // selectable either way), not the clone source's state.
  const [state, setState] = useState<string>(draft?.state ?? "new");
  const [plannedStartDate, setPlannedStartDate] = useState(draft?.plannedStartDate ?? "");
  const [plannedEndDate, setPlannedEndDate] = useState(draft?.plannedEndDate ?? "");
  const [description, setDescription] = useState(draft?.description ?? cloneState?.description ?? "");
  const [justification, setJustification] = useState(
    draft?.justification ?? cloneState?.justification ?? "",
  );
  const [implementationPlan, setImplementationPlan] = useState(draft?.implementationPlan ?? "");
  const [riskImpactAnalysis, setRiskImpactAnalysis] = useState(draft?.riskImpactAnalysis ?? "");
  const [backoutPlan, setBackoutPlan] = useState(draft?.backoutPlan ?? "");
  const [testPlan, setTestPlan] = useState(draft?.testPlan ?? cloneState?.testPlan ?? "");
  const [isPlanningVisibleToCustomers, setIsPlanningVisibleToCustomers] = useState(
    draft?.isPlanningVisibleToCustomers ?? false,
  );
  const [groupId, setGroupId] = useState(draft?.groupId ?? "");
  const [assignedEngineerId, setAssignedEngineerId] = useState(
    draft?.assignedEngineerId ?? cloneState?.assignedEngineerId ?? "",
  );
  const [requestedById, setRequestedById] = useState(draft?.requestedById ?? "");
  // The service request or incident this change request was raised from,
  // when picked — encoded as `"sr:<id>"`/`"inc:<id>"` (see
  // `encodeParentRecordValue`) since the underlying picker searches both
  // kinds of record at once and a bare id can't otherwise say which kind it
  // is. Not part of BeCreateChangeRequestPayload — see handleSubmit's
  // comment. Pre-selected when opened from a service request's or an
  // incident's own "Create change request…" action; stays fully editable
  // from there — the field remains a normal AsyncEntitySelect, not a locked/
  // read-only control, so a wrong pre-fill (or a genuine need to link a
  // different record instead) can still be corrected without leaving the
  // form.
  const [parentValue, setParentValue] = useState(
    draft?.parentValue ??
      (fromCaseState
        ? encodeParentRecordValue("service_request", fromCaseState.caseId)
        : fromIncidentState
          ? encodeParentRecordValue("incident", fromIncidentState.incidentId)
          : ""),
  );
  // Decoded once per render — `undefined` when nothing is selected or the
  // value doesn't parse (never expected in practice, but AsyncEntitySelect's
  // `value` is a plain string so this stays defensive).
  const parentSelection = decodeParentRecordValue(parentValue);
  // The live `PATCH /change-requests/{id}` write only ever resolves `caseId`
  // against the case table (see this file's own header note and
  // `changeRequests.ts`'s "Originating service request picker" section for
  // the verified-live reason) — an incident result can be found by the
  // picker, but never submitted, until the backend adds a path for it.
  const isIncidentParentSelected = parentSelection?.kind === "incident";
  // Display label for a pre-filled `parentValue` above until a fresh search
  // for the same id resolves one from the backend (see AsyncEntitySelect's
  // `knownLabel`).
  const fromCaseKnownLabel = fromCaseState
    ? parentRecordLabel({
        id: fromCaseState.caseId,
        number: fromCaseState.caseNumber,
        subject: fromCaseState.caseSubject,
      })
    : undefined;
  const fromIncidentKnownLabel = fromIncidentState
    ? parentRecordLabel({
        id: fromIncidentState.incidentId,
        number: fromIncidentState.incidentNumber,
        subject: fromIncidentState.incidentSubject,
      })
    : undefined;

  // Defaults "Requested by" to the signed-in user, matching the legacy
  // ServiceNow form's own behaviour (usePostChangeRequest.ts/BE doesn't do
  // this itself — see BeCreateChangeRequestPayload's doc comment). Fires
  // once, when the current user's id first loads; a ref (not the field's own
  // emptiness) gates it so manually clearing the field afterward sticks.
  // Adjusted during render (React's recommended pattern for this) rather
  // than in an effect, which would call setState synchronously post-commit.
  // Starts already "done" when restoring a draft — the restored
  // `requestedById` already reflects whatever this field held (auto-filled,
  // cleared, or reassigned) when the user last edited it, and auto-fill
  // running again here would stomp a deliberate clear.
  const { data: me } = useGetUsersMe();
  const meLabel = me ? userLabel(me) : undefined;
  const autoFilledRequester = useRef(draft !== null);
  if (me?.id && !autoFilledRequester.current) {
    autoFilledRequester.current = true;
    setRequestedById(me.id);
  }

  // Writes the form's current values back to this entry context's draft on
  // every change, so a later unmount/remount (switching operations tabs and
  // back) restores them instead of re-seeding from the original clone/case/
  // incident source. See `changeRequestDraftKey`'s doc comment for why the
  // key is scoped per entry context.
  useEffect(() => {
    saveChangeRequestDraft(draftKey, {
      subject,
      type,
      impact,
      priority,
      state,
      plannedStartDate,
      plannedEndDate,
      description,
      justification,
      implementationPlan,
      riskImpactAnalysis,
      backoutPlan,
      testPlan,
      isPlanningVisibleToCustomers,
      groupId,
      assignedEngineerId,
      requestedById,
      parentValue,
    });
  }, [
    draftKey,
    subject,
    type,
    impact,
    priority,
    state,
    plannedStartDate,
    plannedEndDate,
    description,
    justification,
    implementationPlan,
    riskImpactAnalysis,
    backoutPlan,
    testPlan,
    isPlanningVisibleToCustomers,
    groupId,
    assignedEngineerId,
    requestedById,
    parentValue,
  ]);

  const isSubmitting = postChangeRequest.isPending || patchChangeRequest.isPending;
  // `isIncidentParentSelected` blocks submit entirely rather than just
  // skipping the PATCH — see its own doc comment above for why sending the
  // create-then-PATCH flow through with an incident's id would 404.
  const canSubmit = subject.trim().length > 0 && !isSubmitting && !isIncidentParentSelected;
  // Non-blocking: a past planned start/end is unusual but not forbidden
  // (e.g. logging a change that already happened), so this only warns.
  const plannedStartIsPast = isPastDateTime(parseDateTimeLocal(plannedStartDate));
  const plannedEndIsPast = isPastDateTime(parseDateTimeLocal(plannedEndDate));

  const handleSubmit = (): void => {
    if (!canSubmit) return;

    const payload: BeCreateChangeRequestPayload = { subject: subject.trim() };
    if (type) payload.type = type as BeChangeRequestType;
    if (impact) payload.impact = impact as BeChangeRequestImpact;
    if (priority) payload.priority = priority as BeChangeRequestPriority;
    if (state) payload.state = state as BeChangeRequestState;
    if (plannedStartDate) payload.plannedStartDate = toBackendDateTime(plannedStartDate);
    if (plannedEndDate) payload.plannedEndDate = toBackendDateTime(plannedEndDate);
    // These six are rich-text HTML from Editor, not plain strings — an
    // untouched editor still produces non-empty-looking HTML (e.g.
    // "<p><br></p>"), so `.trim()` truthiness would send blank content as
    // if it were real. isBlankHtml is the same check the detail page uses
    // to decide whether to render a plan section at all.
    if (!isBlankHtml(description)) payload.description = description;
    if (!isBlankHtml(justification)) payload.justification = justification;
    if (!isBlankHtml(implementationPlan)) payload.implementationPlan = implementationPlan;
    if (!isBlankHtml(riskImpactAnalysis)) payload.riskImpactAnalysis = riskImpactAnalysis;
    if (!isBlankHtml(backoutPlan)) payload.backoutPlan = backoutPlan;
    if (!isBlankHtml(testPlan)) payload.testPlan = testPlan;
    payload.isPlanningVisibleToCustomers = isPlanningVisibleToCustomers;
    if (groupId.trim()) payload.groupId = groupId.trim();
    if (assignedEngineerId.trim()) payload.assignedEngineerId = assignedEngineerId.trim();
    if (requestedById.trim()) payload.requestedById = requestedById.trim();

    postChangeRequest.mutate(payload, {
      onSuccess: (created) => {
        // The change request this draft was building now exists — drop it so
        // a later visit to this same entry context (e.g. cloning the same
        // source record again) starts clean rather than restoring this
        // already-submitted content. Cleared regardless of how the follow-up
        // PATCH below resolves; that's a separate, already-created record's
        // linkage, not a reason to keep this draft around.
        clearChangeRequestDraft(draftKey);
        const createdId = created.changeRequest.id;
        // POST /change-requests can't carry the originating-service-request
        // link (it isn't an accepted create field), so it's set with a
        // follow-up PATCH once the change request exists. A failed PATCH
        // still leaves a valid, created change request — navigate there
        // regardless, but surface the link failure rather than hiding it.
        // `canSubmit` already blocks this branch from ever running with an
        // incident selected, so `parentSelection` here is either unset or a
        // service request.
        const resolvedCaseId =
          parentSelection?.kind === "service_request" ? parentSelection.id : undefined;
        if (!resolvedCaseId) {
          navigate(`/operations/change-requests/${createdId}`, {
            state: { from: backTarget },
          });
          return;
        }
        patchChangeRequest.mutate(
          { id: createdId, patch: { caseId: resolvedCaseId } },
          {
            onSuccess: () =>
              navigate(`/operations/change-requests/${createdId}`, {
                state: { from: backTarget },
              }),
            onError: () => {
              showError(
                "The change request was created, but linking it to the originating service request failed. The change request itself is unaffected; the link is not set.",
              );
              navigate(`/operations/change-requests/${createdId}`, {
                state: { from: backTarget },
              });
            },
          },
        );
      },
      onError: (err) => {
        // The backend surfaces real validation messages on 4xx (e.g. an
        // invalid UUID in one of the advanced ID fields); show them.
        const msg =
          err instanceof BackendApiError && err.status < 500 && err.message
            ? err.message
            : "Could not create the change request. Please try again.";
        showError(msg, err);
      },
    });
  };

  // Explicit "I'm abandoning this" signal from the user, unlike navigating to
  // another operations tab mid-edit (which the draft above exists to survive)
  // — Back and Cancel both drop the draft before leaving, so returning to
  // this same entry context later starts clean.
  const handleCancel = (): void => {
    clearChangeRequestDraft(draftKey);
    navigate(backTarget);
  };

  // Shared renderer for a "-- Select --" dropdown, matching the pattern used
  // for optional enum fields in CreateGithubIssueDialog.
  const renderSelect = (
    id: string,
    label: string,
    value: string,
    onChange: (v: string) => void,
    options: Array<{ value: string; label: string }>,
  ): JSX.Element => (
    <FormControl fullWidth size="small" disabled={isSubmitting}>
      <InputLabel id={`${id}-label`} shrink>
        {label}
      </InputLabel>
      <Select
        labelId={`${id}-label`}
        label={label}
        value={value}
        displayEmpty
        onChange={(e) => onChange(String(e.target.value))}
      >
        <MenuItem value={UNSET}>
          <Typography component="span" color="text.secondary">
            {SELECT_PLACEHOLDER}
          </Typography>
        </MenuItem>
        {options.map((o) => (
          <MenuItem key={o.value} value={o.value}>
            {o.label}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );

  // Shared renderer for a Planning rich-text field. Editor doesn't accept an
  // `id`/native label association, so the visible label is a separate
  // Typography and the pair is tied together via role="group" +
  // aria-labelledby for assistive tech (matches CsmCaseCreatePage's own
  // Description field, the other place this editor is used for a form
  // field rather than a comment box).
  const renderEditorField = (
    id: string,
    label: string,
    value: string,
    onChange: (v: string) => void,
    placeholder?: string,
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
      <Box role="group" aria-labelledby={`${id}-label`}>
        <Editor
          value={value}
          onChange={onChange}
          placeholder={placeholder}
          minHeight={100}
          maxHeight={300}
          toolbarVariant="full"
          disabled={isSubmitting}
        />
      </Box>
    </Box>
  );

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={handleCancel}
        sx={{ mb: 1 }}
      >
        Back
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New change request
      </Typography>

      {cloneState && (
        <Alert severity="info" sx={{ mb: 2 }}>
          {cloneState.sourceNumber
            ? `Cloned from ${cloneState.sourceNumber}. `
            : "Cloned from an existing change request. "}
          {CLONE_SOURCE_GAP_MESSAGE}
        </Alert>
      )}
      {fromCaseState && (
        <Alert severity="info" sx={{ mb: 2 }}>
          Linking to {fromCaseState.caseNumber ?? "the service request"} — its id is
          carried through automatically as the Originating service request field below.
        </Alert>
      )}
      {fromIncidentState && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          Opened from {fromIncidentState.incidentNumber ?? "an incident"} — it's pre-filled below
          as the intended parent, but linking a change request directly to an incident isn't
          supported by the backend yet. You'll need to remove it (or link a service request
          instead) before this form can be submitted.
        </Alert>
      )}

      <Card variant="outlined" sx={{ p: 3 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Change request</Typography>

          <TextField
            label="Subject"
            value={subject}
            onChange={(e) => setSubject(e.target.value.slice(0, SUBJECT_MAX))}
            fullWidth
            required
            disabled={isSubmitting}
            placeholder="Short summary of the change"
            helperText={charsLeftHelper(subject, SUBJECT_MAX)}
          />

          {/* Deliberately called out with its own bordered/tinted panel near
              the top of the form, not just another inlined field — this used
              to sit at the bottom of a collapsed "More options" section,
              where it was easy to skip entirely. The change request/service
              request link has no way to be added back in after creation
              except through this same field on the detail page's edit
              dialog, so a user who skips it here has to notice and fix that
              gap later. The treatment (bordered box, tinted background,
              icon + heavier label) mirrors the emphasis ProjectSelectionField
              gives its own "must not get this wrong" field. */}
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              gap: 1,
              p: 2,
              borderRadius: 1,
              border: "1px solid",
              borderColor: "primary.main",
              bgcolor: (theme) => alpha(theme.palette.primary.main, 0.06),
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Box sx={{ display: "flex", color: "primary.main" }}>
                <Link2 size={16} aria-hidden />
              </Box>
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                Originating service request or incident
              </Typography>
            </Box>
            <Typography variant="body2" color="text.secondary">
              {fromCaseState
                ? "Pre-filled from the service request you opened this from — change it below if that's not right."
                : fromIncidentState
                  ? "Pre-filled from the incident you opened this from — change it below if that's not right."
                  : "If this change request was raised from a service request or incident, link it here — search by CS or INC number. It's much harder to find and add later."}
            </Typography>
            <AsyncEntitySelect<ParentRecordOption>
              id="cr-originating-service-request"
              label="Originating service request or incident"
              placeholder="Search service requests or incidents…"
              value={parentValue}
              onChange={setParentValue}
              disabled={isSubmitting}
              // Opened from a service request's own "Create change
              // request…" action: its project is threaded through as
              // `searchExtra` so the search prefers service requests
              // from the same project first (see
              // useSearchServiceRequestsForSelect's doc comment for how
              // that stays additive, not a hard filter). Opened any
              // other way (this page's own "New change request" entry
              // point, a Clone, or an incident's own entry point) there's
              // no case context at all, so `searchExtra` is undefined and
              // the search stays exactly the unscoped, system-wide search
              // it's always been.
              useSearch={useSearchParentRecordsForSelect}
              searchExtra={fromCaseState?.projectId}
              getId={(o) => encodeParentRecordValue(o.kind, o.id)}
              getLabel={parentRecordLabel}
              knownLabel={fromCaseKnownLabel ?? fromIncidentKnownLabel}
            />
            {isIncidentParentSelected && (
              <Alert severity="warning">
                Linking a change request directly to an incident isn&apos;t available yet —
                pending a backend change. Submitting is disabled while an incident is selected
                here; pick a service request instead, or clear this field, to continue.
              </Alert>
            )}
          </Box>

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 200px" }}>
              {renderSelect("cr-type", "Type", type, setType, TYPE_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 200px" }}>
              {renderSelect("cr-priority", "Priority", priority, setPriority, PRIORITY_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 200px" }}>
              {renderSelect("cr-impact", "Impact", impact, setImpact, IMPACT_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 200px" }}>
              {renderSelect("cr-state", "State", state, setState, STATE_OPTIONS)}
            </Box>
          </Box>

          <Typography variant="subtitle2" sx={{ mt: 1 }}>
            Planning
          </Typography>

          {renderEditorField("cr-description", "Description", description, setDescription, "What is changing?")}
          {renderEditorField(
            "cr-justification",
            "Justification",
            justification,
            setJustification,
            "Why is this change needed?",
          )}
          {renderEditorField(
            "cr-implementation-plan",
            "Implementation plan",
            implementationPlan,
            setImplementationPlan,
          )}
          {renderEditorField(
            "cr-risk-impact-analysis",
            "Risk and impact analysis",
            riskImpactAnalysis,
            setRiskImpactAnalysis,
          )}
          {renderEditorField("cr-backout-plan", "Backout plan", backoutPlan, setBackoutPlan)}
          {renderEditorField("cr-test-plan", "Test plan", testPlan, setTestPlan)}

          <FormControlLabel
            sx={{ ml: 0, justifyContent: "space-between", width: "100%" }}
            labelPlacement="start"
            control={
              <Switch
                size="small"
                checked={isPlanningVisibleToCustomers}
                onChange={(e) => setIsPlanningVisibleToCustomers(e.target.checked)}
                disabled={isSubmitting}
                inputProps={{ "aria-label": "Implementation Plan visible to customers" }}
              />
            }
            label={
              <Typography variant="body2" color="text.secondary">
                Implementation Plan visible to customers
              </Typography>
            }
          />

          <Typography variant="subtitle2" sx={{ mt: 1 }}>
            Schedule
          </Typography>

          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
              <DateTimePicker
                label="Planned start"
                value={parseDateTimeLocal(plannedStartDate)}
                maxDateTime={parseDateTimeLocal(plannedEndDate) ?? undefined}
                onChange={(date) =>
                  setPlannedStartDate(
                    date instanceof Date && !Number.isNaN(date.getTime())
                      ? formatDateTimeLocal(date)
                      : "",
                  )
                }
                disabled={isSubmitting}
                sx={{ flex: "1 1 240px" }}
                slotProps={{
                  textField: {
                    size: "small",
                    fullWidth: true,
                    helperText: plannedStartIsPast
                      ? "This date is in the past."
                      : undefined,
                  },
                  field: { clearable: true },
                }}
              />
              <DateTimePicker
                label="Planned end"
                value={parseDateTimeLocal(plannedEndDate)}
                minDateTime={parseDateTimeLocal(plannedStartDate) ?? undefined}
                onChange={(date) =>
                  setPlannedEndDate(
                    date instanceof Date && !Number.isNaN(date.getTime())
                      ? formatDateTimeLocal(date)
                      : "",
                  )
                }
                disabled={isSubmitting}
                sx={{ flex: "1 1 240px" }}
                slotProps={{
                  textField: {
                    size: "small",
                    fullWidth: true,
                    helperText: plannedEndIsPast
                      ? "This date is in the past."
                      : undefined,
                  },
                  field: { clearable: true },
                }}
              />
            </Box>
          </LocalizationProvider>

          <Typography variant="subtitle2" sx={{ mt: 1 }}>
            More options
          </Typography>

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeGroup>
                id="cr-group"
                label="Assignment group"
                placeholder="Search groups…"
                value={groupId}
                onChange={setGroupId}
                disabled={isSubmitting}
                useSearch={useSearchGroups}
                getId={(g) => g.id}
                getLabel={(g) => g.name}
              />
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeUser>
                id="cr-assigned-engineer"
                label="Assigned to"
                placeholder="Search people…"
                value={assignedEngineerId}
                onChange={setAssignedEngineerId}
                disabled={isSubmitting}
                useSearch={useSearchInternalUsersByName}
                // useSearchUsersByName filters out any user without an id,
                // so every option here is guaranteed to have one.
                getId={(u) => u.id!}
                getLabel={userLabel}
                knownLabel={cloneState?.assignedEngineerLabel}
              />
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeUser>
                id="cr-requested-by"
                label="Requested by"
                placeholder="Search people…"
                value={requestedById}
                onChange={setRequestedById}
                disabled={isSubmitting}
                useSearch={useSearchInternalUsersByName}
                // useSearchUsersByName filters out any user without an id,
                // so every option here is guaranteed to have one.
                getId={(u) => u.id!}
                getLabel={userLabel}
                knownLabel={meLabel}
                helperText="Defaults to you — clear it if this wasn't your request."
              />
            </Box>
          </Box>
        </Box>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={handleCancel}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSubmit}
            disabled={!canSubmit}
            loading={isSubmitting}
          >
            Create change request
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
