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
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchItServices } from "@api/useSearchItServices";
import { useSearchServiceOfferings } from "@api/useSearchServiceOfferings";
import { useSearchConfigurationItems } from "@api/useSearchConfigurationItems";
import { useSearchUsersByName } from "@api/useSearchUsersByName";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import AsyncEntityMultiSelect from "@components/AsyncEntityMultiSelect";
import {
  CATEGORY_OPTIONS,
  CONTACT_TYPE_OPTIONS,
  IMPACT_OPTIONS,
  SUBCATEGORY_OPTIONS_BY_CATEGORY,
  URGENCY_OPTIONS,
  configurationItemLabel,
  itServiceLabel,
  userLabel,
} from "@features/csm-operations/utils/incidentFormOptions";
import { computeIncidentPriority } from "@features/csm-operations/utils/incidentPriorityMatrix";
import { INCIDENT_STATES, incidentStateLabel } from "@features/csm-operations/utils/incidents";
import type {
  BeIncidentCategory,
  BeIncidentContactType,
  BeIncidentDetail,
  BeIncidentImpact,
  BeIncidentState,
  BeIncidentSubcategory,
  BeIncidentUrgency,
  BeUpdateIncidentPayload,
  BeConfigurationItem,
  BeGroup,
  BeItService,
  BeServiceOffering,
  BeUser,
} from "@api/backend/types";

const UNSET = "" as const;
const SELECT_PLACEHOLDER = "-- Select --";

interface EditIncidentDialogProps {
  incident: BeIncidentDetail;
  /** True while the PATCH is in flight; disables the actions. */
  isSaving: boolean;
  onClose: () => void;
  /** Submit only the changed fields (`PATCH /incidents/{id}`). */
  onSave: (patch: BeUpdateIncidentPayload) => void;
}

/** Local editable copy of the fields this dialog covers, derived once from `incident`. */
interface EditState {
  subject: string;
  category: BeIncidentCategory | "";
  subcategory: BeIncidentSubcategory | "";
  contactType: BeIncidentContactType | "";
  impact: BeIncidentImpact | "";
  urgency: BeIncidentUrgency | "";
  state: BeIncidentState | "";
  /** Write-only (see `BeUpdateIncidentPayload`) — `IncidentDetail` never
   * echoes these back, so they always start blank, unlike every other field
   * here. Required by ServiceNow to move `state` to `RESOLVED`/`CLOSED`. */
  resolutionCode: string;
  resolutionNotes: string;
  serviceId: string;
  serviceOfferingId: string;
  configurationItemId: string;
  assignmentGroupId: string;
  assignedEngineerId: string;
  watchList: string[];
  workNotes: string;
  additionalComments: string;
  parentId: string;
  changeRequestId: string;
  problemId: string;
  causedById: string;
}

function toEditState(incident: BeIncidentDetail): EditState {
  return {
    subject: incident.subject ?? "",
    category: incident.category ?? UNSET,
    subcategory: incident.subcategory ?? UNSET,
    contactType: incident.contactType ?? UNSET,
    impact: incident.impact ?? UNSET,
    urgency: incident.urgency ?? UNSET,
    state: incident.state ?? UNSET,
    resolutionCode: "",
    resolutionNotes: "",
    serviceId: incident.service?.id ?? "",
    serviceOfferingId: incident.serviceOffering?.id ?? "",
    configurationItemId: incident.configurationItem?.id ?? "",
    assignmentGroupId: incident.assignmentGroup?.id ?? "",
    assignedEngineerId: incident.assignedTo?.id ?? "",
    watchList: (incident.watchList ?? []).map((w) => w.id),
    workNotes: incident.workNotes ?? "",
    additionalComments: incident.additionalComments ?? "",
    parentId: incident.parent?.id ?? "",
    changeRequestId: incident.changeRequest?.id ?? "",
    problemId: incident.problem?.id ?? "",
    causedById: incident.causedBy?.id ?? "",
  };
}

function sameIds(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const sortedA = [...a].sort();
  const sortedB = [...b].sort();
  return sortedA.every((id, i) => id === sortedB[i]);
}

