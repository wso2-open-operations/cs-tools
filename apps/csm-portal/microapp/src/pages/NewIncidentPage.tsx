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

import { useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Button,
  Chip,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { incidents } from "@src/services/incidents";
import { changeRequestLookups, type EntityOption } from "@src/services/changeRequestLookups";
import { users } from "@src/services/users";
import type {
  IncidentCategory,
  IncidentContactType,
  IncidentCreatePayloadDto,
  IncidentImpact,
  IncidentSubcategory,
  IncidentUrgency,
} from "@src/types";
import { Logger } from "@utils/logger";
import { incidentPriorityColor, incidentPriorityLabel } from "@components/operations/incidentConfig";
import {
  CATEGORY_OPTIONS,
  CONTACT_TYPE_OPTIONS,
  computeIncidentPriority,
  IMPACT_OPTIONS,
  SUBCATEGORY_OPTIONS_BY_CATEGORY,
  URGENCY_OPTIONS,
} from "@components/operations/incidentFormOptions";
import { AsyncEntitySelect } from "@components/operations/AsyncEntitySelect";
import { AsyncEntityMultiSelect } from "@components/operations/AsyncEntityMultiSelect";

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

// Ports the webapp's CreateIncidentPage.tsx, following this app's own NewChangeRequestPage.tsx for
// structural conventions (plain useState per field, a renderSelect helper, a collapsed "More
// options" Accordion for the less-common ServiceNow reference lookups). Unlike change requests,
// most of incidents' core fields are actually required — the backend hard-requires
// callerId/category/serviceId/impact/urgency/subject, and this form also requires
// subcategory/contactType client-side, same as the webapp's own validation. There is deliberately
// no priority or state field to fill in: priority is only ever a computed live preview
// (impact × urgency → ITIL matrix), and every new incident starts at ServiceNow's default state.
export default function NewIncidentPage() {
  const navigate = useNavigate();

  const [shortDescription, setShortDescription] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState<IncidentCategory | typeof UNSET>(UNSET);
  const [subcategory, setSubcategory] = useState<IncidentSubcategory | typeof UNSET>(UNSET);
  const [contactType, setContactType] = useState<IncidentContactType | typeof UNSET>(UNSET);
  const [impact, setImpact] = useState<IncidentImpact | typeof UNSET>(UNSET);
  const [urgency, setUrgency] = useState<IncidentUrgency | typeof UNSET>(UNSET);
  const [caller, setCaller] = useState<EntityOption | null>(null);
  const [service, setService] = useState<EntityOption | null>(null);
  const [serviceOffering, setServiceOffering] = useState<EntityOption | null>(null);
  const [configurationItem, setConfigurationItem] = useState<EntityOption | null>(null);
  const [assignmentGroup, setAssignmentGroup] = useState<EntityOption | null>(null);
  const [assignedEngineer, setAssignedEngineer] = useState<EntityOption | null>(null);
  const [watchList, setWatchList] = useState<EntityOption[]>([]);
  const [workNotes, setWorkNotes] = useState("");
  const [parentId, setParentId] = useState("");
  const [changeRequestId, setChangeRequestId] = useState("");
  const [problemId, setProblemId] = useState("");
  const [causedById, setCausedById] = useState("");
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Defaults "Caller" to the signed-in user, matching the webapp's own create form — the backend
  // doesn't do this itself, and callerId is one of its hard-required fields. Fires once, when the
  // current user first loads; a ref (not the field's own emptiness) gates it so manually clearing
  // the field afterward sticks.
  const { data: me } = useQuery(users.me());
  const autoFilledCaller = useRef(false);
  if (me?.id && !autoFilledCaller.current) {
    autoFilledCaller.current = true;
    setCaller({ id: me.id, label: me.fullName });
  }

  const createIncident = useMutation({ mutationFn: incidents.create });

  const subcategoryOptions = category ? SUBCATEGORY_OPTIONS_BY_CATEGORY[category] : [];
  const previewPriority = computeIncidentPriority(impact, urgency);

  const handleCategoryChange = (next: IncidentCategory | typeof UNSET): void => {
    setCategory(next);
    // A subcategory only makes sense under its own category's curated list — drop it rather than
    // leave a stale pairing.
    setSubcategory(UNSET);
  };

  const canSubmit =
    shortDescription.trim().length > 0 &&
    !!category &&
    !!subcategory &&
    !!contactType &&
    !!impact &&
    !!urgency &&
    !!caller &&
    !!service &&
    !createIncident.isPending;

  const handleSubmit = (): void => {
    if (!canSubmit || !category || !subcategory || !contactType || !impact || !urgency || !caller || !service) return;
    setSubmitError(null);

    const payload: IncidentCreatePayloadDto = {
      subject: shortDescription.trim(),
      category,
      subcategory,
      contactType,
      impact,
      urgency,
      callerId: caller.id,
      serviceId: service.id,
    };
    if (description.trim()) payload.additionalComments = description.trim();
    if (serviceOffering) payload.serviceOfferingId = serviceOffering.id;
    if (configurationItem) payload.configurationItemId = configurationItem.id;
    if (assignmentGroup) payload.assignmentGroupId = assignmentGroup.id;
    if (assignedEngineer) payload.assignedEngineerId = assignedEngineer.id;
    if (watchList.length > 0) payload.watchList = watchList.map((w) => w.id);
    if (workNotes.trim()) payload.workNotes = workNotes.trim();
    if (parentId.trim()) payload.parentId = parentId.trim();
    if (changeRequestId.trim()) payload.changeRequestId = changeRequestId.trim();
    if (problemId.trim()) payload.problemId = problemId.trim();
    if (causedById.trim()) payload.causedById = causedById.trim();

    createIncident.mutate(payload, {
      onSuccess: (created) => navigate(`/operations/incidents/${created.incident.id}`),
      onError: (err) => {
        Logger.warn("Could not create the incident", err);
        setSubmitError("Could not create the incident. Please try again.");
      },
    });
  };

  // Shared renderer for a "-- Select --" dropdown.
  const renderSelect = <T extends string>(
    id: string,
    label: string,
    value: T | typeof UNSET,
    onChange: (v: T | typeof UNSET) => void,
    options: Array<{ value: T; label: string }>,
    required = false,
    disabled = false,
  ) => (
    <FormControl fullWidth size="small" required={required} disabled={createIncident.isPending || disabled}>
      <InputLabel id={`${id}-label`} shrink>
        {label}
      </InputLabel>
      <Select
        labelId={`${id}-label`}
        label={label}
        value={value}
        displayEmpty
        onChange={(e) => onChange(e.target.value as T | typeof UNSET)}
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
    <Stack gap={2}>
      <Typography variant="h6">New Incident</Typography>

      <Stack gap={2}>
        <TextField
          label="Short description"
          value={shortDescription}
          onChange={(e) => setShortDescription(e.target.value)}
          size="small"
          fullWidth
          required
          disabled={createIncident.isPending}
          placeholder="Brief summary of the incident"
        />
        <TextField
          label="Description"
          placeholder="More detail — visible to the customer"
          size="small"
          fullWidth
          multiline
          minRows={3}
          disabled={createIncident.isPending}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          helperText="Visible to the customer — do not include internal-only details."
        />

        <Typography variant="subtitle2">Classification</Typography>

        {renderSelect("incident-category", "Category", category, handleCategoryChange, CATEGORY_OPTIONS, true)}
        {renderSelect(
          "incident-subcategory",
          "Subcategory",
          subcategory,
          setSubcategory,
          subcategoryOptions,
          true,
          !category,
        )}
        {renderSelect("incident-contact-type", "Contact type", contactType, setContactType, CONTACT_TYPE_OPTIONS, true)}
        {renderSelect("incident-impact", "Impact", impact, setImpact, IMPACT_OPTIONS, true)}
        {renderSelect("incident-urgency", "Urgency", urgency, setUrgency, URGENCY_OPTIONS, true)}

        <Stack direction="row" gap={1} alignItems="center">
          <Typography variant="body2" color="text.secondary">
            Priority
          </Typography>
          {previewPriority ? (
            <Chip
              size="small"
              label={incidentPriorityLabel(previewPriority)}
              color={incidentPriorityColor(previewPriority)}
            />
          ) : (
            <Typography variant="body2" color="text.secondary">
              Set Impact and Urgency to preview
            </Typography>
          )}
          <Typography variant="caption" color="text.secondary">
            (computed by the backing data source on creation — not sent)
          </Typography>
        </Stack>

        <Stack direction="row" gap={1} alignItems="center">
          <Typography variant="body2" color="text.secondary">
            State
          </Typography>
          <Chip size="small" label="New" />
        </Stack>

        <Typography variant="subtitle2">Requester &amp; service</Typography>

        <AsyncEntitySelect
          label="Caller"
          placeholder="Search people…"
          value={caller}
          onChange={setCaller}
          disabled={createIncident.isPending}
          search={changeRequestLookups.users}
          helperText="Defaults to you — clear it if this wasn't reported by you."
        />
        <AsyncEntitySelect
          label="Service"
          placeholder="Search services…"
          value={service}
          onChange={(next) => {
            setService(next);
            // A service offering only makes sense under its own service — drop it rather than
            // leave a stale pairing.
            setServiceOffering(null);
          }}
          disabled={createIncident.isPending}
          search={changeRequestLookups.itServices}
        />

        {/* Everything below is optional and used less often at creation time — collapsed by
            default so the form isn't dominated by fields most incidents won't need up front. */}
        <Accordion disableGutters sx={{ "&:before": { display: "none" } }}>
          <AccordionSummary expandIcon={<ChevronDown size={16} />}>
            <Typography variant="body2" color="text.secondary">
              More options (optional)
            </Typography>
          </AccordionSummary>
          <AccordionDetails sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <AsyncEntitySelect
              label="Service offering"
              placeholder="Search service offerings…"
              value={serviceOffering}
              onChange={setServiceOffering}
              disabled={createIncident.isPending}
              search={changeRequestLookups.serviceOfferings}
              searchExtra={service?.id}
              helperText={service ? undefined : "Narrows to a Service once one is picked."}
            />
            <AsyncEntitySelect
              label="Configuration item"
              placeholder="Search configuration items…"
              value={configurationItem}
              onChange={setConfigurationItem}
              disabled={createIncident.isPending}
              search={changeRequestLookups.configurationItems}
            />
            <AsyncEntitySelect
              label="Assignment group"
              placeholder="Search groups…"
              value={assignmentGroup}
              onChange={setAssignmentGroup}
              disabled={createIncident.isPending}
              search={changeRequestLookups.groups}
            />
            <AsyncEntitySelect
              label="Assigned to"
              placeholder="Search people…"
              value={assignedEngineer}
              onChange={setAssignedEngineer}
              disabled={createIncident.isPending}
              search={changeRequestLookups.users}
            />
            <AsyncEntityMultiSelect
              label="Watch list"
              placeholder="Search people…"
              value={watchList}
              onChange={setWatchList}
              disabled={createIncident.isPending}
              search={changeRequestLookups.users}
            />
            <TextField
              label="Internal work note"
              multiline
              minRows={2}
              size="small"
              fullWidth
              value={workNotes}
              onChange={(e) => setWorkNotes(e.target.value)}
              disabled={createIncident.isPending}
              helperText="Internal only — never shown to the customer."
            />

            <Typography variant="body2" color="text.secondary">
              Advanced linking
            </Typography>
            <TextField
              label="Parent ID (case/incident/CR/problem)"
              size="small"
              fullWidth
              value={parentId}
              onChange={(e) => setParentId(e.target.value)}
              disabled={createIncident.isPending}
            />
            <TextField
              label="Change request ID"
              size="small"
              fullWidth
              value={changeRequestId}
              onChange={(e) => setChangeRequestId(e.target.value)}
              disabled={createIncident.isPending}
            />
            <TextField
              label="Problem ID"
              size="small"
              fullWidth
              value={problemId}
              onChange={(e) => setProblemId(e.target.value)}
              disabled={createIncident.isPending}
            />
            <TextField
              label="Caused by ID"
              size="small"
              fullWidth
              value={causedById}
              onChange={(e) => setCausedById(e.target.value)}
              disabled={createIncident.isPending}
            />
          </AccordionDetails>
        </Accordion>

        {submitError && (
          <Typography variant="body2" color="error.main">
            {submitError}
          </Typography>
        )}

        <Stack direction="row" gap={1} justifyContent="flex-end">
          {/* navigate(-1) rather than a hardcoded /operations path — this app's Operations tab
              state isn't URL-driven, so going back preserves whichever tab (Incidents) the Fab
              was pressed from, same as NewChangeRequestPage.tsx's own Cancel button. */}
          <Button onClick={() => navigate(-1)} disabled={createIncident.isPending}>
            Cancel
          </Button>
          <Button variant="contained" disabled={!canSubmit} onClick={handleSubmit}>
            {createIncident.isPending ? "Creating…" : "Create Incident"}
          </Button>
        </Stack>
      </Stack>
    </Stack>
  );
}
