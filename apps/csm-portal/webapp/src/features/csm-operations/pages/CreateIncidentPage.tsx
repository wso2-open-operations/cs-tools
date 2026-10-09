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
  Box,
  Button,
  Card,
  Chip,
  Divider,
  FormControl,
  FormHelperText,
  InputLabel,
  Link,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { useRef, useState, type JSX, type ReactNode } from "react";
import { useLocation, useNavigate } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import type { CreateIncidentFromCaseNavState } from "@features/csm-cases/types/csmCases";
import { useGetIncidentCreateDefaults } from "@features/csm-operations/api/useGetIncidentCreateDefaults";
import { usePostIncident } from "@features/csm-operations/api/usePostIncident";
import type { CreateIncidentFromIncidentNavState } from "@features/csm-operations/utils/incidents";
import { useGetUsersMe } from "@features/settings/api/useGetUsersMe";
import { useSearchSupportGroups } from "@api/useSearchGroups";
import { useSearchItServices } from "@api/useSearchItServices";
import { useSearchServiceOfferings } from "@api/useSearchServiceOfferings";
import { useSearchConfigurationItems } from "@api/useSearchConfigurationItems";
import { useSearchInternalUsersByName } from "@api/useSearchUsersByName";
import AsyncEntitySelect, {
  type AsyncEntitySelectPinnedOption,
} from "@components/AsyncEntitySelect";
import AsyncEntityMultiSelect from "@components/AsyncEntityMultiSelect";
import { computeIncidentPriority } from "@features/csm-operations/utils/incidentPriorityMatrix";
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
import type {
  BeIncidentCategory,
  BeIncidentContactType,
  BeIncidentImpact,
  BeIncidentSubcategory,
  BeIncidentUrgency,
  BeCreateIncidentPayload,
  BeConfigurationItem,
  BeEntityRef,
  BeGroup,
  BeItService,
  BeServiceOffering,
  BeUser,
} from "@api/backend/types";

const UNSET = "" as const;
const SELECT_PLACEHOLDER = "-- Select --";

const REQUIRED_HELPER = "Required";

const OPERATIONS_INCIDENTS_PATH = "/operations?tab=incidents";

const NO_SERVICE_GROUP_HELPER = "Defaults to the selected service's support group";
const SERVICE_SUPPORT_GROUP_CAPTION = "service's support group";
const DEFAULT_TEAM_CAPTION = "default team";

/** `errorCode` of a create refused because of the assignment group (not the
 * support group of any service); any other 400 carries no code. */
const INCIDENT_ASSIGNMENT_GROUP_NOT_ALLOWED = "incident_assignment_group_not_allowed";
const ASSIGNMENT_GROUP_REFUSED_FALLBACK =
  "This group can't be assigned. Pick a group listed for a service.";

/** A create refused because of the assignment group: shown on the group
 * field rather than the banner. Keyed on the backend's stable `errorCode`
 * (`BackendApiError.payload`), never on its words. */
function isAssignmentGroupRefusal(err: unknown): err is BackendApiError {
  return (
    err instanceof BackendApiError &&
    err.status === 400 &&
    err.payload?.errorCode === INCIDENT_ASSIGNMENT_GROUP_NOT_ALLOWED
  );
}

/**
 * Create-incident form against `POST /incidents` (ServiceNow data source
 * only), styled after ServiceNow's own incident form: category-scoped
 * subcategory, a live-computed Priority badge (impact × urgency, the
 * standard ITIL matrix — see `computeIncidentPriority`), and a fixed "New"
 * State badge. Neither Priority nor State is sent on create — there's no
 * `priority` field on {@link BeCreateIncidentPayload} at all (ServiceNow
 * computes it server-side), and every new incident starts at ServiceNow's
 * own default state regardless of what the portal sends.
 */
