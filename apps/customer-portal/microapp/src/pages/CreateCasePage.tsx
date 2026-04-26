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

import { useLocation, useNavigate } from "react-router-dom";
import { Folder } from "@wso2/oxygen-ui-icons-react";
import { Button, Stack, Typography, InputAdornment, pxToRem, CircularProgress, Grid, Box } from "@wso2/oxygen-ui";
import { SelectField, TextField, ConversationSummary } from "@components/features/create";
import { useFormik } from "formik";
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useProject } from "@context/project";
import { projects } from "@src/services/projects";
import { cases } from "@src/services/cases";
import type { CaseClassificationResponseDto, Case } from "@src/types";
import { useEffect, useMemo, useState } from "react";
import * as Yup from "yup";
import { overrideOrDefault } from "../utils/others";
import { useNotify } from "../context/snackbar";
import { useFilters } from "../context/filters";
import { useLayout } from "../context/layout";
import { RichText, SectionCard } from "../components/shared";
import { InfoField } from "../components/features/detail";
import DOMPurify from "dompurify";
import { DEPLOYMENT_DISABLED_PROJECT_TYPES } from "../config/constants";

type CreateCaseFormValues = {
  project: string;
  product: string;
  deployment: string;
  type: string;
  severity: string;
  title: string;
  description: string;
};