/**
 * Diff `next` against `initial` so only changed fields are sent. Reference
 * and note fields send an explicit `null` when cleared (distinct from
 * omitting the field, which the BE treats as "leave unchanged" — see
 * {@link BeUpdateIncidentPayload}); the fixed-enum fields (category, impact,
 * etc.) never clear to null, only to a different value.
 *
 * `resolutionCode`/`resolutionNotes` don't fit that diff pattern — they're
 * write-only (no `initial` value exists to diff against), so they're sent
 * whenever filled in, regardless of whether anything else changed.
 */
function buildPatch(initial: EditState, next: EditState): BeUpdateIncidentPayload {
  const patch: BeUpdateIncidentPayload = {};
  if (next.subject.trim() !== initial.subject && next.subject.trim()) patch.subject = next.subject.trim();
  if (next.category !== initial.category && next.category) patch.category = next.category;
  if (next.subcategory !== initial.subcategory && next.subcategory) patch.subcategory = next.subcategory;
  if (next.contactType !== initial.contactType && next.contactType) patch.contactType = next.contactType;
  if (next.impact !== initial.impact && next.impact) patch.impact = next.impact;
  if (next.urgency !== initial.urgency && next.urgency) patch.urgency = next.urgency;
  if (next.state !== initial.state && next.state) patch.state = next.state;
  if (next.resolutionCode.trim()) patch.resolutionCode = next.resolutionCode.trim();
  if (next.resolutionNotes.trim()) patch.resolutionNotes = next.resolutionNotes.trim();
  if (next.serviceId !== initial.serviceId) patch.serviceId = next.serviceId || null;
  if (next.serviceOfferingId !== initial.serviceOfferingId) patch.serviceOfferingId = next.serviceOfferingId || null;
  if (next.configurationItemId !== initial.configurationItemId)
    patch.configurationItemId = next.configurationItemId || null;
  if (next.assignmentGroupId !== initial.assignmentGroupId) patch.assignmentGroupId = next.assignmentGroupId || null;
  if (next.assignedEngineerId !== initial.assignedEngineerId)
    patch.assignedEngineerId = next.assignedEngineerId || null;
  if (!sameIds(next.watchList, initial.watchList)) patch.watchList = next.watchList;
  if (next.workNotes.trim() !== initial.workNotes) patch.workNotes = next.workNotes.trim() || null;
  if (next.additionalComments.trim() !== initial.additionalComments)
    patch.additionalComments = next.additionalComments.trim() || null;
  if (next.parentId.trim() !== initial.parentId) patch.parentId = next.parentId.trim() || null;
  if (next.changeRequestId.trim() !== initial.changeRequestId)
    patch.changeRequestId = next.changeRequestId.trim() || null;
  if (next.problemId.trim() !== initial.problemId) patch.problemId = next.problemId.trim() || null;
  if (next.causedById.trim() !== initial.causedById) patch.causedById = next.causedById.trim() || null;
  return patch;
}

/**
 * Edit dialog for an incident's classification, state, assignment, watch
 * list, notes, and linked-record ids — mirrors `CreateIncidentPage.tsx`'s
 * fields (same option lists, same async pickers) plus a State select, which
 * only makes sense once an incident already exists. Priority is never sent
 * directly here either — ServiceNow computes it from impact × urgency, same
 * rule as create.
 */
