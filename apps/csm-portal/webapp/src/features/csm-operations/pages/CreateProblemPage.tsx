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
  Card,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useNavigate } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { usePostProblem } from "@features/csm-operations/api/usePostProblem";
import { useSearchCasesForSelect } from "@features/csm-operations/api/useSearchCasesForSelect";
import { useSearchIncidentsForSelect } from "@features/csm-operations/api/useSearchIncidentsForSelect";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import type {
  BeCaseSearchView,
  BeCreateProblemPayload,
  BeIncident,
} from "@api/backend/types";

const UNSET = "";
const SELECT_PLACEHOLDER = "-- Select --";

// Live SN choice-list values for problem.category / problem.subcategory
// (confirmed via sys_choice against wso2sndev), hardcoded here since there is
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

/**
 * Create-problem form against `POST /problems` (ServiceNow data source only).
 * `subject` is the only required field. There is no Priority field — priority
 * is not settable on create (SN computes/defaults it server-side, confirmed
 * by live testing).
 */
export default function CreateProblemPage(): JSX.Element {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();
  const postProblem = usePostProblem();

  const [subject, setSubject] = useState("");
  const [category, setCategory] = useState<string>(UNSET);
  const [subcategory, setSubcategory] = useState<string>(UNSET);
  const [originCaseId, setOriginCaseId] = useState("");
  const [primaryIncidentId, setPrimaryIncidentId] = useState("");
  const [touched, setTouched] = useState(false);

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
  const canSubmit = isSubjectValid && !postProblem.isPending;

  const handleSubmit = (): void => {
    if (!canSubmit) {
      setTouched(true);
      return;
    }

    const payload: BeCreateProblemPayload = { subject: subject.trim() };
    if (category) payload.category = category;
    if (subcategory) payload.subcategory = subcategory;
    if (originCaseId) payload.originCaseId = originCaseId;
    if (primaryIncidentId) payload.primaryIncidentId = primaryIncidentId;

    postProblem.mutate(payload, {
      onSuccess: (created) => navigate(`/operations/problems/${created.id}`),
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
        onClick={() => navigate(OPERATIONS_PROBLEMS_PATH)}
        sx={{ mb: 1 }}
      >
        Back to operations
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New problem
      </Typography>

      <Card variant="outlined" sx={{ p: 3 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Problem details</Typography>

          <TextField
            label="Subject"
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
              />
            </Box>
          </Box>
        </Box>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={() => navigate(OPERATIONS_PROBLEMS_PATH)}>
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
