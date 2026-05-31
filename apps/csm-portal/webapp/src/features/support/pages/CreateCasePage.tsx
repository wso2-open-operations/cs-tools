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
  Autocomplete,
  Box,
  Button,
  Card,
  CircularProgress,
  Divider,
  FormControl,
  FormHelperText,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronLeft } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useMemo,
  useState,
  type FormEvent,
  type JSX,
} from "react";
import { useNavigate } from "react-router";
import Editor from "@components/rich-text-editor/Editor";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useLogger } from "@hooks/useLogger";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { useSearchProjects } from "@features/csm-projects/api/useSearchProjects";
import { useSearchDeployments } from "@features/support/api/useSearchDeployments";
import { useSearchDeployedProducts } from "@features/support/api/useSearchDeployedProducts";
import { useSearchProducts } from "@features/support/api/useSearchProducts";
import { htmlToPlainText } from "@features/support/utils/richTextEditor";
import {
  CASE_SEVERITY,
  type CaseCreatePayload,
  type CaseSeverityLabel,
  type Deployment,
  type Product,
} from "@features/support/types/case";
import type { Project } from "@features/csm-projects/types/csmProjects";

const SEVERITY_OPTIONS: ReadonlyArray<{
  label: CaseSeverityLabel;
  description: string;
}> = [
  { label: "P0", description: "Critical — production down" },
  { label: "P1", description: "High — major impact" },
  { label: "P2", description: "Medium — limited impact" },
  { label: "P3", description: "Low — minor issue" },
  { label: "P4", description: "Informational" },
];

function formatDeploymentLabel(d: Deployment): string {
  return `${d.name} (${d.type.replace(/_/g, " ")})`;
}

/**
 * Combines a deployed-product row with its product display name. Versions are
 * intentionally omitted from the dropdown label until we add a version lookup.
 */
interface DeployedProductOption {
  deployedProductId: string;
  productId: string;
  productName: string;
}