export default function EditIncidentDialog({
  incident,
  isSaving,
  onClose,
  onSave,
}: EditIncidentDialogProps): JSX.Element {
  const initial = useMemo(() => toEditState(incident), [incident]);
  const [state, setState] = useState<EditState>(initial);

  const knownWatchListLabels = useMemo(
    () => Object.fromEntries((incident.watchList ?? []).map((w) => [w.id, w.name])),
    [incident.watchList],
  );

  const subcategoryOptions = state.category ? SUBCATEGORY_OPTIONS_BY_CATEGORY[state.category] : [];
  const priority = computeIncidentPriority(state.impact || "", state.urgency || "");

  const patch = useMemo(() => buildPatch(initial, state), [initial, state]);
  const hasChanges = Object.keys(patch).length > 0;

  const set = <K extends keyof EditState>(key: K, value: EditState[K]): void =>
    setState((prev) => ({ ...prev, [key]: value }));

  const handleCategoryChange = (next: string): void => {
    set("category", next as BeIncidentCategory | "");
    // A subcategory only makes sense under its own category — drop it rather
    // than leave a stale pairing, same rule as CreateIncidentPage.
    set("subcategory", UNSET);
  };

  const handleServiceChange = (next: string): void => {
    set("serviceId", next);
    set("serviceOfferingId", "");
  };

  const renderSelect = (
    label: string,
    value: string,
    onChange: (v: string) => void,
    options: Array<{ value: string; label: string }>,
    opts?: { disabled?: boolean; helperText?: string },
  ): JSX.Element => (
    <FormControl fullWidth size="small" disabled={isSaving || opts?.disabled}>
      <InputLabel id={`${label}-label`} shrink>
        {label}
      </InputLabel>
      <Select
        labelId={`${label}-label`}
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

  return (
    <Dialog open onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>Edit incident</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <TextField
            label="Subject"
            value={state.subject}
            onChange={(e) => set("subject", e.target.value)}
            fullWidth
            disabled={isSaving}
          />

          <Divider />
          <Typography variant="subtitle2">Classification</Typography>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect("Category", state.category, handleCategoryChange, CATEGORY_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect(
                "Subcategory",
                state.subcategory,
                (v) => set("subcategory", v as BeIncidentSubcategory | ""),
                subcategoryOptions,
                { disabled: !state.category, helperText: state.category ? undefined : "Pick a category first." },
              )}
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect("Contact type", state.contactType, (v) => set("contactType", v as BeIncidentContactType | ""), CONTACT_TYPE_OPTIONS)}
            </Box>
          </Box>

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect("Impact", state.impact, (v) => set("impact", v as BeIncidentImpact | ""), IMPACT_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect("Urgency", state.urgency, (v) => set("urgency", v as BeIncidentUrgency | ""), URGENCY_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect(
                "State",
                state.state,
                (v) => set("state", v as BeIncidentState | ""),
                INCIDENT_STATES.map((s) => ({ value: s, label: incidentStateLabel(s) })),
              )}
            </Box>
          </Box>

          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="caption" color="text.secondary">
              Priority
            </Typography>
            {priority ? (
              <Chip
                size="small"
                label={`${priority.label} (${priority.code})`}
                sx={{ bgcolor: priority.bg, color: priority.fg, fontWeight: 600 }}
              />
            ) : (
              <Chip size="small" label="Set impact & urgency" variant="outlined" disabled />
            )}
          </Box>

          {(state.state === "RESOLVED" || state.state === "CLOSED") && (
            <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <Typography variant="caption" color="text.secondary">
                ServiceNow requires a resolution to move this incident to {incidentStateLabel(state.state)}.
              </Typography>
              <TextField
                label="Resolution code"
                value={state.resolutionCode}
                onChange={(e) => set("resolutionCode", e.target.value)}
                disabled={isSaving}
                fullWidth
                size="small"
              />
              <TextField
                label="Resolution notes"
                value={state.resolutionNotes}
                onChange={(e) => set("resolutionNotes", e.target.value)}
                disabled={isSaving}
                multiline
                minRows={2}
                fullWidth
              />
            </Box>
          )}

          <Divider />
          <Typography variant="subtitle2">Service &amp; assignment</Typography>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeItService>
                id="edit-incident-service"
                label="Service"
                placeholder="Search services…"
                value={state.serviceId}
                onChange={handleServiceChange}
                disabled={isSaving}
                useSearch={useSearchItServices}
                getId={(s) => s.id}
                getLabel={itServiceLabel}
                knownLabel={incident.service?.name}
              />
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeServiceOffering>
                id="edit-incident-service-offering"
                label="Service offering"
                placeholder="Search service offerings…"
                value={state.serviceOfferingId}
                onChange={(v) => set("serviceOfferingId", v)}
                disabled={isSaving}
                useSearch={useSearchServiceOfferings}
                searchExtra={state.serviceId || undefined}
                getId={(o) => o.id}
                getLabel={(o) => o.name}
                knownLabel={incident.serviceOffering?.name}
                helperText={state.serviceId ? undefined : "Narrows to a Service once one is picked."}
              />
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeConfigurationItem>
                id="edit-incident-configuration-item"
                label="Configuration item"
                placeholder="Search configuration items…"
                value={state.configurationItemId}
                onChange={(v) => set("configurationItemId", v)}
                disabled={isSaving}
                useSearch={useSearchConfigurationItems}
                getId={(ci) => ci.id}
                getLabel={configurationItemLabel}
                knownLabel={incident.configurationItem?.name}
              />
            </Box>
          </Box>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeGroup>
                id="edit-incident-assignment-group"
                label="Assignment group"
                placeholder="Search groups…"
                value={state.assignmentGroupId}
                onChange={(v) => set("assignmentGroupId", v)}
                disabled={isSaving}
                useSearch={useSearchGroups}
                getId={(g) => g.id}
                getLabel={(g) => g.name}
                knownLabel={incident.assignmentGroup?.name}
              />
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeUser>
                id="edit-incident-assigned-engineer"
                label="Assigned to"
                placeholder="Search people…"
                value={state.assignedEngineerId}
                onChange={(v) => set("assignedEngineerId", v)}
                disabled={isSaving}
                useSearch={useSearchUsersByName}
                getId={(u) => u.id!}
                getLabel={userLabel}
                knownLabel={incident.assignedTo?.name}
              />
            </Box>
          </Box>

          <AsyncEntityMultiSelect<BeUser>
            id="edit-incident-watch-list"
            label="Watch list"
            placeholder="Search people…"
            values={state.watchList}
            onChange={(v) => set("watchList", v)}
            disabled={isSaving}
            useSearch={useSearchUsersByName}
            getId={(u) => u.id!}
            getLabel={userLabel}
            knownLabels={knownWatchListLabels}
            helperText="Notified on updates to this incident."
          />

          <Divider />
          <Typography variant="subtitle2">Notes</Typography>
          <TextField
            label="Additional comments"
            multiline
            minRows={2}
            value={state.additionalComments}
            onChange={(e) => set("additionalComments", e.target.value)}
            disabled={isSaving}
            fullWidth
            helperText="Visible to the customer."
          />
          <TextField
            label="Internal work note"
            multiline
            minRows={2}
            value={state.workNotes}
            onChange={(e) => set("workNotes", e.target.value)}
            disabled={isSaving}
            fullWidth
            helperText="Internal only — never shown to the customer."
          />

          <Divider />
          <Typography variant="caption" color="text.secondary">
            Advanced linking (portal UUIDs — no lookup available for these yet)
          </Typography>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <TextField
              label="Parent incident ID"
              size="small"
              value={state.parentId}
              onChange={(e) => set("parentId", e.target.value)}
              disabled={isSaving}
              sx={{ flex: "1 1 220px" }}
            />
            <TextField
              label="Change request ID"
              size="small"
              value={state.changeRequestId}
              onChange={(e) => set("changeRequestId", e.target.value)}
              disabled={isSaving}
              sx={{ flex: "1 1 220px" }}
            />
            <TextField
              label="Problem ID"
              size="small"
              value={state.problemId}
              onChange={(e) => set("problemId", e.target.value)}
              disabled={isSaving}
              sx={{ flex: "1 1 220px" }}
            />
            <TextField
              label="Caused by ID"
              size="small"
              value={state.causedById}
              onChange={(e) => set("causedById", e.target.value)}
              disabled={isSaving}
              sx={{ flex: "1 1 220px" }}
            />
          </Box>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button variant="contained" onClick={() => onSave(patch)} disabled={isSaving || !hasChanges}>
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
