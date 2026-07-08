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
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useNavigate } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { usePostChangeRequest } from "@features/csm-operations/api/usePostChangeRequest";
import type {
  BeChangeRequestCategory,
  BeChangeRequestImpact,
  BeChangeRequestPriority,
  BeChangeRequestRisk,
  BeChangeRequestType,
  BeCreateChangeRequestPayload,
} from "@api/backend/types";

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

// Field limits and defaults below mirror the legacy ServiceNow "Create New
// Change Request" form (Short description: 500 chars; the five Planning
// journal fields: 4000 chars each; Type/Category/Impact/Risk pre-selected
// rather than left blank).
const SUBJECT_MAX = 500;
const PLAN_FIELD_MAX = 4000;

const TYPE_OPTIONS: Array<{ value: BeChangeRequestType; label: string }> = [
  { value: "standard", label: "Standard" },
  { value: "normal", label: "Normal" },
  { value: "emergency", label: "Emergency" },
  { value: "model", label: "Model" },
  { value: "site_reliability_ops", label: "Site reliability ops" },
  { value: "azure", label: "Azure" },
];

const CATEGORY_OPTIONS: Array<{ value: BeChangeRequestCategory; label: string }> = [
  { value: "hardware", label: "Hardware" },
  { value: "software", label: "Software" },
  { value: "service", label: "Service" },
  { value: "system_software", label: "System software" },
  { value: "applications_software", label: "Applications software" },
  { value: "network", label: "Network" },
  { value: "telecom", label: "Telecom" },
  { value: "documentation", label: "Documentation" },
  { value: "regular_release_cloud", label: "Regular release (cloud)" },
  { value: "hotfix_release_cloud", label: "Hotfix release (cloud)" },
  { value: "devops", label: "DevOps" },
  { value: "cloud_computing", label: "Cloud computing" },
  { value: "other", label: "Other" },
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

const RISK_OPTIONS: Array<{ value: BeChangeRequestRisk; label: string }> = [
  { value: "high", label: "High" },
  { value: "moderate", label: "Moderate" },
  { value: "low", label: "Low" },
];

/** `datetime-local` input value ("YYYY-MM-DDTHH:MM") to the BE's expected
 * "YYYY-MM-DD HH:MM:SS" string. */
function toBackendDateTime(localValue: string): string {
  return `${localValue.replace("T", " ")}:00`;
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

  const [subject, setSubject] = useState("");
  // Pre-selected to match the legacy ServiceNow form's own defaults, rather
  // than leaving every dropdown blank — most change requests are Normal
  // type, Other category, Low impact, Moderate risk. Priority has no default
  // there either ("-- None --"), so it stays unset here too.
  const [type, setType] = useState<string>("normal");
  const [category, setCategory] = useState<string>("other");
  const [impact, setImpact] = useState<string>("low");
  const [priority, setPriority] = useState<string>(UNSET);
  const [risk, setRisk] = useState<string>("moderate");
  const [plannedStartDate, setPlannedStartDate] = useState("");
  const [plannedEndDate, setPlannedEndDate] = useState("");
  const [description, setDescription] = useState("");
  const [justification, setJustification] = useState("");
  const [implementationPlan, setImplementationPlan] = useState("");
  const [riskImpactAnalysis, setRiskImpactAnalysis] = useState("");
  const [backoutPlan, setBackoutPlan] = useState("");
  const [testPlan, setTestPlan] = useState("");
  const [serviceId, setServiceId] = useState("");
  const [serviceOfferingId, setServiceOfferingId] = useState("");
  const [configurationItemId, setConfigurationItemId] = useState("");
  const [groupId, setGroupId] = useState("");
  const [assignedEngineerId, setAssignedEngineerId] = useState("");
  const [requestedById, setRequestedById] = useState("");
  const [comment, setComment] = useState("");
  const [workNote, setWorkNote] = useState("");

  const canSubmit = subject.trim().length > 0 && !postChangeRequest.isPending;

  const handleSubmit = (): void => {
    if (!canSubmit) return;

    const payload: BeCreateChangeRequestPayload = { subject: subject.trim() };
    if (type) payload.type = type as BeChangeRequestType;
    if (category) payload.category = category as BeChangeRequestCategory;
    if (impact) payload.impact = impact as BeChangeRequestImpact;
    if (priority) payload.priority = priority as BeChangeRequestPriority;
    if (risk) payload.risk = risk as BeChangeRequestRisk;
    if (plannedStartDate) payload.plannedStartDate = toBackendDateTime(plannedStartDate);
    if (plannedEndDate) payload.plannedEndDate = toBackendDateTime(plannedEndDate);
    if (description.trim()) payload.description = description.trim();
    if (justification.trim()) payload.justification = justification.trim();
    if (implementationPlan.trim()) payload.implementationPlan = implementationPlan.trim();
    if (riskImpactAnalysis.trim()) payload.riskImpactAnalysis = riskImpactAnalysis.trim();
    if (backoutPlan.trim()) payload.backoutPlan = backoutPlan.trim();
    if (testPlan.trim()) payload.testPlan = testPlan.trim();
    if (serviceId.trim()) payload.serviceId = serviceId.trim();
    if (serviceOfferingId.trim()) payload.serviceOfferingId = serviceOfferingId.trim();
    if (configurationItemId.trim()) payload.configurationItemId = configurationItemId.trim();
    if (groupId.trim()) payload.groupId = groupId.trim();
    if (assignedEngineerId.trim()) payload.assignedEngineerId = assignedEngineerId.trim();
    if (requestedById.trim()) payload.requestedById = requestedById.trim();
    if (comment.trim()) payload.comment = comment.trim();
    if (workNote.trim()) payload.workNote = workNote.trim();

    postChangeRequest.mutate(payload, {
      onSuccess: (created) =>
        navigate(`/operations/change-requests/${created.changeRequest.id}`),
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
    <FormControl fullWidth size="small" disabled={postChangeRequest.isPending}>
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

  // Shared renderer for a 4000-char-capped Planning textarea.
  const renderPlanField = (
    label: string,
    value: string,
    onChange: (v: string) => void,
    placeholder?: string,
  ): JSX.Element => (
    <TextField
      label={label}
      multiline
      minRows={3}
      value={value}
      onChange={(e) => onChange(e.target.value.slice(0, PLAN_FIELD_MAX))}
      fullWidth
      disabled={postChangeRequest.isPending}
      placeholder={placeholder}
      helperText={charsLeftHelper(value, PLAN_FIELD_MAX)}
    />
  );

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(OPERATIONS_CHANGE_REQUESTS_PATH)}
        sx={{ mb: 1 }}
      >
        Back to operations
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New change request
      </Typography>

      <Card variant="outlined" sx={{ p: 3 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Change request</Typography>

          <TextField
            label="Subject"
            value={subject}
            onChange={(e) => setSubject(e.target.value.slice(0, SUBJECT_MAX))}
            fullWidth
            required
            disabled={postChangeRequest.isPending}
            placeholder="Short summary of the change"
            helperText={charsLeftHelper(subject, SUBJECT_MAX)}
          />

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 200px" }}>
              {renderSelect("cr-type", "Type", type, setType, TYPE_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 200px" }}>
              {renderSelect("cr-category", "Category", category, setCategory, CATEGORY_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 200px" }}>
              {renderSelect("cr-priority", "Priority", priority, setPriority, PRIORITY_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 200px" }}>
              {renderSelect("cr-impact", "Impact", impact, setImpact, IMPACT_OPTIONS)}
            </Box>
            <Box sx={{ flex: "1 1 200px" }}>
              {renderSelect("cr-risk", "Risk", risk, setRisk, RISK_OPTIONS)}
            </Box>
          </Box>

          <Typography variant="subtitle2" sx={{ mt: 1 }}>
            Planning
          </Typography>

          <TextField
            label="Description"
            multiline
            minRows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            fullWidth
            disabled={postChangeRequest.isPending}
            placeholder="What is changing?"
          />
          {renderPlanField(
            "Justification",
            justification,
            setJustification,
            "Why is this change needed?",
          )}
          {renderPlanField("Implementation plan", implementationPlan, setImplementationPlan)}
          {renderPlanField("Risk and impact analysis", riskImpactAnalysis, setRiskImpactAnalysis)}
          {renderPlanField("Backout plan", backoutPlan, setBackoutPlan)}
          {renderPlanField("Test plan", testPlan, setTestPlan)}

          <Typography variant="subtitle2" sx={{ mt: 1 }}>
            Schedule
          </Typography>

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <TextField
              label="Planned start"
              type="datetime-local"
              value={plannedStartDate}
              onChange={(e) => setPlannedStartDate(e.target.value)}
              disabled={postChangeRequest.isPending}
              size="small"
              sx={{ flex: "1 1 240px" }}
              slotProps={{ inputLabel: { shrink: true } }}
            />
            <TextField
              label="Planned end"
              type="datetime-local"
              value={plannedEndDate}
              onChange={(e) => setPlannedEndDate(e.target.value)}
              disabled={postChangeRequest.isPending}
              size="small"
              sx={{ flex: "1 1 240px" }}
              slotProps={{ inputLabel: { shrink: true } }}
            />
          </Box>

          {/* Everything below is optional and used less often at creation
              time — collapsed by default so the form isn't dominated by
              fields most requests won't need up front. */}
          <Accordion disableGutters sx={{ "&:before": { display: "none" }, mt: 1 }}>
            <AccordionSummary expandIcon={<ChevronDown size={16} />}>
              <Typography variant="body2" color="text.secondary">
                More options (optional)
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
              {/* These map to two different ServiceNow journal fields — mixing
                  them up would either leak an internal note to the customer or
                  bury something they were meant to see, so the distinction is
                  spelled out rather than left to a generic "Comment" label. */}
              <TextField
                label="Additional comments (customer-visible)"
                multiline
                minRows={2}
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                disabled={postChangeRequest.isPending}
                fullWidth
                helperText="Visible to the customer — do not include internal-only details."
              />
              <TextField
                label="Internal work note"
                multiline
                minRows={2}
                value={workNote}
                onChange={(e) => setWorkNote(e.target.value)}
                disabled={postChangeRequest.isPending}
                fullWidth
                helperText="Internal only — never shown to the customer."
              />

              {/* No lookup/search UI exists for these ServiceNow CMDB
                  references yet, so they're plain UUID text inputs — the
                  backend validates the format and rejects an invalid one. */}
              <Typography variant="caption" color="text.secondary">
                The fields below take a ServiceNow record's UUID directly —
                there's no picker for these yet.
              </Typography>
              <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
                <TextField
                  label="Service ID"
                  value={serviceId}
                  onChange={(e) => setServiceId(e.target.value)}
                  disabled={postChangeRequest.isPending}
                  size="small"
                  sx={{ flex: "1 1 220px" }}
                  placeholder="UUID"
                />
                <TextField
                  label="Service offering ID"
                  value={serviceOfferingId}
                  onChange={(e) => setServiceOfferingId(e.target.value)}
                  disabled={postChangeRequest.isPending}
                  size="small"
                  sx={{ flex: "1 1 220px" }}
                  placeholder="UUID"
                />
                <TextField
                  label="Configuration item ID"
                  value={configurationItemId}
                  onChange={(e) => setConfigurationItemId(e.target.value)}
                  disabled={postChangeRequest.isPending}
                  size="small"
                  sx={{ flex: "1 1 220px" }}
                  placeholder="UUID"
                />
                <TextField
                  label="Group ID"
                  value={groupId}
                  onChange={(e) => setGroupId(e.target.value)}
                  disabled={postChangeRequest.isPending}
                  size="small"
                  sx={{ flex: "1 1 220px" }}
                  placeholder="UUID"
                />
                <TextField
                  label="Assigned engineer ID"
                  value={assignedEngineerId}
                  onChange={(e) => setAssignedEngineerId(e.target.value)}
                  disabled={postChangeRequest.isPending}
                  size="small"
                  sx={{ flex: "1 1 220px" }}
                  placeholder="UUID"
                />
                <TextField
                  label="Requested by ID"
                  value={requestedById}
                  onChange={(e) => setRequestedById(e.target.value)}
                  disabled={postChangeRequest.isPending}
                  size="small"
                  sx={{ flex: "1 1 220px" }}
                  placeholder="UUID"
                  helperText="Left blank, ServiceNow records no requester — unlike the old form, this doesn't default to you."
                />
              </Box>
            </AccordionDetails>
          </Accordion>
        </Box>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button
            variant="outlined"
            onClick={() => navigate(OPERATIONS_CHANGE_REQUESTS_PATH)}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSubmit}
            disabled={!canSubmit}
            loading={postChangeRequest.isPending}
          >
            Create change request
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
