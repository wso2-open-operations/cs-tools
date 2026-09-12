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
  Accordion,
  AccordionDetails,
  AccordionSummary,
  AdapterDateFns,
  Alert,
  Box,
  Button,
  Card,
  DatePickers,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";

const { DateTimePicker, LocalizationProvider } = DatePickers;
import { ArrowLeft, ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { useRef, useState, type JSX } from "react";
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
import { useSearchUsersByName } from "@api/useSearchUsersByName";
import { useSearchServiceRequestsForSelect } from "@features/csm-operations/api/useSearchServiceRequestsForSelect";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import {
  changeRequestStateLabel,
  CLONE_SOURCE_GAP_MESSAGE,
  type CloneChangeRequestNavState,
} from "@features/csm-operations/utils/changeRequests";
import type { CreateChangeRequestFromCaseNavState } from "@features/csm-cases/types/csmCases";
import type {
  BeChangeRequestImpact,
  BeChangeRequestPriority,
  BeChangeRequestState,
  BeChangeRequestType,
  BeCaseSearchView,
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

/**
 * Display label for an originating-service-request option, as
 * "CS0001234 — subject". Number and subject are both optional on the search
 * view, so it degrades to whichever exists and finally to the id.
 */
function caseSearchLabel(c: BeCaseSearchView): string {
  return [c.number, c.subject].filter(Boolean).join(" — ") || c.id;
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

  // This form can be opened two ways, each carrying its own router state
  // (not query params) — read once: this form's state is what the user edits
  // from here on, so a later change to the *source* record must not reach
  // back in and overwrite what they've typed. The two shapes are mutually
  // exclusive; narrow on `caseId`, the field only the latter carries.
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
  const location = useLocation();
  const locationState = location.state as
    | CloneChangeRequestNavState
    | CreateChangeRequestFromCaseNavState
    | undefined;
  const cloneState =
    locationState && !("caseId" in locationState) ? locationState : undefined;
  const fromCaseState =
    locationState && "caseId" in locationState ? locationState : undefined;

  // Set when opened from a list/detail page's own "Create change request"
  // action with `state: { from: ... }` (same convention as the 4 case-type
  // create pages), so Back/Cancel return there instead of the hardcoded
  // change-requests tab, and the newly created change request's own Back
  // button (reading this same convention) returns there too.
  const backState = location.state as { from?: string } | undefined;
  const backTarget = backState?.from ?? OPERATIONS_CHANGE_REQUESTS_PATH;

  // Slice on seed as well as on change: a source record at or beyond the cap
  // would otherwise load untrimmed, show a negative characters-left count, and
  // submit over-length if the user never edits the field.
  const [subject, setSubject] = useState((cloneState?.subject ?? "").slice(0, SUBJECT_MAX));
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
  const [type, setType] = useState<string>(cloneState?.type ?? "normal");
  const [impact, setImpact] = useState<string>(cloneState?.impact ?? "low");
  const [priority, setPriority] = useState<string>(UNSET);
  // Always "new" regardless of the source record's own state/schedule/
  // approval — cloning must never carry an approval or a stale window
  // across into the new change request.
  const [state, setState] = useState<string>("new");
  const [plannedStartDate, setPlannedStartDate] = useState("");
  const [plannedEndDate, setPlannedEndDate] = useState("");
  const [description, setDescription] = useState(cloneState?.description ?? "");
  const [justification, setJustification] = useState(cloneState?.justification ?? "");
  const [implementationPlan, setImplementationPlan] = useState("");
  const [riskImpactAnalysis, setRiskImpactAnalysis] = useState("");
  const [backoutPlan, setBackoutPlan] = useState("");
  const [testPlan, setTestPlan] = useState(cloneState?.testPlan ?? "");
  const [groupId, setGroupId] = useState("");
  const [assignedEngineerId, setAssignedEngineerId] = useState(
    cloneState?.assignedEngineerId ?? "",
  );
  const [requestedById, setRequestedById] = useState("");
  // The service request this change request was raised from, when picked.
  // Not part of BeCreateChangeRequestPayload — see handleSubmit's comment.
  // Pre-selected when opened from that service request's own "Create change
  // request…" action; stays fully editable from there — the field remains a
  // normal AsyncEntitySelect, not a locked/read-only control, so a wrong
  // pre-fill (or a genuine need to link a different service request instead)
  // can still be corrected without leaving the form.
  const [caseId, setCaseId] = useState(fromCaseState?.caseId ?? "");
  // Display label for the pre-filled `caseId` above until a fresh search for
  // the same id resolves one from the backend (see AsyncEntitySelect's
  // `knownLabel`).
  const fromCaseKnownLabel = fromCaseState
    ? caseSearchLabel({
        id: fromCaseState.caseId,
        number: fromCaseState.caseNumber,
        subject: fromCaseState.caseSubject,
      })
    : undefined;

  // Defaults "Requested by" to the signed-in user, matching the legacy
  // ServiceNow form's own behaviour (usePostChangeRequest.ts/BE doesn't do
  // this itself — see BeCreateChangeRequestPayload's doc comment). Fires
  // once, when the current user's id first loads; a ref (not the field's own
  // emptiness) gates it so manually clearing the field afterward sticks.
  // Adjusted during render (React's recommended pattern for this) rather
  // than in an effect, which would call setState synchronously post-commit.
  const { data: me } = useGetUsersMe();
  const meLabel = me ? userLabel(me) : undefined;
  const autoFilledRequester = useRef(false);
  if (me?.id && !autoFilledRequester.current) {
    autoFilledRequester.current = true;
    setRequestedById(me.id);
  }

  const isSubmitting = postChangeRequest.isPending || patchChangeRequest.isPending;
  const canSubmit = subject.trim().length > 0 && !isSubmitting;
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
    if (groupId.trim()) payload.groupId = groupId.trim();
    if (assignedEngineerId.trim()) payload.assignedEngineerId = assignedEngineerId.trim();
    if (requestedById.trim()) payload.requestedById = requestedById.trim();

    postChangeRequest.mutate(payload, {
      onSuccess: (created) => {
        const createdId = created.changeRequest.id;
        // POST /change-requests can't carry the originating-service-request
        // link (it isn't an accepted create field), so it's set with a
        // follow-up PATCH once the change request exists. A failed PATCH
        // still leaves a valid, created change request — navigate there
        // regardless, but surface the link failure rather than hiding it.
        if (!caseId) {
          navigate(`/operations/change-requests/${createdId}`, {
            state: { from: backTarget },
          });
          return;
        }
        patchChangeRequest.mutate(
          { id: createdId, patch: { caseId } },
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
        onClick={() => navigate(backTarget)}
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
          carried through automatically as the Originating service request below (see
          "More options").
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

          {/* Everything below is optional and used less often at creation
              time — collapsed by default so the form isn't dominated by
              fields most requests won't need up front. */}
          <Accordion
            disableGutters
            // Auto-expanded when cloning and an engineer carried over, or
            // when opened from a service request with its id pre-filled
            // below, so the prefilled value isn't hidden behind a collapsed
            // section.
            defaultExpanded={!!cloneState?.assignedEngineerId || !!fromCaseState}
            sx={{ "&:before": { display: "none" }, mt: 1 }}
          >
            <AccordionSummary expandIcon={<ChevronDown size={16} />}>
              <Typography variant="body2" color="text.secondary">
                More options (optional)
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
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
                    useSearch={useSearchUsersByName}
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
                    useSearch={useSearchUsersByName}
                    // useSearchUsersByName filters out any user without an id,
                    // so every option here is guaranteed to have one.
                    getId={(u) => u.id!}
                    getLabel={userLabel}
                    knownLabel={meLabel}
                    helperText="Defaults to you — clear it if this wasn't your request."
                  />
                </Box>
                <Box sx={{ flex: "1 1 220px" }}>
                  <AsyncEntitySelect<BeCaseSearchView>
                    id="cr-originating-service-request"
                    label="Originating service request"
                    placeholder="Search service requests…"
                    value={caseId}
                    onChange={setCaseId}
                    disabled={isSubmitting}
                    // Opened from a service request's own "Create change
                    // request…" action: its project is threaded through as
                    // `searchExtra` so the search prefers service requests
                    // from the same project first (see
                    // useSearchServiceRequestsForSelect's doc comment for how
                    // that stays additive, not a hard filter). Opened any
                    // other way (this page's own "New change request" entry
                    // point, or a Clone) there's no case context at all, so
                    // `searchExtra` is undefined and the search stays exactly
                    // the unscoped, system-wide search it's always been.
                    useSearch={useSearchServiceRequestsForSelect}
                    searchExtra={fromCaseState?.projectId}
                    getId={(c) => c.id}
                    getLabel={caseSearchLabel}
                    knownLabel={fromCaseKnownLabel}
                    helperText={
                      fromCaseState
                        ? "Pre-filled from the service request you opened this from — change it if that's not right."
                        : "Links this change request back to the service request it was raised from."
                    }
                  />
                </Box>
              </Box>
            </AccordionDetails>
          </Accordion>
        </Box>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={() => navigate(backTarget)}>
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
