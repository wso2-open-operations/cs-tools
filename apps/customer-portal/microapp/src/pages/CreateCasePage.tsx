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
import { Button, Stack, Typography, InputAdornment, pxToRem, Backdrop, CircularProgress } from "@wso2/oxygen-ui";
import { SelectField, TextField, ConversationSummary } from "@components/features/create";
import { useFormik } from "formik";
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useProject } from "@context/project";
import { projects } from "@src/services/projects";
import { cases } from "@src/services/cases";
import type { CaseClassificationResponseDTO } from "@src/types";
import { useEffect, useMemo, useState } from "react";
import * as Yup from "yup";
import { overrideOrDefault } from "../utils/others";
import { useNotify } from "../context/snackbar";

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
  const messages = location.state?.messages || [];
  const classifications: CaseClassificationResponseDTO = location.state?.classifications;
  const queryClient = useQueryClient();
  const { projectId } = useProject();
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
    onSubmit: (values) => {
      mutation.mutate({
        type: "default_case",
        projectId: values.project,
        deploymentId: values.deployment,
        deployedProductId: values.product,
        title: values.title,
        description: values.description,
        issueTypeKey: Number(values.type),
        severityKey: Number(values.severity),
      });
    },
  });

  const { data: filters } = useSuspenseQuery(cases.filters(formik.values.project));
  const issueTypeOptions = filters.issueTypes.map((type) => ({ value: Number(type.id), label: type.label }));
  const severityLevelOptions = filters.severities.map((type) => ({
    value: Number(type.id),
    label: overrideOrDefault(type.label),
  }));

  const deploymentQuery = useQuery({
    ...projects.deployments(formik.values.project),
    enabled: !!formik.values.project,
  });

  const productQuery = useQuery({
    ...projects.products(formik.values.deployment),
    enabled: !!formik.values.deployment,
  });

  const projectsOptions = useSuspenseQuery(projects.all()).data.map((project) => ({
    value: project.id,
    label: project.name,
  }));

  const deploymentOptions = useMemo(
    () => deploymentQuery.data?.map((deployment) => ({ value: deployment.id, label: deployment.name })) ?? [],
    [deploymentQuery.data],
  );

  const productOptions = useMemo(
    () =>
      productQuery.data?.map((product) => ({
        value: product.id,
        label: product.name,
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
    if (matchedDeployment) {
      formik.setFieldValue("deployment", matchedDeployment.value);
      autoFilledFields.add("deployment");
    }

    const matchedProduct = productOptions.find((option) => option.label === classifications.caseInfo.productName);
    if (matchedProduct) {
      formik.setFieldValue("product", matchedProduct.value);
      autoFilledFields.add("product");
    }

    const matchedType = issueTypeOptions.find((option) => option.label === classifications.issueType);
    if (matchedType) {
      formik.setFieldValue("type", matchedType.value);
      autoFilledFields.add("type");
    }

    const matchedSeverity = severityLevelOptions.find((option) => option.label.includes(classifications.severityLevel));
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

  return (
    <>
      <form onSubmit={formik.handleSubmit}>
        <Stack pb={5} gap={5}>
          <Stack gap={2}>
            <SelectField
              required
              name="project"
              label="Project"
              options={projectsOptions}
              value={formik.values.project}
              onChange={(e) => {
                formik.handleChange(e);
                formik.setFieldValue("deployment", "");
                formik.setFieldValue("product", "");
                classified.delete("deployment");
                classified.delete("product");
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
                classified.delete(e.target.name as keyof CreateCaseFormValues);
                classified.delete("product");
              }}
              disabled={!formik.values.project || deploymentQuery.isLoading}
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
                classified.delete(e.target.name as keyof CreateCaseFormValues);
              }}
              disabled={!formik.values.deployment || productQuery.isLoading}
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
            <TextField
              required
              name="title"
              label="Issue Title"
              placeholder="Briefly describe the issue"
              aiLabel={classified.has("title") ? "Generated from Chat" : undefined}
              value={formik.values.title}
              onChange={(e) => {
                formik.handleChange(e);
                classified.delete(e.target.name as keyof CreateCaseFormValues);
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
                classified.delete(e.target.name as keyof CreateCaseFormValues);
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
              options={issueTypeOptions}
              value={formik.values.type}
              onChange={(e) => {
                formik.handleChange(e);
                classified.delete(e.target.name as keyof CreateCaseFormValues);
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
              options={severityLevelOptions}
              value={formik.values.severity}
              onChange={(e) => {
                formik.handleChange(e);
                classified.delete(e.target.name as keyof CreateCaseFormValues);
              }}
              error={formik.touched.severity && Boolean(formik.errors.severity)}
              helperText={formik.touched.severity && formik.errors.severity ? formik.errors.severity : undefined}
            />
          </Stack>
          {messages.length > 0 && <ConversationSummary messages={messages} />}
          <Button
            type="submit"
            variant="contained"
            startIcon={formik.isSubmitting ? <CircularProgress size={16} color="inherit" /> : undefined}
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
