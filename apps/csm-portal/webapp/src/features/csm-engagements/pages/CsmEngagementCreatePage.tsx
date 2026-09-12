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
  FormHelperText,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useLocation, useSearchParams } from "react-router";
import Editor from "@components/rich-text-editor/Editor";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import ProjectSelectionField from "@features/csm-cases/components/ProjectSelectionField";
import { useSearchDeployments } from "@features/csm-cases/api/useSearchDeployments";
import { useDeployedProductOptions } from "@features/csm-cases/api/useDeployedProductOptions";
import { usePostCsmCase } from "@features/csm-cases/api/usePostCsmCase";
import { useNavTransition } from "@hooks/useNavTransition";
import type { BeEngagementPaymentType, BeEngagementType } from "@api/backend/types";

const ENGAGEMENT_TYPES: { value: BeEngagementType; label: string }[] = [
  { value: "migration", label: "Migration" },
  { value: "consultancy", label: "Consultancy" },
  { value: "new_feature_improvement", label: "New feature / improvement" },
  { value: "follow_up", label: "Follow up" },
  { value: "onboarding", label: "Onboarding" },
];

const ENGAGEMENT_PAYMENT_TYPES: { value: BeEngagementPaymentType; label: string }[] = [
  { value: "paid", label: "Paid" },
  { value: "foc", label: "FOC" },
];

/** The rich-text editor emits `<p></p>` when empty; check the stripped text. */
function isEmptyHtml(html: string): boolean {
  return html.replace(/<[^>]*>/g, "").replace(/&nbsp;/g, " ").trim().length === 0;
}