export default function CreateIncidentPage(): JSX.Element {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();
  const postIncident = usePostIncident();

  // Set when opened from a case's "Create incident from case…" action, which
  // navigates here with router state (not query params) so the case's id
  // carries over as the new incident's parent without a query-string round
  // trip or a full page load. See CsmCaseDetailPage.tsx's `create_incident`
  // handler.
  const location = useLocation();
  const locationState = location.state as
    | CreateIncidentFromCaseNavState
    | CreateIncidentFromIncidentNavState
    | { from?: string }
    | undefined;
  // Discriminate by `caseId`/`incidentId` rather than trusting the cast
  // shape alone — `location.state` is one plain object shared by every
  // caller of this page (a case's "Create incident from case…" action, an
  // incident's own "Create child incident" action, or a plain `{ from }`
  // navigation), so an unchecked cast to either nav-state type would read as
  // truthy regardless of which one actually populated it, misapplying the
  // wrong prefill/notice (e.g. the case-origin notice rendering "Incident
  // from case undefined: …" for a child-incident navigation, whose state has
  // no `caseId` at all).
  const originCaseState =
    locationState && "caseId" in locationState ? locationState : undefined;

  // Set when opened from an incident's own "Create child incident" action,
  // which navigates here with router state so the source incident's id
  // carries over as the new incident's parent without a query-string round
  // trip or a full page load. See CsmIncidentDetailPage.tsx's Create menu.
  const originIncidentState =
    locationState && "incidentId" in locationState ? locationState : undefined;

  // Set when opened from a list/detail page's own "Create incident" action
  // with `state: { from: ... }` (same convention as the case-type create
  // pages), so Back/Cancel return there instead of the hardcoded incidents
  // tab, and the newly created incident's own Back button (reading this
  // same convention) returns there too.
  const backState = location.state as { from?: string } | undefined;
  const backTarget = backState?.from ?? OPERATIONS_INCIDENTS_PATH;

  const [shortDescription, setShortDescription] = useState(
    originCaseState?.subject
      ? `Incident from case ${originCaseState.caseNumber ?? originCaseState.caseId}: ${originCaseState.subject}`
      : originIncidentState?.subject
        ? `Child incident of ${originIncidentState.incidentNumber ?? originIncidentState.incidentId}: ${originIncidentState.subject}`
        : "",
  );
  const [description, setDescription] = useState(originCaseState?.description ?? "");
  const [category, setCategory] = useState<BeIncidentCategory | "">(UNSET);
  const [subcategory, setSubcategory] = useState<BeIncidentSubcategory | "">(UNSET);
  // "Channel" in the UI; sent as the wire field `contactType` (see CHANNEL_OPTIONS).
  const [channel, setChannel] = useState<BeIncidentContactType | "">(UNSET);
  const [impact, setImpact] = useState<BeIncidentImpact | "">(UNSET);
  const [urgency, setUrgency] = useState<BeIncidentUrgency | "">(UNSET);
  const [callerId, setCallerId] = useState("");
  const [serviceId, setServiceId] = useState("");
  const [serviceOfferingId, setServiceOfferingId] = useState("");
  const [configurationItemId, setConfigurationItemId] = useState("");
  // Assignment group. Until the user picks one themselves (`groupTouched`)
  // it follows the Service: its support group, else the default team from
  // `GET /incidents/create-defaults`. Once picked it stays put across Service
  // changes; clearing it goes back to following the Service.
  const [serviceName, setServiceName] = useState<string | null>(null);
  const [serviceSupportGroup, setServiceSupportGroup] = useState<BeEntityRef | null>(null);
  const [pickedGroup, setPickedGroup] = useState<BeEntityRef | null>(null);
  const [groupTouched, setGroupTouched] = useState(false);
  const [groupError, setGroupError] = useState<string | null>(null);
  const [assignedEngineerId, setAssignedEngineerId] = useState("");
  const [watchList, setWatchList] = useState<string[]>([]);
  const [workNotes, setWorkNotes] = useState("");
  const [parentId, setParentId] = useState(originCaseState?.caseId ?? "");
  const [parentIncidentId, setParentIncidentId] = useState(
    originIncidentState?.incidentId ?? "",
  );
  const [changeRequestId, setChangeRequestId] = useState("");
  const [problemId, setProblemId] = useState("");
  const [causedById, setCausedById] = useState("");

  // Marked true per field on blur/close, and all at once on a blocked submit
  // attempt — so required-field errors don't all appear before the user has
  // touched anything, but still surface immediately if they try to submit
  // early.
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const markTouched = (field: string): void =>
    setTouched((prev) => (prev[field] ? prev : { ...prev, [field]: true }));

  // Defaults "Caller" to the signed-in user, matching the change-request
  // form's "Requested by" default. Fires once, when the current user's id
  // first loads; a ref (not just the field's own emptiness) gates it so
  // manually clearing the field afterward sticks. The `!callerId` check
  // additionally covers the race where the user picks a different caller
  // before `me` resolves — without it, `me` loading afterward would still
  // overwrite their pick, since autoFilledCaller.current hadn't been set yet.
  // Adjusted during render (React's recommended pattern) rather than in an
  // effect, which would call setState synchronously post-commit.
  const { data: me } = useGetUsersMe();
  const meLabel = me ? userLabel(me) : undefined;
  const autoFilledCaller = useRef(false);
  if (me?.id && !autoFilledCaller.current && !callerId) {
    autoFilledCaller.current = true;
    setCallerId(me.id);
  }

  const { data: createDefaults } = useGetIncidentCreateDefaults();
  const defaultGroup = createDefaults?.defaultGroup ?? null;
  // What the field shows while not touched: nothing until a Service is
  // picked, then its support group, else the default team.
  const followedGroup: BeEntityRef | null = serviceId
    ? (serviceSupportGroup ?? defaultGroup)
    : null;
  const assignmentGroup = groupTouched ? pickedGroup : followedGroup;
  // Listed first in the dropdown: the Service's support group, else the
  // default team (also before any Service is picked).
  const pinnedGroup: AsyncEntitySelectPinnedOption | null = serviceSupportGroup
    ? { id: serviceSupportGroup.id, label: serviceSupportGroup.name, caption: SERVICE_SUPPORT_GROUP_CAPTION }
    : defaultGroup
      ? { id: defaultGroup.id, label: defaultGroup.name, caption: DEFAULT_TEAM_CAPTION }
      : null;

  const handleAssignmentGroupChange = (next: string, group?: BeGroup): void => {
    setGroupError(null);
    if (!next) {
      setGroupTouched(false);
      setPickedGroup(null);
      return;
    }
    // Picking the very group the field would follow anyway is following it.
    if (followedGroup && next === followedGroup.id) {
      setGroupTouched(false);
      setPickedGroup(null);
      return;
    }
    const name =
      group?.name ?? (pinnedGroup && pinnedGroup.id === next ? pinnedGroup.label : next);
    setPickedGroup({ id: next, name });
    setGroupTouched(true);
  };

  const followServiceGroup = (): void => {
    setGroupError(null);
    setGroupTouched(false);
    setPickedGroup(null);
  };

  let assignmentGroupHelper: ReactNode;
  if (groupError) {
    assignmentGroupHelper = groupError;
  } else if (groupTouched) {
    if (serviceId && followedGroup && followedGroup.id !== pickedGroup?.id) {
      assignmentGroupHelper = (
        <>
          {serviceSupportGroup ? "Service support group" : "Default team"}: {followedGroup.name}{" "}
          <Link component="button" type="button" variant="caption" onClick={followServiceGroup}>
            Use it
          </Link>
        </>
      );
    }
  } else if (!serviceId) {
    assignmentGroupHelper = NO_SERVICE_GROUP_HELPER;
  } else if (serviceSupportGroup) {
    assignmentGroupHelper = `Defaults to ${serviceName ?? "the service"}'s support group`;
  } else if (defaultGroup) {
    assignmentGroupHelper = `${serviceName ?? "This service"} has no support group; using the default team`;
  } else {
    assignmentGroupHelper = "No support group; pick one or it will be unassigned";
  }

  const subcategoryOptions = category ? SUBCATEGORY_OPTIONS_BY_CATEGORY[category] : [];
  const priority = computeIncidentPriority(impact || "", urgency || "");

  const handleCategoryChange = (next: string): void => {
    setCategory(next as BeIncidentCategory | "");
    // A subcategory only makes sense under its own category — drop it
    // rather than leave a stale pairing (matches the Service/Service
    // offering pattern below).
    setSubcategory(UNSET);
  };

  const isShortDescriptionValid = shortDescription.trim().length > 0;
  const isCategoryValid = !!category;
  // Subcategory is optional: entity-service (validateCreateIncidentRequest),
  // the portal backend (validateCreateIncidentBody) and the incident table
  // (nullable subcategory_id) all accept an incident with none, so it's only
  // sent when one was picked.
  const isChannelValid = !!channel;
  const isImpactValid = !!impact;
  const isUrgencyValid = !!urgency;
  // Not part of the spec's own field list, but the backend hard-requires
  // both (`validateCreateIncidentBody` 400s without them) — every submission
  // would otherwise fail, so they stay required regardless.
  const isCallerValid = !!callerId;
  const isServiceValid = !!serviceId;

  const canSubmit =
    isShortDescriptionValid &&
    isCategoryValid &&
    isChannelValid &&
    isImpactValid &&
    isUrgencyValid &&
    isCallerValid &&
    isServiceValid &&
    !postIncident.isPending;

  const handleSubmit = (): void => {
    if (!canSubmit) {
      setTouched({
        shortDescription: true,
        category: true,
        channel: true,
        impact: true,
        urgency: true,
        callerId: true,
        serviceId: true,
      });
      return;
    }

    const payload: BeCreateIncidentPayload = {
      subject: shortDescription.trim(),
      category: category as BeIncidentCategory,
      serviceId,
      contactType: channel as BeIncidentContactType,
      impact: impact as BeIncidentImpact,
      urgency: urgency as BeIncidentUrgency,
      callerId,
    };
    // No dedicated "description" field on the backend — the closest
    // equivalent is the customer-visible additionalComments journal field.
    if (subcategory) payload.subcategory = subcategory;
    if (assignmentGroup) payload.assignmentGroupId = assignmentGroup.id;
    if (description.trim()) payload.additionalComments = description.trim();
    if (serviceOfferingId) payload.serviceOfferingId = serviceOfferingId;
    if (configurationItemId) payload.configurationItemId = configurationItemId;
    if (assignedEngineerId) payload.assignedEngineerId = assignedEngineerId;
    if (watchList.length > 0) payload.watchList = watchList;
    if (workNotes.trim()) payload.workNotes = workNotes.trim();
    if (parentId.trim()) payload.parentId = parentId.trim();
    if (parentIncidentId.trim()) payload.parentIncidentId = parentIncidentId.trim();
    if (changeRequestId.trim()) payload.changeRequestId = changeRequestId.trim();
    if (problemId.trim()) payload.problemId = problemId.trim();
    if (causedById.trim()) payload.causedById = causedById.trim();

    postIncident.mutate(payload, {
      onSuccess: (created) =>
        navigate(`/operations/incidents/${created.incident.id}`, {
          state: { from: backTarget },
        }),
      onError: (err) => {
        // A refused group belongs on its own field; the form keeps its
        // values either way.
        if (isAssignmentGroupRefusal(err)) {
          setGroupError(err.payload?.message?.trim() || ASSIGNMENT_GROUP_REFUSED_FALLBACK);
          return;
        }
        // The backend surfaces real validation messages on 4xx (e.g. an
        // invalid UUID in one of the linking fields); show them.
        const msg =
          err instanceof BackendApiError && err.status < 500 && err.message
            ? err.message
            : "Could not create the incident. Please try again.";
        showError(msg, err);
      },
    });
  };

  const renderSelect = (
    fieldKey: string,
    label: string,
    value: string,
    onChange: (v: string) => void,
    options: Array<{ value: string; label: string }>,
    opts?: { required?: boolean; disabled?: boolean; helperText?: string },
  ): JSX.Element => {
    const isInvalid = !!opts?.required && !!touched[fieldKey] && !value;
    return (
      <FormControl
        fullWidth
        size="small"
        required={opts?.required}
        disabled={postIncident.isPending || opts?.disabled}
        error={isInvalid}
      >
        <InputLabel id={`${fieldKey}-label`} shrink>
          {label}
        </InputLabel>
        <Select
          labelId={`${fieldKey}-label`}
          label={label}
          value={value}
          displayEmpty
          onChange={(e) => onChange(String(e.target.value))}
          onClose={() => markTouched(fieldKey)}
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
        {(isInvalid || opts?.helperText) && (
          <FormHelperText>{isInvalid ? REQUIRED_HELPER : opts?.helperText}</FormHelperText>
        )}
      </FormControl>
    );
  };

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
      <Typography
        variant="h5"
        sx={{ mb: originCaseState || originIncidentState ? 0.5 : 2 }}
      >
        New incident
      </Typography>
      {originCaseState && (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Linked to case {originCaseState.caseNumber ?? originCaseState.caseId} — its id is
          carried through automatically as this incident's parent.
        </Typography>
      )}
      {originIncidentState && (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Linked to incident {originIncidentState.incidentNumber ?? originIncidentState.incidentId}
          {" "}— its id is carried through automatically as this incident's parent incident.
        </Typography>
      )}

      <Card variant="outlined" sx={{ p: 3 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Incident details</Typography>

          <TextField
            label="Short description"
            value={shortDescription}
            onChange={(e) => setShortDescription(e.target.value)}
            onBlur={() => markTouched("shortDescription")}
            fullWidth
            required
            error={touched.shortDescription && !isShortDescriptionValid}
            helperText={
              touched.shortDescription && !isShortDescriptionValid
                ? REQUIRED_HELPER
                : undefined
            }
            disabled={postIncident.isPending}
            placeholder="Short summary of the incident"
          />

          <TextField
            label="Description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            multiline
            minRows={3}
            fullWidth
            disabled={postIncident.isPending}
            placeholder="What's happening? Steps to reproduce, who's affected, etc."
            helperText="Visible to the customer."
          />

          <Divider />
          <Typography variant="subtitle2">Classification</Typography>

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect("category", "Category", category, handleCategoryChange, CATEGORY_OPTIONS, {
                required: true,
              })}
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect(
                "subcategory",
                "Subcategory",
                subcategory,
                (v) => setSubcategory(v as BeIncidentSubcategory | ""),
                subcategoryOptions,
                {
                  disabled: !category,
                  helperText: category ? undefined : "Pick a category first.",
                },
              )}
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect(
                "channel",
                "Channel",
                channel,
                (v) => setChannel(v as BeIncidentContactType | ""),
                CHANNEL_OPTIONS,
                { required: true },
              )}
            </Box>
          </Box>

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect(
                "impact",
                "Impact",
                impact,
                (v) => setImpact(v as BeIncidentImpact | ""),
                IMPACT_OPTIONS,
                { required: true },
              )}
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect(
                "urgency",
                "Urgency",
                urgency,
                (v) => setUrgency(v as BeIncidentUrgency | ""),
                URGENCY_OPTIONS,
                { required: true },
              )}
            </Box>
            <Box sx={{ flex: "1 1 220px", display: "flex", gap: 2, alignItems: "flex-start" }}>
              <Box sx={{ flex: 1 }}>
                <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 0.5 }}>
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
              <Box sx={{ flex: 1 }}>
                <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 0.5 }}>
                  State
                </Typography>
                <Chip size="small" label="New" variant="outlined" />
              </Box>
            </Box>
          </Box>

          <Divider />
          <Typography variant="subtitle2">
            Requester &amp; service
          </Typography>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 260px" }}>
              <AsyncEntitySelect<BeUser>
                id="incident-caller"
                label="Caller"
                required
                placeholder="Search people…"
                value={callerId}
                onChange={(v) => {
                  setCallerId(v);
                  markTouched("callerId");
                }}
                disabled={postIncident.isPending}
                useSearch={useSearchInternalUsersByName}
                // useSearchUsersByName filters out any user without an id,
                // so every option here is guaranteed to have one.
                getId={(u) => u.id!}
                getLabel={userLabel}
                knownLabel={meLabel}
                helperText={
                  touched.callerId && !isCallerValid
                    ? REQUIRED_HELPER
                    : "Defaults to you — clear it if this wasn't reported by you."
                }
              />
            </Box>
            <Box sx={{ flex: "1 1 260px" }}>
              <AsyncEntitySelect<BeItService>
                id="incident-service"
                label="Service"
                required
                placeholder="Search services…"
                value={serviceId}
                onChange={(next, service) => {
                  setServiceId(next);
                  markTouched("serviceId");
                  // A service offering only makes sense under its own
                  // service — drop it rather than leave a stale pairing.
                  setServiceOfferingId("");
                  // An untouched Assignment group follows the new Service.
                  setServiceSupportGroup(service?.supportGroup ?? null);
                  setServiceName(service ? itServiceLabel(service) : null);
                  setGroupError(null);
                }}
                disabled={postIncident.isPending}
                useSearch={useSearchItServices}
                getId={(s) => s.id}
                getLabel={itServiceLabel}
                helperText={touched.serviceId && !isServiceValid ? REQUIRED_HELPER : undefined}
              />
            </Box>
            <Box sx={{ flex: "1 1 260px" }}>
              <AsyncEntitySelect<BeGroup>
                id="incident-assignment-group"
                label="Assignment group"
                placeholder={NO_SERVICE_GROUP_HELPER}
                value={assignmentGroup?.id ?? ""}
                onChange={handleAssignmentGroupChange}
                disabled={postIncident.isPending}
                // Every service support group, whichever Service is picked.
                useSearch={useSearchSupportGroups}
                getId={(g) => g.id}
                getLabel={(g) => g.name}
                knownLabel={assignmentGroup?.name}
                pinnedOption={pinnedGroup}
                error={!!groupError}
                helperText={assignmentGroupHelper}
              />
            </Box>
          </Box>

          {/* Everything below is optional and used less often at creation
              time — collapsed by default so the form isn't dominated by
              fields most incidents won't need up front. */}
          <Accordion disableGutters sx={{ "&:before": { display: "none" }, mt: 1 }}>
            <AccordionSummary expandIcon={<ChevronDown size={16} />}>
              <Typography variant="body2" color="text.secondary">
                More options (optional)
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
                <Box sx={{ flex: "1 1 220px" }}>
                  <AsyncEntitySelect<BeServiceOffering>
                    id="incident-service-offering"
                    label="Service offering"
                    placeholder="Search service offerings…"
                    value={serviceOfferingId}
                    onChange={setServiceOfferingId}
                    disabled={postIncident.isPending}
                    useSearch={useSearchServiceOfferings}
                    searchExtra={serviceId || undefined}
                    getId={(o) => o.id}
                    getLabel={(o) => o.name}
                    helperText={serviceId ? undefined : "Narrows to a Service once one is picked."}
                  />
                </Box>
                <Box sx={{ flex: "1 1 220px" }}>
                  <AsyncEntitySelect<BeConfigurationItem>
                    id="incident-configuration-item"
                    label="Configuration item"
                    placeholder="Search configuration items…"
                    value={configurationItemId}
                    onChange={setConfigurationItemId}
                    disabled={postIncident.isPending}
                    useSearch={useSearchConfigurationItems}
                    getId={(ci) => ci.id}
                    getLabel={configurationItemLabel}
                  />
                </Box>
              </Box>

              <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
                <Box sx={{ flex: "1 1 220px" }}>
                  <AsyncEntitySelect<BeUser>
                    id="incident-assigned-engineer"
                    label="Assigned to"
                    placeholder="Search people…"
                    value={assignedEngineerId}
                    onChange={setAssignedEngineerId}
                    disabled={postIncident.isPending}
                    useSearch={useSearchInternalUsersByName}
                    getId={(u) => u.id!}
                    getLabel={userLabel}
                  />
                </Box>
              </Box>

              <AsyncEntityMultiSelect<BeUser>
                id="incident-watch-list"
                label="Watch list"
                placeholder="Search people…"
                values={watchList}
                onChange={setWatchList}
                disabled={postIncident.isPending}
                useSearch={useSearchInternalUsersByName}
                getId={(u) => u.id!}
                getLabel={userLabel}
                helperText="Notified on updates to this incident."
              />

              <TextField
                label="Internal work note"
                multiline
                minRows={2}
                value={workNotes}
                onChange={(e) => setWorkNotes(e.target.value)}
                disabled={postIncident.isPending}
                fullWidth
                helperText="Internal only — never shown to the customer."
              />

              <Typography variant="caption" color="text.secondary" sx={{ mt: 1 }}>
                Advanced linking (portal UUIDs — no lookup available for these yet)
              </Typography>
              <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
                <TextField
                  label="Parent ID (case / CR / problem)"
                  size="small"
                  value={parentId}
                  onChange={(e) => setParentId(e.target.value)}
                  disabled={postIncident.isPending}
                  helperText={
                    originCaseState
                      ? `Carried through from case ${originCaseState.caseNumber ?? originCaseState.caseId}.`
                      : undefined
                  }
                  sx={{ flex: "1 1 220px" }}
                />
                <TextField
                  label="Parent incident ID"
                  size="small"
                  value={parentIncidentId}
                  onChange={(e) => setParentIncidentId(e.target.value)}
                  disabled={postIncident.isPending}
                  helperText={
                    originIncidentState
                      ? `Carried through from incident ${originIncidentState.incidentNumber ?? originIncidentState.incidentId}.`
                      : "The major incident this is linked to."
                  }
                  sx={{ flex: "1 1 220px" }}
                />
                <TextField
                  label="Change request ID"
                  size="small"
                  value={changeRequestId}
                  onChange={(e) => setChangeRequestId(e.target.value)}
                  disabled={postIncident.isPending}
                  sx={{ flex: "1 1 220px" }}
                />
                <TextField
                  label="Problem ID"
                  size="small"
                  value={problemId}
                  onChange={(e) => setProblemId(e.target.value)}
                  disabled={postIncident.isPending}
                  sx={{ flex: "1 1 220px" }}
                />
                <TextField
                  label="Caused by ID"
                  size="small"
                  value={causedById}
                  onChange={(e) => setCausedById(e.target.value)}
                  disabled={postIncident.isPending}
                  sx={{ flex: "1 1 220px" }}
                />
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
            loading={postIncident.isPending}
          >
            Create incident
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