export default function CreateCasePage() {
  const navigate = useNavigate();
  const location = useLocation();
  const layout = useLayout();
  const messages = location.state?.messages || [];
  const classifications: CaseClassificationResponseDto = location.state?.classifications;
  const relatedCase: Case | undefined = location.state?.case;
  const queryClient = useQueryClient();
  const { projectId, type } = useProject();
  const notify = useNotify();

  const [classified, setClassified] = useState<Set<keyof CreateCaseFormValues>>(new Set());

  const formik = useFormik<CreateCaseFormValues>({
    initialValues: {
      project: projectId!,
      product: "",
      deployment: "",
      title: "",
      description: "",
      type: "",
      severity: "",
    },
    validationSchema: createCaseValidationSchema,
    validateOnBlur: true,
    validateOnChange: true,
    onSubmit: async (values) => {
      mutation.mutateAsync({
        type: "default_case",
        projectId: values.project,
        deploymentId: values.deployment,
        deployedProductId: values.product,
        title: values.title,
        description: values.description,
        issueTypeKey: Number(values.type),
        severityKey: Number(values.severity),
        relatedCaseId: relatedCase?.id,
      });
    },
  });

  const { data: filters } = useFilters();
  const issueTypeOptions = filters?.issueTypes.map((type) => ({ value: Number(type.id), label: type.label }));
  const severityLevelOptions = filters?.severities.map((type) => ({
    value: Number(type.id),
    label: overrideOrDefault(type.label),
  }));

  const { data: features } = useQuery(projects.features(projectId!));

  const deploymentQuery = useQuery({
    ...projects.deployments(formik.values.project),
    enabled: !!formik.values.project,
  });

  const productQuery = useQuery({
    ...projects.products(formik.values.deployment, {
      filters: { productCategories: features?.defaultCaseProductCategories ?? undefined },
    }),
    enabled: !!formik.values.deployment,
  });

  const projectsOptions = useSuspenseQuery(projects.all()).data.map((project) => ({
    value: project.id,
    label: project.name,
  }));

  const deploymentsFieldDisabled = type ? DEPLOYMENT_DISABLED_PROJECT_TYPES.includes(type) : false;

  const deploymentOptions = useMemo(() => {
    const data = deploymentQuery.data ?? [];

    const filteredData = deploymentsFieldDisabled
      ? data.filter((deployment) => deployment.type === "Primary Production").slice(0, 1)
      : data;

    return filteredData.map((deployment) => ({
      value: deployment.id,
      label: deployment.name,
    }));
  }, [deploymentQuery.data, deploymentsFieldDisabled]);

  const productOptions = useMemo(
    () =>
      productQuery.data?.map((product) => ({
        value: product.id,
        label: `${product.name} ${product.version}`,
      })) ?? [],
    [productQuery.data],
  );

  const mutation = useMutation({
    ...cases.create,
    onSuccess: ({ id }) => {
      queryClient.invalidateQueries({ queryKey: ["cases"] });
      setTimeout(() => {
        navigate(`/cases/${id}`);
      }, 500);
    },
    onError: () => {
      notify.error("Failed to create case. Please try again.");
    },
  });

  useEffect(() => {
    if (!classifications) return;

    const autoFilledFields = new Set<keyof CreateCaseFormValues>();

    const matchedDeployment = deploymentOptions.find((option) => option.label === classifications.caseInfo.environment);
    if (matchedDeployment && !deploymentsFieldDisabled) {
      formik.setFieldValue("deployment", matchedDeployment.value);
      autoFilledFields.add("deployment");
    }

    const matchedProduct = productOptions.find((option) => option.label === classifications.caseInfo.productName);
    if (matchedProduct) {
      formik.setFieldValue("product", matchedProduct.value);
      autoFilledFields.add("product");
    }

    const matchedType = issueTypeOptions?.find((option) => option.label === classifications.issueType);

    if (matchedType) {
      formik.setFieldValue("type", matchedType.value);
      autoFilledFields.add("type");
    }

    const matchedSeverity = severityLevelOptions?.find((option) =>
      option.label.includes(classifications.severityLevel),
    );

    if (matchedSeverity) {
      formik.setFieldValue("severity", matchedSeverity.value);
      autoFilledFields.add("severity");
    }

    if (classifications.caseInfo.shortDescription) {
      formik.setFieldValue("title", classifications.caseInfo.shortDescription);
      autoFilledFields.add("title");
    }

    if (classifications.caseInfo.description) {
      formik.setFieldValue("description", classifications.caseInfo.description);
      autoFilledFields.add("description");
    }

    setClassified(autoFilledFields);
  }, [classifications, deploymentOptions]);

  useEffect(() => {
    if (!relatedCase) return;

    layout.setTitleOverride("Create Related Case");

    formik.setFieldValue("title", relatedCase.title);

    const matchedDeployment = deploymentOptions.find((option) => option.label === relatedCase.deployment);
    if (matchedDeployment) {
      formik.setFieldValue("deployment", matchedDeployment.value);
    }

    const matchedProduct = productOptions.find((option) => option.label === relatedCase.product);
    if (matchedProduct) {
      formik.setFieldValue("product", matchedProduct.value);
    }

    return () => layout.setTitleOverride(undefined);
  }, [relatedCase, deploymentOptions]);

  useEffect(() => {
    if (!deploymentsFieldDisabled) return;
    if (!deploymentOptions.length) return;

    formik.setFieldValue("deployment", deploymentOptions[0].value);
  }, [deploymentsFieldDisabled]);

  return (
    <>
      {relatedCase && <RelatedCaseSection relatedCase={relatedCase} />}
      <form onSubmit={formik.handleSubmit}>
        <Stack pb={5} gap={5}>
          <Stack gap={2}>
            <SelectField
              required
              disabled
              name="project"
              label="Project"
              options={projectsOptions}
              value={formik.values.project}
              onChange={(e) => {
                formik.handleChange(e);
                formik.setFieldValue("deployment", "");
                formik.setFieldValue("product", "");
                setClassified((prev) => {
                  const next = new Set(prev);
                  next.delete("deployment");
                  next.delete("product");
                  return next;
                });
              }}
              startAdornment={
                <InputAdornment position="start">
                  <Folder size={pxToRem(20)} />
                </InputAdornment>
              }
            />
            <SelectField
              required
              name="deployment"
              label="Deployment Type"
              aiLabel={classified.has("deployment") ? "Auto Detected" : undefined}
              placeholder="Select Deployment Type"
              options={deploymentOptions}
              value={formik.values.deployment}
              onChange={(e) => {
                formik.handleChange(e);
                formik.setFieldValue("product", "");
                setClassified((prev) => {
                  const next = new Set(prev);
                  next.delete(e.target.name as keyof CreateCaseFormValues);
                  next.delete("product");
                  return next;
                });
              }}
              disabled={
                !!relatedCase || !formik.values.project || deploymentQuery.isLoading || deploymentsFieldDisabled
              }
              error={formik.touched.deployment && Boolean(formik.errors.deployment)}
              helperText={formik.touched.deployment && formik.errors.deployment ? formik.errors.deployment : undefined}
            />
            <SelectField
              required
              name="product"
              label="Product & Version"
              aiLabel={classified.has("product") ? "Auto Detected" : undefined}
              placeholder="Select Product & Version"
              options={productOptions}
              value={formik.values.product}
              onChange={(e) => {
                formik.handleChange(e);
                setClassified((prev) => {
                  const next = new Set(prev);
                  next.delete(e.target.name as keyof CreateCaseFormValues);
                  return next;
                });
              }}
              disabled={!!relatedCase || !formik.values.deployment || productQuery.isLoading}
              error={formik.values.deployment ? formik.touched.product && Boolean(formik.errors.product) : false}
              helperText={
                formik.values.deployment
                  ? formik.touched.product && formik.errors.product
                    ? formik.errors.product
                    : undefined
                  : undefined
              }
            />
          </Stack>
          <Stack gap={2}>
            <Typography variant="body1" fontWeight="medium">
              Case Details
            </Typography>

            {relatedCase && (
              <TextField required disabled name="relatedCaseId" label="Related Case ID" value={relatedCase.id} />
            )}

            <TextField
              required
              disabled={!!relatedCase}
              name="title"
              label="Issue Title"
              placeholder="Briefly describe the issue"
              aiLabel={classified.has("title") ? "Generated from Chat" : undefined}
              value={formik.values.title}
              onChange={(e) => {
                formik.handleChange(e);
                setClassified((prev) => {
                  const next = new Set(prev);
                  next.delete(e.target.name as keyof CreateCaseFormValues);
                  return next;
                });
              }}
              error={formik.touched.title && Boolean(formik.errors.title)}
              helperText={formik.touched.title && formik.errors.title ? formik.errors.title : undefined}
            />

            <TextField
              required
              multiline
              name="description"
              label="Case Description"
              placeholder="Explain the issue, including any relevant details"
              aiLabel={classified.has("description") ? "From Conversation" : undefined}
              value={formik.values.description}
              onChange={(e) => {
                formik.handleChange(e);
                setClassified((prev) => {
                  const next = new Set(prev);
                  next.delete(e.target.name as keyof CreateCaseFormValues);
                  return next;
                });
              }}
              error={formik.touched.description && Boolean(formik.errors.description)}
              helperText={
                formik.touched.description && formik.errors.description ? formik.errors.description : undefined
              }
            />

            <SelectField
              required
              name="type"
              label="Issue Type"
              placeholder="Select Issue Type"
              aiLabel={classified.has("type") ? "AI Classified" : undefined}
              options={issueTypeOptions ?? []}
              value={formik.values.type}
              onChange={(e) => {
                formik.handleChange(e);
                setClassified((prev) => {
                  const next = new Set(prev);
                  next.delete(e.target.name as keyof CreateCaseFormValues);
                  return next;
                });
              }}
              error={formik.touched.type && Boolean(formik.errors.type)}
              helperText={formik.touched.type && formik.errors.type ? formik.errors.type : undefined}
            />

            <SelectField
              required
              name="severity"
              label="Severity Levels"
              placeholder="Select Severity"
              aiLabel={classified.has("severity") ? "AI Classified" : undefined}
              options={severityLevelOptions ?? []}
              value={formik.values.severity}
              onChange={(e) => {
                formik.handleChange(e);
                setClassified((prev) => {
                  const next = new Set(prev);
                  next.delete(e.target.name as keyof CreateCaseFormValues);
                  return next;
                });
              }}
              error={formik.touched.severity && Boolean(formik.errors.severity)}
              helperText={formik.touched.severity && formik.errors.severity ? formik.errors.severity : undefined}
            />
          </Stack>

          {messages.length > 0 && <ConversationSummary messages={messages} />}

          <Button
            type="submit"
            variant="contained"
            startIcon={
              formik.isSubmitting || mutation.isPending ? <CircularProgress size={16} color="inherit" /> : undefined
            }
            sx={{ textTransform: "initial" }}
          >
            {formik.isSubmitting ? "Saving..." : "Create Case"}
          </Button>
        </Stack>
      </form>
    </>
  );
}