export default function CreateCasePage(): JSX.Element {
  const navigate = useNavigate();
  const logger = useLogger();
  const { showSuccess } = useSuccessBanner();

  const [projectInput, setProjectInput] = useState("");
  const debouncedProjectInput = useDebouncedValue(projectInput, 250);
  const [selectedProject, setSelectedProject] = useState<Project | null>(null);
  const [selectedDeployment, setSelectedDeployment] = useState<Deployment | null>(
    null,
  );
  const [
    selectedDeployedProduct,
    setSelectedDeployedProduct,
  ] = useState<DeployedProductOption | null>(null);
  const [subject, setSubject] = useState("");
  const [descriptionHtml, setDescriptionHtml] = useState("");
  const [severity, setSeverity] = useState<CaseSeverityLabel>("P3");
  const [submitting, setSubmitting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  const projectsQuery = useSearchProjects({
    pagination: { limit: 20, offset: 0 },
    searchQuery: debouncedProjectInput.trim() || undefined,
  });

  const deploymentsQuery = useSearchDeployments(
    {
      pagination: { limit: 100, offset: 0 },
      projectIds: selectedProject ? [selectedProject.id] : undefined,
    },
    { enabled: Boolean(selectedProject) },
  );

  const deployedProductsQuery = useSearchDeployedProducts({
    deploymentId: selectedDeployment?.id,
    limit: 100,
  });

  const productsQuery = useSearchProducts({
    limit: 100,
    enabled: Boolean(selectedDeployment),
  });

  const handleProjectChange = useCallback((value: Project | null) => {
    setSelectedProject(value);
    setSelectedDeployment(null);
    setSelectedDeployedProduct(null);
  }, []);

  const handleDeploymentChange = useCallback((value: Deployment | null) => {
    setSelectedDeployment(value);
    setSelectedDeployedProduct(null);
  }, []);

  const productById = useMemo(() => {
    const map = new Map<string, Product>();
    for (const p of productsQuery.data?.products ?? []) map.set(p.id, p);
    return map;
  }, [productsQuery.data?.products]);

  const productOptions = useMemo<DeployedProductOption[]>(() => {
    const deployed = deployedProductsQuery.data?.deployedProducts ?? [];
    return deployed.map((dp) => ({
      deployedProductId: dp.id,
      productId: dp.productId,
      productName: productById.get(dp.productId)?.name ?? "Unknown product",
    }));
  }, [deployedProductsQuery.data?.deployedProducts, productById]);

  const descriptionPlainText = useMemo(
    () => htmlToPlainText(descriptionHtml),
    [descriptionHtml],
  );

  const validate = useCallback((): string | null => {
    if (!selectedProject) return "Select a project.";
    if (!selectedDeployment) return "Select an environment.";
    if (!selectedDeployedProduct) return "Select a product.";
    if (!subject.trim()) return "Enter a case subject.";
    if (!descriptionPlainText) return "Enter a case description.";
    return null;
  }, [
    selectedProject,
    selectedDeployment,
    selectedDeployedProduct,
    subject,
    descriptionPlainText,
  ]);

  const handleSubmit = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      const err = validate();
      if (err) {
        setValidationError(err);
        return;
      }
      setValidationError(null);

      // Cast: validate() guarantees non-null. Asserting keeps the payload typed.
      const payload: CaseCreatePayload = {
        type: "DEFAULT_CASE",
        projectId: selectedProject!.id,
        deploymentId: selectedDeployment!.id,
        deployedProductId: selectedDeployedProduct!.deployedProductId,
        title: subject.trim(),
        description: descriptionHtml,
        severityKey: CASE_SEVERITY[severity],
      };

      // Backend POST /cases is not wired yet. Log the payload so the contract
      // is visible during manual QA, show a success banner, and bounce back.
      setSubmitting(true);
      logger.info("Case create stub — backend POST /cases is pending", payload);
      window.setTimeout(() => {
        setSubmitting(false);
        showSuccess(
          "Case prepared (backend not connected yet). Payload logged to console.",
        );
        navigate("/cases");
      }, 250);
    },
    [
      validate,
      selectedProject,
      selectedDeployment,
      selectedDeployedProduct,
      subject,
      descriptionHtml,
      severity,
      logger,
      navigate,
      showSuccess,
    ],
  );

  return (
    <Box
      component="form"
      onSubmit={handleSubmit}
      sx={{ display: "flex", flexDirection: "column", gap: 3 }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Button
          startIcon={<ChevronLeft size={16} />}
          onClick={() => navigate("/cases")}
          color="inherit"
          size="small"
        >
          Cases
        </Button>
        <Typography variant="h5">New case</Typography>
      </Box>

      <Card sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2.5 }}>
        <Typography variant="subtitle1">Scope</Typography>
        <Typography variant="body2" color="text.secondary">
          Pick the project, environment, and product affected by this case.
        </Typography>

        <Autocomplete
          options={projectsQuery.data?.projects ?? []}
          value={selectedProject}
          onChange={(_, value) => handleProjectChange(value)}
          inputValue={projectInput}
          onInputChange={(_, value) => setProjectInput(value)}
          getOptionLabel={(p: Project) => p.name}
          isOptionEqualToValue={(a, b) => a.id === b.id}
          loading={projectsQuery.isFetching}
          renderInput={(params) => (
            <TextField
              {...params}
              label="Project"
              placeholder="Search projects"
              required
            />
          )}
        />

        <Autocomplete
          disabled={!selectedProject}
          options={deploymentsQuery.data?.deployments ?? []}
          value={selectedDeployment}
          onChange={(_, value) => handleDeploymentChange(value)}
          getOptionLabel={formatDeploymentLabel}
          isOptionEqualToValue={(a, b) => a.id === b.id}
          loading={deploymentsQuery.isFetching}
          renderInput={(params) => (
            <TextField
              {...params}
              label="Environment"
              placeholder={
                selectedProject
                  ? "Select an environment"
                  : "Select a project first"
              }
              required
            />
          )}
        />

        <Autocomplete
          disabled={!selectedDeployment}
          options={productOptions}
          value={selectedDeployedProduct}
          onChange={(_, value) => setSelectedDeployedProduct(value)}
          getOptionLabel={(o: DeployedProductOption) => o.productName}
          isOptionEqualToValue={(a, b) =>
            a.deployedProductId === b.deployedProductId
          }
          loading={
            deployedProductsQuery.isFetching || productsQuery.isFetching
          }
          renderInput={(params) => (
            <TextField
              {...params}
              label="Product"
              placeholder={
                selectedDeployment
                  ? "Select a product"
                  : "Select an environment first"
              }
              required
            />
          )}
        />
      </Card>

      <Card sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2.5 }}>
        <Typography variant="subtitle1">Case details</Typography>

        <TextField
          label="Subject"
          placeholder="Brief summary of the issue"
          value={subject}
          onChange={(e) => setSubject(e.target.value)}
          required
          slotProps={{ htmlInput: { maxLength: 200 } }}
        />

        <FormControl required>
          <InputLabel id="case-severity-label">Severity</InputLabel>
          <Select
            labelId="case-severity-label"
            label="Severity"
            value={severity}
            onChange={(e) => setSeverity(e.target.value as CaseSeverityLabel)}
          >
            {SEVERITY_OPTIONS.map((opt) => (
              <MenuItem key={opt.label} value={opt.label}>
                <Box>
                  <Typography component="span" variant="body2" sx={{ fontWeight: 500 }}>
                    {opt.label}
                  </Typography>
                  <Typography
                    component="span"
                    variant="body2"
                    color="text.secondary"
                    sx={{ ml: 1 }}
                  >
                    {opt.description}
                  </Typography>
                </Box>
              </MenuItem>
            ))}
          </Select>
          <FormHelperText>
            P0 is reserved for production-down incidents.
          </FormHelperText>
        </FormControl>

        <Box>
          <Typography variant="body2" sx={{ mb: 1 }}>
            Description
          </Typography>
          <Editor
            id="case-description-editor"
            value={descriptionHtml}
            onChange={setDescriptionHtml}
            placeholder="Describe the issue, steps to reproduce, and any error messages."
            minHeight={220}
            maxHeight={420}
          />
        </Box>
      </Card>

      {validationError && <Alert severity="error">{validationError}</Alert>}

      <Divider />

      <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1 }}>
        <Button
          variant="text"
          color="inherit"
          onClick={() => navigate("/cases")}
          disabled={submitting}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="contained"
          disabled={submitting}
          startIcon={submitting ? <CircularProgress size={16} /> : undefined}
        >
          {submitting ? "Submitting..." : "Create case"}
        </Button>
      </Box>
    </Box>
  );
}