export default function CsmEngagementCreatePage(): JSX.Element {
  const navigate = useNavTransition();
  const { showError } = useErrorBanner();

  // Opened from a project's page (`/engagements/new?projectId=…`), the project
  // is fixed and shown read-only, mirroring CsmCaseCreatePage's convention.
  const [searchParams] = useSearchParams();
  const lockedProjectId = searchParams.get("projectId") ?? "";

  // Set when opened from a project's page Create menu (state: { from:
  // "/customers/projects/:id" }), so Back/Cancel return there instead of the
  // hardcoded engagements list, and the newly created engagement's own Back
  // button (reading this same convention) returns there too.
  const backState = useLocation().state as { from?: string } | undefined;
  const backTarget = backState?.from ?? "/engagements";

  const [projectId, setProjectId] = useState(lockedProjectId);
  const [deploymentId, setDeploymentId] = useState("");
  const [deployedProductId, setDeployedProductId] = useState("");
  const [engagementType, setEngagementType] = useState<BeEngagementType | "">("");
  const [engagementPaymentType, setEngagementPaymentType] = useState<
    BeEngagementPaymentType | ""
  >("");
  const [subject, setSubject] = useState("");
  const [description, setDescription] = useState("");

  const deployments = useSearchDeployments(projectId || undefined);
  const deployedProducts = useDeployedProductOptions(deploymentId || undefined);
  const postCase = usePostCsmCase();
  const [submitting, setSubmitting] = useState(false);

  const hasOptionsError = deployments.isError || deployedProducts.isError;
  const retryOptions = (): void => {
    if (deployments.isError) void deployments.refetch();
    if (deployedProducts.isError) void deployedProducts.refetch();
  };

  const canSubmit = useMemo(
    () =>
      !!projectId &&
      !!deploymentId &&
      !!deployedProductId &&
      !!engagementType &&
      !!engagementPaymentType &&
      subject.trim().length > 0 &&
      !isEmptyHtml(description) &&
      !submitting,
    [
      projectId,
      deploymentId,
      deployedProductId,
      engagementType,
      engagementPaymentType,
      subject,
      description,
      submitting,
    ],
  );

  const onProjectChange = (next: string): void => {
    setProjectId(next);
    setDeploymentId("");
    setDeployedProductId("");
  };
  const onDeploymentChange = (next: string): void => {
    setDeploymentId(next);
    setDeployedProductId("");
  };

  const handleSubmit = async (): Promise<void> => {
    if (!canSubmit || !engagementType || !engagementPaymentType) return;
    setSubmitting(true);
    try {
      const created = await postCase.mutateAsync({
        type: "engagement",
        projectId,
        deploymentId,
        deployedProductId,
        subject: subject.trim(),
        description,
        engagementType,
        engagementPaymentType,
      });
      navigate(`/engagements/${created.id}`, { state: { from: backTarget } });
    } catch (err) {
      setSubmitting(false);
      showError("Could not create the engagement. Please try again.", err);
    }
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
        New engagement
      </Typography>

      <Card variant="outlined" sx={{ p: 3 }}>
        {hasOptionsError && (
          <Box
            sx={{
              mb: 2,
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              flexWrap: "wrap",
            }}
          >
            <Typography variant="body2" color="error.main">
              Some dropdown options failed to load.
            </Typography>
            <Button size="small" variant="outlined" onClick={retryOptions}>
              Retry
            </Button>
          </Box>
        )}
        <Grid container spacing={2.5}>
          <Grid size={{ xs: 12, md: 4 }}>
            <ProjectSelectionField
              value={projectId}
              onChange={onProjectChange}
              lockedProjectId={lockedProjectId}
              required
            />
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="engagement-deployment-label"
                shrink={deploymentId !== ""}
                sx={{ top: "0px !important" }}
              >
                Deployment
              </InputLabel>
              <Select
                labelId="engagement-deployment-label"
                label="Deployment"
                value={deploymentId}
                onChange={(e) => onDeploymentChange(String(e.target.value))}
                disabled={!projectId || deployments.isLoading}
                notched={deploymentId !== ""}
              >
                {(deployments.data ?? []).map((d) => (
                  <MenuItem key={d.id} value={d.id}>
                    {d.name ?? d.id}
                  </MenuItem>
                ))}
              </Select>
              {!projectId ? (
                <FormHelperText>Select a project first</FormHelperText>
              ) : deployments.isLoading ? (
                <FormHelperText>Loading deployments…</FormHelperText>
              ) : (deployments.data ?? []).length === 0 ? (
                <FormHelperText>No deployments found for this project.</FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="engagement-product-label"
                shrink={deployedProductId !== ""}
                sx={{ top: "0px !important" }}
              >
                Deployed product
              </InputLabel>
              <Select
                labelId="engagement-product-label"
                label="Deployed product"
                value={deployedProductId}
                onChange={(e) => setDeployedProductId(String(e.target.value))}
                disabled={!deploymentId || deployedProducts.isLoading}
                notched={deployedProductId !== ""}
              >
                {(deployedProducts.data ?? []).map((dp) => (
                  <MenuItem key={dp.id} value={dp.id}>
                    {dp.label}
                  </MenuItem>
                ))}
              </Select>
              {!deploymentId ? (
                <FormHelperText>Select a deployment first</FormHelperText>
              ) : deployedProducts.isLoading ? (
                <FormHelperText>Loading products…</FormHelperText>
              ) : (deployedProducts.data ?? []).length === 0 ? (
                <FormHelperText>No deployed products found for this deployment.</FormHelperText>
              ) : null}
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="engagement-type-label"
                shrink={engagementType !== ""}
                sx={{ top: "0px !important" }}
              >
                Engagement type
              </InputLabel>
              <Select
                labelId="engagement-type-label"
                label="Engagement type"
                value={engagementType}
                onChange={(e) => setEngagementType(e.target.value as BeEngagementType)}
                notched={engagementType !== ""}
              >
                {ENGAGEMENT_TYPES.map((et) => (
                  <MenuItem key={et.value} value={et.value}>
                    {et.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth size="small" required>
              <InputLabel
                id="engagement-payment-type-label"
                shrink={engagementPaymentType !== ""}
                sx={{ top: "0px !important" }}
              >
                Engagement payment type
              </InputLabel>
              <Select
                labelId="engagement-payment-type-label"
                label="Engagement payment type"
                value={engagementPaymentType}
                onChange={(e) =>
                  setEngagementPaymentType(e.target.value as BeEngagementPaymentType)
                }
                notched={engagementPaymentType !== ""}
              >
                {ENGAGEMENT_PAYMENT_TYPES.map((ept) => (
                  <MenuItem key={ept.value} value={ept.value}>
                    {ept.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <TextField
              label="Subject"
              size="small"
              fullWidth
              required
              value={subject}
              onChange={(e) => setSubject(e.target.value.slice(0, 200))}
              helperText={
                subject.length >= 160 ? `${subject.length}/200` : undefined
              }
            />
          </Grid>
          <Grid size={{ xs: 12 }}>
            <Typography
              id="engagement-description-label"
              component="label"
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Description
            </Typography>
            <Box role="group" aria-labelledby="engagement-description-label">
              <Editor
                value={description}
                onChange={setDescription}
                placeholder="Describe the engagement…"
                minHeight={180}
                maxHeight={420}
                toolbarVariant="full"
                disabled={submitting}
              />
            </Box>
          </Grid>
        </Grid>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={() => navigate(backTarget)}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() => void handleSubmit()}
            disabled={!canSubmit}
          >
            {submitting ? "Creating…" : "Create engagement"}
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
