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
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { useRef, useState, type JSX } from "react";
import { useNavigate } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { usePostIncident } from "@features/csm-operations/api/usePostIncident";
import { useGetUsersMe } from "@features/settings/api/useGetUsersMe";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchItServices } from "@api/useSearchItServices";
import { useSearchServiceOfferings } from "@api/useSearchServiceOfferings";
import { useSearchConfigurationItems } from "@api/useSearchConfigurationItems";
import { useSearchUsersByName } from "@api/useSearchUsersByName";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import AsyncEntityMultiSelect from "@components/AsyncEntityMultiSelect";
import { computeIncidentPriority } from "@features/csm-operations/utils/incidentPriorityMatrix";
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
import type {
  BeIncidentCategory,
  BeIncidentContactType,
  BeIncidentImpact,
  BeIncidentSubcategory,
  BeIncidentUrgency,
  BeCreateIncidentPayload,
  BeConfigurationItem,
  BeGroup,
  BeItService,
  BeServiceOffering,
  BeUser,
} from "@api/backend/types";

const UNSET = "" as const;
const SELECT_PLACEHOLDER = "-- Select --";

const REQUIRED_HELPER = "Required";

const OPERATIONS_INCIDENTS_PATH = "/operations?tab=incidents";

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

  const [shortDescription, setShortDescription] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState<BeIncidentCategory | "">(UNSET);
  const [subcategory, setSubcategory] = useState<BeIncidentSubcategory | "">(UNSET);
  const [contactType, setContactType] = useState<BeIncidentContactType | "">(UNSET);
  const [impact, setImpact] = useState<BeIncidentImpact | "">(UNSET);
  const [urgency, setUrgency] = useState<BeIncidentUrgency | "">(UNSET);
  const [callerId, setCallerId] = useState("");
  const [serviceId, setServiceId] = useState("");
  const [serviceOfferingId, setServiceOfferingId] = useState("");
  const [configurationItemId, setConfigurationItemId] = useState("");
  const [assignmentGroupId, setAssignmentGroupId] = useState("");
  const [assignedEngineerId, setAssignedEngineerId] = useState("");
  const [watchList, setWatchList] = useState<string[]>([]);
  const [workNotes, setWorkNotes] = useState("");
  const [parentId, setParentId] = useState("");
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
  const isSubcategoryValid = !!subcategory;
  const isContactTypeValid = !!contactType;
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
    isSubcategoryValid &&
    isContactTypeValid &&
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
        subcategory: true,
        contactType: true,
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
      subcategory: subcategory as BeIncidentSubcategory,
      serviceId,
      contactType: contactType as BeIncidentContactType,
      impact: impact as BeIncidentImpact,
      urgency: urgency as BeIncidentUrgency,
      callerId,
    };
    // No dedicated "description" field on the backend — the closest
    // equivalent is the customer-visible additionalComments journal field.
    if (description.trim()) payload.additionalComments = description.trim();
    if (serviceOfferingId) payload.serviceOfferingId = serviceOfferingId;
    if (configurationItemId) payload.configurationItemId = configurationItemId;
    if (assignmentGroupId) payload.assignmentGroupId = assignmentGroupId;
    if (assignedEngineerId) payload.assignedEngineerId = assignedEngineerId;
    if (watchList.length > 0) payload.watchList = watchList;
    if (workNotes.trim()) payload.workNotes = workNotes.trim();
    if (parentId.trim()) payload.parentId = parentId.trim();
    if (changeRequestId.trim()) payload.changeRequestId = changeRequestId.trim();
    if (problemId.trim()) payload.problemId = problemId.trim();
    if (causedById.trim()) payload.causedById = causedById.trim();

    postIncident.mutate(payload, {
      onSuccess: (created) => navigate(`/operations/incidents/${created.incident.id}`),
      onError: (err) => {
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
        onClick={() => navigate(OPERATIONS_INCIDENTS_PATH)}
        sx={{ mb: 1 }}
      >
        Back to operations
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New incident
      </Typography>

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
                  required: true,
                  disabled: !category,
                  helperText: category ? undefined : "Pick a category first.",
                },
              )}
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              {renderSelect(
                "contactType",
                "Contact type",
                contactType,
                (v) => setContactType(v as BeIncidentContactType | ""),
                CONTACT_TYPE_OPTIONS,
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
                placeholder="Search people…"
                value={callerId}
                onChange={(v) => {
                  setCallerId(v);
                  markTouched("callerId");
                }}
                disabled={postIncident.isPending}
                useSearch={useSearchUsersByName}
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
                placeholder="Search services…"
                value={serviceId}
                onChange={(next) => {
                  setServiceId(next);
                  markTouched("serviceId");
                  // A service offering only makes sense under its own
                  // service — drop it rather than leave a stale pairing.
                  setServiceOfferingId("");
                }}
                disabled={postIncident.isPending}
                useSearch={useSearchItServices}
                getId={(s) => s.id}
                getLabel={itServiceLabel}
                helperText={touched.serviceId && !isServiceValid ? REQUIRED_HELPER : undefined}
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
                  <AsyncEntitySelect<BeGroup>
                    id="incident-assignment-group"
                    label="Assignment group"
                    placeholder="Search groups…"
                    value={assignmentGroupId}
                    onChange={setAssignmentGroupId}
                    disabled={postIncident.isPending}
                    useSearch={useSearchGroups}
                    getId={(g) => g.id}
                    getLabel={(g) => g.name}
                  />
                </Box>
                <Box sx={{ flex: "1 1 220px" }}>
                  <AsyncEntitySelect<BeUser>
                    id="incident-assigned-engineer"
                    label="Assigned to"
                    placeholder="Search people…"
                    value={assignedEngineerId}
                    onChange={setAssignedEngineerId}
                    disabled={postIncident.isPending}
                    useSearch={useSearchUsersByName}
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
                useSearch={useSearchUsersByName}
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
                  label="Parent incident ID"
                  size="small"
                  value={parentId}
                  onChange={(e) => setParentId(e.target.value)}
                  disabled={postIncident.isPending}
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
          <Button variant="outlined" onClick={() => navigate(OPERATIONS_INCIDENTS_PATH)}>
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
