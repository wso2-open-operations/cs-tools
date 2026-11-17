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
  Alert,
  Box,
  Button,
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
import { useMemo, useState, type JSX, type ReactNode } from "react";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchItServices } from "@api/useSearchItServices";
import { useSearchServiceOfferings } from "@api/useSearchServiceOfferings";
import { useSearchConfigurationItems } from "@api/useSearchConfigurationItems";
import { useSearchInternalUsersByName } from "@api/useSearchUsersByName";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import AsyncEntityMultiSelect from "@components/AsyncEntityMultiSelect";
import { useSearchIncidentsExcludingIds } from "@features/csm-operations/api/useSearchIncidentsForSelect";
import { useSearchChangeRequestsForSelect } from "@features/csm-operations/api/useSearchChangeRequestsForSelect";
import { useSearchProblemsForSelect } from "@features/csm-operations/api/useSearchProblemsForSelect";
import {
  CATEGORY_OPTIONS,
  CHANNEL_OPTIONS,
  IMPACT_OPTIONS,
  SUBCATEGORY_OPTIONS_BY_CATEGORY,
  URGENCY_OPTIONS,
  configurationItemLabel,
  itServiceLabel,
  userLabel,
} from "@features/csm-operations/utils/incidentFormOptions";
import {
  bulkIncidentTargetStates,
  incidentStateLabel,
  isIncidentTerminal,
} from "@features/csm-operations/utils/incidents";
import { INCIDENT_RESOLUTION_CODES, INCIDENT_RESOLUTION_CODE_LABELS } from "@features/csm-operations/utils/incidentResolution";
import {
  BULK_WATCH_LIST_MODES,
  type BulkIncidentEdit,
  type BulkWatchListMode,
} from "@features/csm-operations/utils/bulkIncidentEdit";
import type {
  BeChangeRequestSearchView,
  BeConfigurationItem,
  BeGroup,
  BeIncident,
  BeIncidentCategory,
  BeIncidentContactType,
  BeIncidentImpact,
  BeIncidentResolutionCode,
  BeIncidentState,
  BeIncidentSubcategory,
  BeIncidentUrgency,
  BeItService,
  BeProblemSearchView,
  BeServiceOffering,
  BeUpdateIncidentPayload,
  BeUser,
} from "@api/backend/types";

interface BulkEditIncidentsDialogProps {
  /** The selected incidents (none closed or cancelled: the list does not let those be selected). */
  incidents: BeIncident[];
  isApplying: boolean;
  onClose: () => void;
  /** The one change to apply to every selected incident: only the fields set here. */
  onApply: (edit: BulkIncidentEdit) => void;
}

const NO_CHANGE = "";
/** The short description's limit, as on ServiceNow's form. */
const SUBJECT_MAX = 500;
/** `incident.environment` is VARCHAR(40): a longer value is refused for every incident. */
const ENVIRONMENT_MAX = 40;

function incidentLinkLabel(i: BeIncident): string {
  return [i.number, i.subject].filter(Boolean).join(" — ") || i.id || "";
}

function changeRequestLinkLabel(cr: BeChangeRequestSearchView): string {
  return [cr.number, cr.subject].filter(Boolean).join(" — ") || cr.id;
}

function problemLinkLabel(p: BeProblemSearchView): string {
  return [p.number, p.subject].filter(Boolean).join(" — ") || p.id;
}

/**
 * Bulk edit for the incidents selected in the list: the incident form's
 * fields, every one on "No change" until it is set, and only what is set is
 * sent, so each incident keeps its own value for the rest (ServiceNow's list
 * "Update selected"). It offers every field `PATCH /incidents/{id}` accepts:
 *
 * - State offers only what every selected incident can move to (the Edit
 *   dialog's own table); Resolved and Closed need a resolution code and notes,
 *   the same for all of them. Resolution code and notes can also be set alone.
 * - Subcategory follows the category being set, or the category every selected
 *   incident already shares. A new category needs a subcategory too: the
 *   PATCH can't clear one, so the old category's subcategory would stay on the
 *   ServiceNow paths (the single Edit dialog's rule).
 * - Service offering needs the service being set, and narrows to it: an
 *   offering is never written onto incidents of another service.
 * - Incident report is off while resolving: resolving generates the report,
 *   which would overwrite what was typed seconds later.
 * - The watch list can have people added or removed (from each incident's own
 *   list) or be replaced.
 * - Parent incident never offers a selected incident. It sets the parent
 *   incident (`parentIncidentId`), as the single Edit dialog does, not the
 *   generic `parentId`.
 *
 * Priority is not offered: it follows from impact and urgency, as on a single
 * incident. The backend stays the authority: what it refuses for one incident
 * (e.g. Closed by somebody who is not the assignee) is reported for that one
 * and the rest go through.
 */
