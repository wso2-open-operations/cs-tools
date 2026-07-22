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

import { Box, Button, Card, TextField, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useNavigate } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { usePostProblem } from "@features/csm-operations/api/usePostProblem";
import type { BeCreateProblemPayload } from "@api/backend/types";

const OPERATIONS_PROBLEMS_PATH = "/operations?tab=problems";

/**
 * Create-problem form against `POST /problems` (ServiceNow data source only).
 * `subject` is the only required field. There is no Priority field — priority
 * is not settable on create (SN computes/defaults it server-side, confirmed
 * by live testing) — and there's no metadata source yet for a Category /
 * Subcategory picker, so both are plain text inputs, same as the "advanced
 * linking" ID fields on `CreateIncidentPage`.
 */
export default function CreateProblemPage(): JSX.Element {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();
  const postProblem = usePostProblem();

  const [subject, setSubject] = useState("");
  const [category, setCategory] = useState("");
  const [subcategory, setSubcategory] = useState("");
  const [originCaseId, setOriginCaseId] = useState("");
  const [primaryIncidentId, setPrimaryIncidentId] = useState("");
  const [touched, setTouched] = useState(false);

  const isSubjectValid = subject.trim().length > 0;
  const canSubmit = isSubjectValid && !postProblem.isPending;

  const handleSubmit = (): void => {
    if (!canSubmit) {
      setTouched(true);
      return;
    }

    const payload: BeCreateProblemPayload = { subject: subject.trim() };
    if (category.trim()) payload.category = category.trim();
    if (subcategory.trim()) payload.subcategory = subcategory.trim();
    if (originCaseId.trim()) payload.originCaseId = originCaseId.trim();
    if (primaryIncidentId.trim()) payload.primaryIncidentId = primaryIncidentId.trim();

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
            <TextField
              label="Category"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              disabled={postProblem.isPending}
              sx={{ flex: "1 1 220px" }}
            />
            <TextField
              label="Subcategory"
              value={subcategory}
              onChange={(e) => setSubcategory(e.target.value)}
              disabled={postProblem.isPending}
              sx={{ flex: "1 1 220px" }}
            />
          </Box>

          <Typography variant="caption" color="text.secondary">
            Advanced linking (portal UUIDs — no lookup available for these yet)
          </Typography>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <TextField
              label="Origin case ID"
              size="small"
              value={originCaseId}
              onChange={(e) => setOriginCaseId(e.target.value)}
              disabled={postProblem.isPending}
              sx={{ flex: "1 1 220px" }}
            />
            <TextField
              label="Primary incident ID"
              size="small"
              value={primaryIncidentId}
              onChange={(e) => setPrimaryIncidentId(e.target.value)}
              disabled={postProblem.isPending}
              sx={{ flex: "1 1 220px" }}
            />
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
