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
  Card,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useLocation, useNavigate } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { formatBytes } from "@utils/formatBytes";
import { isBlankHtml } from "@utils/sanitizeHtml";
import Editor from "@components/rich-text-editor/Editor";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { usePostProblem } from "@features/csm-operations/api/usePostProblem";
import { useSearchCasesForSelect } from "@features/csm-operations/api/useSearchCasesForSelect";
import { useSearchIncidentsForSelect } from "@features/csm-operations/api/useSearchIncidentsForSelect";
import type { CreateProblemFromIncidentNavState } from "@features/csm-operations/utils/problems";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import type {
  BeCaseSearchView,
  BeCreateProblemPayload,
  BeIncident,
} from "@api/backend/types";

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

// Live choice-list values for problem.category / problem.subcategory
// (confirmed against the backing data source's own choice lists), hardcoded here since there is
// no metadata endpoint for problem categories/subcategories the way there is
// for cases. Subcategory is dependent on category — see
// PROBLEM_SUBCATEGORY_OPTIONS_BY_CATEGORY below.
const PROBLEM_CATEGORY_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "software", label: "Software" },
  { value: "hardware", label: "Hardware" },
  { value: "network", label: "Network" },
  { value: "database", label: "Database" },
];

const PROBLEM_SUBCATEGORY_OPTIONS_BY_CATEGORY: Record<
  string,
  Array<{ value: string; label: string }>
> = {
  hardware: [
    { value: "cpu", label: "CPU" },
    { value: "monitor", label: "Monitor" },
    { value: "disk", label: "Disk" },
    { value: "mouse", label: "Mouse" },
    { value: "keyboard", label: "Keyboard" },
    { value: "memory", label: "Memory" },
  ],
  database: [
    { value: "sql server", label: "MS SQL Server" },
    { value: "db2", label: "DB2" },
    { value: "oracle", label: "Oracle" },
  ],
  network: [
    { value: "vpn", label: "VPN" },
    { value: "dhcp", label: "DHCP" },
    { value: "wireless", label: "Wireless" },
    { value: "dns", label: "DNS" },
    { value: "ip address", label: "IP Address" },
  ],
  software: [
    { value: "email", label: "Email" },
    { value: "os", label: "Operating System" },
  ],
};

function caseSearchLabel(c: BeCaseSearchView): string {
  return [c.number, c.subject].filter(Boolean).join(" — ") || c.id;
}

function incidentSearchLabel(i: BeIncident): string {
  return [i.number, i.subject].filter(Boolean).join(" — ") || i.id || "";
}

const OPERATIONS_PROBLEMS_PATH = "/operations?tab=problems";

// Cap the optional rich-text description, which can carry base64 inline
// images — same reasoning and same 1 MiB backend cap (maxRequestBodyBytes)
// as CsmCaseCreatePage.tsx's own MAX_DESCRIPTION_CONTENT_BYTES.
const MAX_DESCRIPTION_BODY_BYTES = 1024 * 1024;
const MAX_DESCRIPTION_CONTENT_BYTES = MAX_DESCRIPTION_BODY_BYTES - 4 * 1024;

/**
 * Create-problem form against `POST /problems` (ServiceNow data source only).
 * `subject` (labeled "Problem statement" — matching ServiceNow's own label
 * for `short_description`, this field's real destination) is the only
 * required field. `description` is optional rich text; it is validated and
 * forwarded to the backend but not yet sent on to ServiceNow — see
 * entity-service's own `CreateProblem` doc comment for why. There is no
 * Priority field — priority is not settable on create (SN computes/defaults
 * it server-side, confirmed by live testing).
 */
