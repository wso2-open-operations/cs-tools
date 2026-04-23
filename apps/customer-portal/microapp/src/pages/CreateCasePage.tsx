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
import { Circle, Folder } from "@wso2/oxygen-ui-icons-react";
import { Button, Stack, Typography, InputAdornment, pxToRem, colors } from "@wso2/oxygen-ui";
import { SelectField, TextField, ConversationSummary } from "@components/features/create";
import { useFormik } from "formik";
import { useEffect } from "react";

type CreateCaseFormValues = {
  project: number;
  product: number;
  deployment: number;
  type: number;
  severity: number;
  title: string;
  description: string;
};

export default function CreateCasePage() {
  const navigate = useNavigate();
  const location = useLocation();
  const messages = location.state?.messages || [];

  const projects = [
    { value: 0, label: "Dreamworks Inc" },
    { value: 1, label: "Newsline Enterprise" },
    { value: 2, label: "Goods Store Mart" },
  ];

  const products = [
    { value: 0, label: "WSO2 API Manager v4.2.0" },
    { value: 1, label: "WSO2 Identity Access Manager v4.2.0" },
  ];

  const deploymentTypes = [
    { value: 0, label: "Production" },
    { value: 1, label: "Staging" },
    { value: 2, label: "Development" },
  ];

  const issueTypes = [
    { value: 0, label: "Configuration Issue" },
    { value: 1, label: "Query" },
    { value: 2, label: "Security Vulnerability" },
  ];

  const severityLevels = [
    {
      value: 0,
      label: (
        <Stack direction="row" alignItems="center" gap={1}>
          <Circle fill={colors.red[500]} color={colors.red[500]} size={pxToRem(12)} />
          S1 Critical
        </Stack>
      ),
    },
    {
      value: 1,
      label: (
        <Stack direction="row" alignItems="center" gap={1}>
          <Circle fill={colors.orange[500]} color={colors.orange[500]} size={pxToRem(12)} />
          S2 Medium
        </Stack>
      ),
    },
    {
      value: 2,
      label: (
        <Stack direction="row" alignItems="center" gap={1}>
          <Circle fill={colors.blue[500]} color={colors.blue[500]} size={pxToRem(12)} />
          S3 Low
        </Stack>
      ),
    },
  ];

  const formik = useFormik<CreateCaseFormValues>({
    initialValues: {
      project: 0,
      product: 0,
      deployment: 0,
      title: "",
      description: "",
      type: 0,
      severity: 0,
    },
    onSubmit: () => {
      navigate("/support");
    },
  });

  // TODO: Remove this temporary auto-fill once backend integration is completed.
  useEffect(() => {
    formik.setFieldValue("title", "API Gateway timeout issues in production");
    formik.setFieldValue(
      "description",
      "Novera: Hi! I'm Novera, your AI-powered support assistant. How can I help you today? Please describe the issue you're experiencing. Customer: fadfad Novera: Thanks for those details. Based on what you've shared, here are a few things to check:",
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <form onSubmit={formik.handleSubmit}>
      <Stack pb={5} gap={5}>
        <Stack gap={2}>
          <SelectField
            name="project"
            label="Project"
            aiLabel="Auto Detected"
            options={projects}
            value={formik.values.project}
            onChange={formik.handleChange}
            startAdornment={
              <InputAdornment position="start">
                <Folder size={pxToRem(20)} />
              </InputAdornment>
            }
            required
          />
          <SelectField
            name="product"
            label="Product & Version"
            aiLabel="Auto Detected"
            options={products}
            value={formik.values.product}
            onChange={formik.handleChange}
            required
          />
          <SelectField
            name="deployment"
            label="Deployment Type"
            aiLabel="Auto Detected"
            options={deploymentTypes}
            value={formik.values.deployment}
            onChange={formik.handleChange}
            required
          />
        </Stack>
        <Stack gap={2}>
          <Typography variant="body1" fontWeight="medium">
            Case Details
          </Typography>
          <TextField
            name="title"
            label="Issue Title"
            aiLabel="Generated from Chat"
            value={formik.values.title}
            onChange={formik.handleChange}
            required
          />
          <TextField
            multiline
            name="description"
            label="Case Description"
            aiLabel="From Coversation"
            value={formik.values.description}
            onChange={formik.handleChange}
            required
          />
          <SelectField
            name="type"
            label="Issue Type"
            aiLabel="AI Classified"
            options={issueTypes}
            value={formik.values.type}
            onChange={formik.handleChange}
            required
          />
          <SelectField
            name="severity"
            label="Severity Levels"
            aiLabel="AI Accessed"
            options={severityLevels}
            value={formik.values.severity}
            onChange={formik.handleChange}
            required
          />
        </Stack>
        <ConversationSummary messages={messages} />
        <Button type="submit" variant="contained" sx={{ textTransform: "initial" }}>
          Create Case
        </Button>
      </Stack>
    </form>
  );
}