export default function BulkEditIncidentsDialog({ incidents, isApplying, onClose, onApply }: BulkEditIncidentsDialogProps): JSX.Element {
  const [subject, setSubject] = useState("");
  const [state, setState] = useState<BeIncidentState | "">(NO_CHANGE);
  const [channel, setChannel] = useState<BeIncidentContactType | "">(NO_CHANGE);
  const [impact, setImpact] = useState<BeIncidentImpact | "">(NO_CHANGE);
  const [urgency, setUrgency] = useState<BeIncidentUrgency | "">(NO_CHANGE);
  const [category, setCategory] = useState<BeIncidentCategory | "">(NO_CHANGE);
  const [subcategory, setSubcategory] = useState<BeIncidentSubcategory | "">(NO_CHANGE);
  const [environment, setEnvironment] = useState("");
  const [serviceId, setServiceId] = useState("");
  const [serviceOfferingId, setServiceOfferingId] = useState("");
  const [configurationItemId, setConfigurationItemId] = useState("");
  const [assignmentGroupId, setAssignmentGroupId] = useState("");
  const [assignedEngineerId, setAssignedEngineerId] = useState("");
  const [watchListMode, setWatchListMode] = useState<BulkWatchListMode>("add");
  const [watchers, setWatchers] = useState<string[]>([]);
  const [workNotes, setWorkNotes] = useState("");
  const [additionalComments, setAdditionalComments] = useState("");
  const [parentIncidentId, setParentIncidentId] = useState("");
  const [changeRequestId, setChangeRequestId] = useState("");
  const [problemId, setProblemId] = useState("");
  const [causedById, setCausedById] = useState("");
  const [resolutionCode, setResolutionCode] = useState<BeIncidentResolutionCode | "">(NO_CHANGE);
  const [resolutionNotes, setResolutionNotes] = useState("");
  const [incidentReport, setIncidentReport] = useState("");

  // One closed or cancelled since it was picked is skipped by the run, so it does not narrow the states.
  const editable = useMemo(() => incidents.filter((i) => !isIncidentTerminal(i.state)), [incidents]);
  const finishedCount = incidents.length - editable.length;
  const targetStates = useMemo(() => bulkIncidentTargetStates(editable.map((i) => i.state)), [editable]);
  const needsResolution = state === "RESOLVED" || state === "CLOSED";
  const resolutionMissing = needsResolution && (!resolutionCode || !resolutionNotes.trim());
  // Resolving generates the incident report (entity-service's IncidentReportService), overwriting a typed one.
  const reportGenerated = state === "RESOLVED";

  // The category the subcategory belongs to: the one being set, else the one every selected incident shares.
  const sharedCategory = useMemo(() => {
    const categories = new Set(editable.map((i) => i.category ?? ""));
    const only = categories.size === 1 ? [...categories][0] : "";
    return (only || "") as BeIncidentCategory | "";
  }, [editable]);
  const subcategoryCategory = category || sharedCategory;
  const subcategoryOptions = subcategoryCategory ? SUBCATEGORY_OPTIONS_BY_CATEGORY[subcategoryCategory] ?? [] : [];
  // A PATCH can't clear a subcategory, so a new category goes with one of its own (when it has any).
  const subcategoryMissing = !!category && subcategoryOptions.length > 0 && !subcategory;
  const excludedParents = useMemo(() => incidents.map((i) => i.id).filter(Boolean).join(","), [incidents]);

  const edit = useMemo<BulkIncidentEdit>(() => {
    const patch: BeUpdateIncidentPayload = {};
    if (subject.trim()) patch.subject = subject.trim();
    if (state) patch.state = state;
    if (channel) patch.contactType = channel;
    if (impact) patch.impact = impact;
    if (urgency) patch.urgency = urgency;
    if (category) patch.category = category;
    if (subcategory) patch.subcategory = subcategory;
    if (environment.trim()) patch.environment = environment.trim();
    if (serviceId) patch.serviceId = serviceId;
    if (serviceOfferingId) patch.serviceOfferingId = serviceOfferingId;
    if (configurationItemId) patch.configurationItemId = configurationItemId;
    if (assignmentGroupId) patch.assignmentGroupId = assignmentGroupId;
    if (assignedEngineerId) patch.assignedEngineerId = assignedEngineerId;
    if (workNotes.trim()) patch.workNotes = workNotes.trim();
    if (additionalComments.trim()) patch.additionalComments = additionalComments.trim();
    if (parentIncidentId) patch.parentIncidentId = parentIncidentId;
    if (changeRequestId) patch.changeRequestId = changeRequestId;
    if (problemId) patch.problemId = problemId;
    if (causedById) patch.causedById = causedById;
    if (resolutionCode) patch.resolutionCode = resolutionCode;
    if (resolutionNotes.trim()) patch.resolutionNotes = resolutionNotes.trim();
    if (incidentReport.trim() && !reportGenerated) patch.incidentReport = incidentReport.trim();
    // "Replace with nobody" would clear every watch list: only a real list of people is a change here.
    const watchList = watchers.length > 0 ? { mode: watchListMode, userIds: watchers } : undefined;
    return watchList ? { patch, watchList } : { patch };
  }, [
    subject, state, channel, impact, urgency, category, subcategory, environment, serviceId, serviceOfferingId,
    configurationItemId, assignmentGroupId, assignedEngineerId, workNotes, additionalComments, parentIncidentId,
    changeRequestId, problemId, causedById, resolutionCode, resolutionNotes, incidentReport, reportGenerated, watchers,
    watchListMode,
  ]);

  const hasChanges = Object.keys(edit.patch).length > 0 || !!edit.watchList;
  const canApply = hasChanges && !resolutionMissing && !subcategoryMissing && !isApplying;
  const count = incidents.length;
  const noun = `incident${count === 1 ? "" : "s"}`;

  const select = <V extends string>(
    id: string,
    label: string,
    value: V | "",
    onChange: (v: V | "") => void,
    options: ReadonlyArray<{ value: V; label: string }>,
    opts: { helper?: string; disabled?: boolean; required?: boolean } = {},
  ): JSX.Element => (
    <FormControl fullWidth size="small" disabled={isApplying || opts.disabled} required={opts.required}>
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

  const section = (title: string, children: ReactNode, first = false): JSX.Element => (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {!first && <Divider />}
      <Typography variant="subtitle2" component="h3">
        {title}
      </Typography>
      {children}
    </Box>
  );
  // Wide, with columns that never squeeze a field below a readable size.
  const twoColumns = { display: "grid", gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" }, gap: 2 } as const;
  const threeColumns = { display: "grid", gridTemplateColumns: { xs: "1fr", md: "repeat(3, minmax(0, 1fr))" }, gap: 2 } as const;

  return (
    <Dialog open onClose={isApplying ? undefined : onClose} maxWidth="lg" fullWidth>
      <DialogTitle>
        Edit {count} {noun}
      </DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
        <Typography variant="body2" color="text.secondary">
          Only the fields you change are applied; the rest keep each incident's own value.
        </Typography>
        {finishedCount > 0 && (
          <Alert severity="warning">
            {finishedCount} of the selected incidents {finishedCount === 1 ? "has" : "have"} been closed or cancelled
            since {finishedCount === 1 ? "it was" : "they were"} selected and will be skipped.
          </Alert>
        )}

        {section(
          "Incident",
          <>
            <TextField
              id="bulk-incident-subject"
              label="Short description"
              placeholder="No change"
              size="small"
              value={subject}
              onChange={(e) => setSubject(e.target.value.slice(0, SUBJECT_MAX))}
              disabled={isApplying}
              slotProps={{ inputLabel: { shrink: true } }}
              helperText={subject ? `Sets the same short description on every selected incident. ${SUBJECT_MAX - subject.length} characters left.` : undefined}
            />
            <Box sx={twoColumns}>
              {select(
                "bulk-incident-state",
                "State",
                state,
                setState,
                targetStates.map((s) => ({ value: s, label: incidentStateLabel(s) })),
                {
                  helper:
                    targetStates.length === 0
                      ? "The selected incidents have no state they can all move to."
                      : "Only the states every selected incident can move to.",
                },
              )}
              {select("bulk-incident-channel", "Channel", channel, setChannel, CHANNEL_OPTIONS)}
            </Box>
            <Box sx={threeColumns}>
              {select("bulk-incident-impact", "Impact", impact, setImpact, IMPACT_OPTIONS)}
              {select("bulk-incident-urgency", "Urgency", urgency, setUrgency, URGENCY_OPTIONS)}
              <TextField
                id="bulk-incident-environment"
                label="Environment"
                placeholder="No change"
                size="small"
                value={environment}
                onChange={(e) => setEnvironment(e.target.value.slice(0, ENVIRONMENT_MAX))}
                disabled={isApplying}
                slotProps={{ inputLabel: { shrink: true } }}
                helperText={environment ? `${ENVIRONMENT_MAX - environment.length} characters left.` : undefined}
              />
            </Box>
            <Box sx={twoColumns}>
              {select("bulk-incident-category", "Category", category, (v) => {
                setCategory(v);
                // A subcategory only makes sense under its own category.
                setSubcategory(NO_CHANGE);
              }, CATEGORY_OPTIONS)}
              {select("bulk-incident-subcategory", "Subcategory", subcategory, setSubcategory, subcategoryOptions, {
                disabled: !subcategoryCategory,
                required: subcategoryMissing,
                helper: subcategoryCategory
                  ? category && subcategoryOptions.length > 0
                    ? "Required when the category changes: an incident's old subcategory can't be cleared otherwise."
                    : undefined
                  : "Set a category first: the selected incidents have different categories.",
              })}
            </Box>
          </>,
          true,
        )}

        {section(
          "Service",
          <Box sx={threeColumns}>
            <AsyncEntitySelect<BeItService>
              id="bulk-incident-service"
              label="Service"
              placeholder="No change"
              shrinkLabel
              value={serviceId}
              onChange={(v) => {
                setServiceId(v);
                setServiceOfferingId("");
              }}
              disabled={isApplying}
              useSearch={useSearchItServices}
              getId={(s) => s.id}
              getLabel={itServiceLabel}
            />
            <AsyncEntitySelect<BeServiceOffering>
              id="bulk-incident-service-offering"
              label="Service offering"
              placeholder="No change"
              shrinkLabel
              value={serviceOfferingId}
              onChange={setServiceOfferingId}
              disabled={isApplying || !serviceId}
              useSearch={useSearchServiceOfferings}
              searchExtra={serviceId || undefined}
              getId={(o) => o.id}
              getLabel={(o) => o.name}
              helperText={
                serviceId ? undefined : "Set a Service first: an offering must belong to the incidents' service."
              }
            />
            <AsyncEntitySelect<BeConfigurationItem>
              id="bulk-incident-configuration-item"
              label="Configuration item"
              placeholder="No change"
              shrinkLabel
              value={configurationItemId}
              onChange={setConfigurationItemId}
              disabled={isApplying}
              useSearch={useSearchConfigurationItems}
              getId={(c) => c.id}
              getLabel={configurationItemLabel}
            />
          </Box>,
        )}

        {section(
          "Assignment",
          <Box sx={twoColumns}>
            <AsyncEntitySelect<BeGroup>
              id="bulk-incident-assignment-group"
              label="Assignment group"
              placeholder="No change"
              shrinkLabel
              value={assignmentGroupId}
              onChange={setAssignmentGroupId}
              disabled={isApplying}
              useSearch={useSearchGroups}
              getId={(g) => g.id}
              getLabel={(g) => g.name}
            />
            <AsyncEntitySelect<BeUser>
              id="bulk-incident-assigned-to"
              label="Assigned to"
              placeholder="No change"
              shrinkLabel
              value={assignedEngineerId}
              onChange={setAssignedEngineerId}
              disabled={isApplying}
              useSearch={useSearchInternalUsersByName}
              getId={(u) => u.id!}
              getLabel={userLabel}
            />
          </Box>,
        )}

        {section(
          "Watch list",
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "minmax(0, 1fr) minmax(0, 2fr)" }, gap: 2 }}>
            <FormControl fullWidth size="small" disabled={isApplying}>
              <InputLabel id="bulk-incident-watch-mode-label" shrink>
                Change
              </InputLabel>
              <Select
                labelId="bulk-incident-watch-mode-label"
                label="Change"
                value={watchListMode}
                onChange={(e) => setWatchListMode(e.target.value as BulkWatchListMode)}
              >
                {BULK_WATCH_LIST_MODES.map((m) => (
                  <MenuItem key={m.value} value={m.value}>
                    {m.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <AsyncEntityMultiSelect<BeUser>
              id="bulk-incident-watch-list"
              label="Watch list"
              placeholder="No change"
              shrinkLabel
              values={watchers}
              onChange={setWatchers}
              disabled={isApplying}
              useSearch={useSearchInternalUsersByName}
              getId={(u) => u.id!}
              getLabel={userLabel}
              helperText={
                watchListMode === "replace"
                  ? "Every selected incident's watch list becomes exactly these people."
                  : watchListMode === "add"
                    ? "Added to each selected incident's own watch list."
                    : "Taken off each selected incident's own watch list."
              }
            />
          </Box>,
        )}

        {section(
          "Notes",
          <>
            <TextField
              id="bulk-incident-work-notes"
              label="Work note (internal)"
              multiline
              minRows={2}
              size="small"
              value={workNotes}
              onChange={(e) => setWorkNotes(e.target.value)}
              disabled={isApplying}
              helperText="Added to every selected incident; never shown to the customer."
            />
            <TextField
              id="bulk-incident-additional-comments"
              label="Additional comment (customer-visible)"
              multiline
              minRows={2}
              size="small"
              value={additionalComments}
              onChange={(e) => setAdditionalComments(e.target.value)}
              disabled={isApplying}
              helperText="Added to every selected incident, and visible to the customer."
            />
          </>,
        )}

        {section(
          "Related records",
          <Box sx={twoColumns}>
            <AsyncEntitySelect<BeIncident>
              id="bulk-incident-parent"
              label="Parent incident"
              placeholder="No change"
              shrinkLabel
              value={parentIncidentId}
              onChange={setParentIncidentId}
              disabled={isApplying}
              useSearch={useSearchIncidentsExcludingIds}
              searchExtra={excludedParents}
              getId={(i) => i.id!}
              getLabel={incidentLinkLabel}
              helperText="The selected incidents themselves are not offered."
            />
            <AsyncEntitySelect<BeChangeRequestSearchView>
              id="bulk-incident-change-request"
              label="Change request"
              placeholder="No change"
              shrinkLabel
              value={changeRequestId}
              onChange={setChangeRequestId}
              disabled={isApplying}
              useSearch={useSearchChangeRequestsForSelect}
              getId={(cr) => cr.id}
              getLabel={changeRequestLinkLabel}
            />
            <AsyncEntitySelect<BeProblemSearchView>
              id="bulk-incident-problem"
              label="Problem"
              placeholder="No change"
              shrinkLabel
              value={problemId}
              onChange={setProblemId}
              disabled={isApplying}
              useSearch={useSearchProblemsForSelect}
              getId={(p) => p.id}
              getLabel={problemLinkLabel}
            />
            <AsyncEntitySelect<BeChangeRequestSearchView>
              id="bulk-incident-caused-by"
              label="Caused by change"
              placeholder="No change"
              shrinkLabel
              value={causedById}
              onChange={setCausedById}
              disabled={isApplying}
              useSearch={useSearchChangeRequestsForSelect}
              getId={(cr) => cr.id}
              getLabel={changeRequestLinkLabel}
            />
          </Box>,
        )}

        {section(
          "Resolution",
          <>
            {select(
              "bulk-incident-resolution-code",
              "Resolution code",
              resolutionCode,
              setResolutionCode,
              INCIDENT_RESOLUTION_CODES.map((c) => ({ value: c, label: INCIDENT_RESOLUTION_CODE_LABELS[c] })),
              { required: needsResolution },
            )}
            <TextField
              id="bulk-incident-resolution-notes"
              label="Resolution notes"
              required={needsResolution}
              multiline
              minRows={2}
              size="small"
              value={resolutionNotes}
              onChange={(e) => setResolutionNotes(e.target.value)}
              disabled={isApplying}
              helperText={
                needsResolution
                  ? `Required to move an incident to ${incidentStateLabel(state)}; the same for every selected incident.`
                  : "Sets the same resolution notes on every selected incident."
              }
            />
          </>,
        )}

        {section(
          "Incident report",
          <TextField
            id="bulk-incident-report"
            label="Incident report"
            placeholder="No change"
            multiline
            minRows={3}
            size="small"
            value={reportGenerated ? "" : incidentReport}
            onChange={(e) => setIncidentReport(e.target.value)}
            disabled={isApplying || reportGenerated}
            slotProps={{ inputLabel: { shrink: true } }}
            helperText={
              reportGenerated
                ? "Resolving generates each incident's report, so it can't be set in the same edit. Edit it once they are resolved."
                : "Replaces each selected incident's report."
            }
          />,
        )}

        {(resolutionMissing || subcategoryMissing) && (
          <Alert severity="info" role="status">
            {resolutionMissing
              ? `${incidentStateLabel(state)} needs a resolution code and notes.`
              : "A new category needs a subcategory."}
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