export default function CreateProblemPage(): JSX.Element {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();
  const postProblem = usePostProblem();

  // Set when opened from a list/detail page's own "Create problem" action
  // with `state: { from: ... }` (same convention as the case-type create
  // pages), so Back/Cancel return there instead of the hardcoded problems
  // tab, and the newly created problem's own Back button (reading this same
  // convention) returns there too.
  const location = useLocation();
  const backState = location.state as { from?: string } | undefined;
  const backTarget = backState?.from ?? OPERATIONS_PROBLEMS_PATH;

  // Set when opened from an incident's own "Create problem" action, which
  // navigates here with router state so the incident's id carries over as
  // the new problem's `primaryIncidentId` without a query-string round trip.
  // See CsmIncidentDetailPage.tsx's Create menu. Discriminated by `incidentId`
  // rather than trusting the cast shape alone — a plain `{ from: ... }`
  // navigation (opened from the Problems list, not an incident) is still a
  // truthy object, so an unchecked cast would render the "Opened from an
  // incident…" notice/prefill even when there's no incident at all.
  const rawIncidentState = location.state as
    | CreateProblemFromIncidentNavState
    | { from?: string }
    | undefined;
  const originIncidentState =
    rawIncidentState && "incidentId" in rawIncidentState
      ? rawIncidentState
      : undefined;

  const [subject, setSubject] = useState(
    originIncidentState?.incidentSubject
      ? `Problem from incident ${originIncidentState.incidentNumber ?? originIncidentState.incidentId}: ${originIncidentState.incidentSubject}`
      : "",
  );
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState<string>(UNSET);
  const [subcategory, setSubcategory] = useState<string>(UNSET);
  const [originCaseId, setOriginCaseId] = useState("");
  const [primaryIncidentId, setPrimaryIncidentId] = useState(
    originIncidentState?.incidentId ?? "",
  );
  const [touched, setTouched] = useState(false);

  // UTF-8 byte size of the description; the BE caps the whole create body, so
  // mirror it here to fail fast with a clear message instead of a 413.
  const descriptionBytes = useMemo(
    () => new TextEncoder().encode(description).length,
    [description],
  );
  const descriptionOverLimit = descriptionBytes > MAX_DESCRIPTION_CONTENT_BYTES;
  const descriptionError = descriptionOverLimit
    ? `The description is too large (${formatBytes(
        descriptionBytes,
      )}). Maximum is ${formatBytes(
        MAX_DESCRIPTION_BODY_BYTES,
      )} — reduce the size or the number of inline images and try again.`
    : null;

  // Display label for a pre-filled `primaryIncidentId` above until a fresh
  // search for the same id resolves one from the backend (see
  // AsyncEntitySelect's `knownLabel`).
  const primaryIncidentKnownLabel = originIncidentState
    ? ([originIncidentState.incidentNumber, originIncidentState.incidentSubject]
        .filter(Boolean)
        .join(" — ") || originIncidentState.incidentId)
    : undefined;

  const subcategoryOptions = category
    ? (PROBLEM_SUBCATEGORY_OPTIONS_BY_CATEGORY[category] ?? [])
    : [];

  const handleCategoryChange = (next: string): void => {
    setCategory(next);
    // Drop the subcategory if it no longer belongs to the newly selected
    // category rather than leaving a stale, no-longer-valid pairing.
    const stillValid = (PROBLEM_SUBCATEGORY_OPTIONS_BY_CATEGORY[next] ?? []).some(
      (o) => o.value === subcategory,
    );
    if (!stillValid) setSubcategory(UNSET);
  };

  const isSubjectValid = subject.trim().length > 0;
  const canSubmit = isSubjectValid && !descriptionOverLimit && !postProblem.isPending;

  const handleSubmit = (): void => {
    if (!canSubmit) {
      setTouched(true);
      return;
    }

    const payload: BeCreateProblemPayload = { subject: subject.trim() };
    if (!isBlankHtml(description)) payload.description = description;
    if (category) payload.category = category;
    if (subcategory) payload.subcategory = subcategory;
    if (originCaseId) payload.originCaseId = originCaseId;
    if (primaryIncidentId) payload.primaryIncidentId = primaryIncidentId;

    postProblem.mutate(payload, {
      onSuccess: (created) =>
        navigate(`/operations/problems/${created.id}`, {
          state: { from: backTarget },
        }),
      onError: (err) => {
        // The backend surfaces real validation messages on 4xx (e.g. an
        // invalid UUID in one of the linking fields); show them.
        const msg =
          err instanceof BackendApiError && err.status < 500 && err.message
            ? err.message
            : "Could not create the problem. Please try again.";
        showError(msg, err);
      },
    });
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
      <Typography variant="h5" sx={{ mb: 2 }}>
        New problem
      </Typography>

      {originIncidentState && (
        <Alert severity="info" sx={{ mb: 2 }}>
          Opened from {originIncidentState.incidentNumber ?? "an incident"} — it's pre-filled
          below as the primary incident.
        </Alert>
      )}

      <Card variant="outlined" sx={{ p: 3 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Problem details</Typography>

          <TextField
            label="Problem statement"
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            onBlur={() => setTouched(true)}
            fullWidth
            required
            error={touched && !isSubjectValid}
            helperText={touched && !isSubjectValid ? "Required" : undefined}
            disabled={postProblem.isPending}
            placeholder="Short summary of the problem"
          />

          <Box>
            <Typography
              id="problem-description-label"
              component="label"
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Description (optional)
            </Typography>
            {/* Editor doesn't accept an `id`, so associate the label by wrapping
                the editor in a labelled group for assistive tech. */}
            <Box role="group" aria-labelledby="problem-description-label">
              <Editor
                value={description}
                onChange={setDescription}
                placeholder="Add more detail — steps, impact, screenshots…"
                minHeight={140}
                maxHeight={360}
                toolbarVariant="full"
                disabled={postProblem.isPending}
              />
            </Box>
            {descriptionError && (
              <Typography
                variant="caption"
                color="error"
                sx={{ display: "block", mt: 0.5 }}
              >
                {descriptionError}
              </Typography>
            )}
          </Box>

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <FormControl
              fullWidth
              size="small"
              disabled={postProblem.isPending}
              sx={{ flex: "1 1 220px" }}
            >
              <InputLabel id="problem-category-label" shrink>
                Category
              </InputLabel>
              <Select
                labelId="problem-category-label"
                label="Category"
                value={category}
                displayEmpty
                onChange={(e) => handleCategoryChange(String(e.target.value))}
              >
                <MenuItem value={UNSET}>
                  <Typography component="span" color="text.secondary">
                    {SELECT_PLACEHOLDER}
                  </Typography>
                </MenuItem>
                {PROBLEM_CATEGORY_OPTIONS.map((o) => (
                  <MenuItem key={o.value} value={o.value}>
                    {o.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <FormControl
              fullWidth
              size="small"
              disabled={postProblem.isPending || subcategoryOptions.length === 0}
              sx={{ flex: "1 1 220px" }}
            >
              <InputLabel id="problem-subcategory-label" shrink>
                Subcategory
              </InputLabel>
              <Select
                labelId="problem-subcategory-label"
                label="Subcategory"
                value={subcategory}
                displayEmpty
                onChange={(e) => setSubcategory(String(e.target.value))}
              >
                <MenuItem value={UNSET}>
                  <Typography component="span" color="text.secondary">
                    {SELECT_PLACEHOLDER}
                  </Typography>
                </MenuItem>
                {subcategoryOptions.map((o) => (
                  <MenuItem key={o.value} value={o.value}>
                    {o.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Box>

          <Typography variant="caption" color="text.secondary">
            Advanced linking
          </Typography>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeCaseSearchView>
                id="problem-origin-case"
                label="Origin case"
                placeholder="Search cases…"
                value={originCaseId}
                onChange={setOriginCaseId}
                disabled={postProblem.isPending}
                useSearch={useSearchCasesForSelect}
                getId={(c) => c.id}
                getLabel={caseSearchLabel}
              />
            </Box>
            <Box sx={{ flex: "1 1 220px" }}>
              <AsyncEntitySelect<BeIncident>
                id="problem-primary-incident"
                label="Primary incident"
                placeholder="Search incidents…"
                value={primaryIncidentId}
                onChange={setPrimaryIncidentId}
                disabled={postProblem.isPending}
                useSearch={useSearchIncidentsForSelect}
                // useSearchIncidentsForSelect only returns incidents that
                // have an id (server-populated), so this is never null here.
                getId={(i) => i.id!}
                getLabel={incidentSearchLabel}
                knownLabel={primaryIncidentKnownLabel}
              />
            </Box>
          </Box>
        </Box>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={() => navigate(backTarget)}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSubmit}
            disabled={!canSubmit}
            loading={postProblem.isPending}
          >
            Create problem
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