const createCaseValidationSchema = Yup.object({
  project: Yup.string().required("Project is required"),
  deployment: Yup.string().required("Deployment type is required"),
  product: Yup.string().required("Product & version is required"),
  title: Yup.string()
    .trim()
    .min(5, "Title must be at least 5 characters")
    .max(200, "Title must be 200 characters or less")
    .required("Issue title is required"),
  description: Yup.string()
    .trim()
    .min(20, "Description must be at least 20 characters")
    .max(5000, "Description must be 5000 characters or less")
    .required("Case description is required"),
  type: Yup.number().typeError("Issue type is required").required("Issue type is required"),
  severity: Yup.number().typeError("Severity level is required").required("Severity level is required"),
});

function RelatedCaseSection({ relatedCase }: { relatedCase: Case }) {
  return (
    <Box mb={2}>
      <SectionCard>
        <Grid spacing={1.5} container>
          <Grid size={12}>
            <InfoField label="Related Case ID" value={relatedCase.id} />
          </Grid>
          <Grid size={12}>
            <InfoField label="Title" value={relatedCase.title} />
          </Grid>
          <Grid size={12}>
            <InfoField
              label="Description"
              value={<RichText dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(relatedCase.description) }} />}
            />
          </Grid>
        </Grid>
      </SectionCard>
    </Box>
  );
}
