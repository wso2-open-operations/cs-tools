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
import { useEffect, useState } from "react";

type CreateCaseFormValues = {
  project: string;
  product: string;
  deployment: string;
  type: number;
  severity: number;
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

  const { data: filters } = useSuspenseQuery(cases.filters(projectId!));
  const issueTypeOptions = filters.issueTypes.map((type) => ({ value: Number(type.id), label: type.label }));
  const severityLevelOptions = filters.severities.map((type) => ({ value: Number(type.id), label: type.label }));
  const [classified, setClassified] = useState<Set<keyof CreateCaseFormValues>>(new Set());

  const formik = useFormik<CreateCaseFormValues>({
    initialValues: {
      project: projectId!,
      product: "",
      deployment: "",
      title: "",
      description: "",
      type: issueTypeOptions[0].value,
      severity: severityLevelOptions[0].value,
    },
    onSubmit: (values) => {
      mutation.mutate({
        projectId: values.project,
        deploymentId: values.deployment,
        productId: values.product,
        title: values.title,
        description: values.description,
        issueTypeKey: values.type,
        severityKey: values.severity,
      });
    },
  });

  const deploymentQuery = useQuery({
    ...projects.deployments(projectId!),
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

  const deploymentOptions =
    deploymentQuery.data?.map((deployment) => ({ value: deployment.id, label: deployment.type })) ?? [];

  const productOptions =
    productQuery.data?.map((product) => ({
      value: product.id,
      label: product.name,
    })) ?? [];

  const mutation = useMutation({
    ...cases.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cases"] });
      setTimeout(() => {
        navigate("/support");
      }, 500);
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [classifications]);

  return (
    <>
      <Backdrop
        sx={{
          color: "primary.contrastText",
          zIndex: (theme) => theme.zIndex.drawer + 1,
          flexDirection: "column",
          gap: 2,
        }}
        open={mutation.isPending}
      >
        <CircularProgress color="inherit" />
        <Typography variant="h6" color="inherit">
          Saving your case...
        </Typography>
      </Backdrop>
      <form onSubmit={formik.handleSubmit}>
        <Stack pb={5} gap={5}>
          <Stack gap={2}>
            <SelectField
              required
              name="project"
              label="Project"
              options={projectsOptions}
              value={formik.values.project}
              onChange={formik.handleChange}
              startAdornment={
                <InputAdornment position="start">
                  <Folder size={pxToRem(20)} />
                </InputAdornment>
              }
            />
            <SelectField
              name="deployment"
              label="Deployment Type"
              aiLabel={classified.has("deployment") ? "Auto Detected" : undefined}
              placeholder="Select Deployment Type"
              options={deploymentOptions}
              value={formik.values.deployment}
              onChange={(e) => {
                formik.handleChange(e);
                formik.setFieldValue("product", "");
              }}
              disabled={!formik.values.project || deploymentQuery.isLoading}
            />
            <SelectField
              required
              name="product"
              label="Product & Version"
              aiLabel={classified.has("product") ? "Auto Detected" : undefined}
              placeholder="Select Product & Version"
              options={productOptions}
              value={formik.values.product}
              onChange={formik.handleChange}
              disabled={!formik.values.deployment || productQuery.isLoading}
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
              aiLabel={classified.has("title") ? "Generated from Chat" : undefined}
              value={formik.values.title}
              onChange={formik.handleChange}
            />
            <TextField
              required
              multiline
              name="description"
              label="Case Description"
              aiLabel={classified.has("description") ? "From Conversation" : undefined}
              value={formik.values.description}
              onChange={formik.handleChange}
            />
            <SelectField
              required
              name="type"
              label="Issue Type"
              aiLabel={classified.has("type") ? "AI Classified" : undefined}
              options={issueTypeOptions}
              value={formik.values.type}
              onChange={formik.handleChange}
            />
            <SelectField
              required
              name="severity"
              label="Severity Levels"
              aiLabel={classified.has("severity") ? "AI Classified" : undefined}
              options={severityLevelOptions}
              value={formik.values.severity}
              onChange={formik.handleChange}
            />
          </Stack>
          <ConversationSummary messages={messages} />
          <Button type="submit" variant="contained" sx={{ textTransform: "initial" }}>
            Create Case
          </Button>
        </Stack>
      </form>
    </>
  );
}
