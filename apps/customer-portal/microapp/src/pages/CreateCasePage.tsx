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
import { useLocation } from "react-router-dom";

import * as Yup from "yup";
import { Button, CircularProgress, InputAdornment, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { Folder } from "@wso2/oxygen-ui-icons-react";
import { Form, useFormik } from "formik";

import { useProject } from "@context/project";

import { ClassificationBadge, ConversationSummary, SelectField, TextField } from "@features/cases/components";
import { useAutoFill } from "@features/cases/hooks/useAutoFill";
import { useCreateCase } from "@features/cases/hooks/useCreateCase";
import { useCreateCaseData } from "@features/cases/hooks/useCreateCaseData";
import type { CaseClassificationResponseDto } from "@features/cases/types/case.dto";
import type { Case } from "@features/cases/types/case.model";

import { CaseReference } from "../features/cases/components/CaseReference";

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
  const location = useLocation();
  const messages = location.state?.messages || [];
  const classifications: CaseClassificationResponseDto = location.state?.classifications;
  const relatedCase: Case | undefined = location.state?.case;
  const { projectId } = useProject();
  const { mutation } = useCreateCase();

  const { values, isSubmitting, setFieldValue, handleSubmit } = useFormik<CreateCaseFormValues>({
    initialValues: {
      project: projectId!,
      product: "",
      deployment: "",
      title: "",
      description: "",
      type: "",
      severity: "",
    },
    validationSchema,
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

  const {
    projectsOptions,
    deploymentOptions,
    productOptions,
    issueTypeOptions,
    severityLevelOptions,
    deploymentsFieldDisabled,
    isLoadingDeployments,
    isLoadingProducts,
  } = useCreateCaseData(values.project, values.deployment);

  const { classified, setClassified } = useAutoFill({
    classifications,
    relatedCase,
    deploymentOptions,
    productOptions,
    issueTypeOptions,
    severityLevelOptions,
    deploymentsFieldDisabled,
    setFieldValue: setFieldValue,
  });

  return (
    <Form onSubmit={handleSubmit}>
      {relatedCase && <CaseReference data={relatedCase} />}

      <Stack pb={5} gap={5}>
        <Stack gap={2}>
          <SelectField
            required
            disabled
            name="project"
            label="Project"
            options={projectsOptions}
            slots={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Folder size={pxToRem(20)} />
                  </InputAdornment>
                ),
              },
            }}
          />

          <SelectField
            required
            name="deployment"
            label="Deployment Type"
            placeholder="Select Deployment Type"
            options={deploymentOptions}
            disabled={!!relatedCase || !values.project || isLoadingDeployments || deploymentsFieldDisabled}
            slots={{
              label: { endAdornment: classified.has("deployment") && <ClassificationBadge label="Auto Detected" /> },
            }}
            onChange={(e) => {
              setFieldValue("product", "");
              setClassified((prev) => {
                const next = new Set(prev);
                next.delete(e.target.name);
                next.delete("product");
                return next;
              });
            }}
          />

          <SelectField
            required
            name="product"
            label="Product & Version"
            placeholder="Select Product & Version"
            options={productOptions}
            disabled={!!relatedCase || !values.deployment || isLoadingProducts}
            slots={{
              label: { endAdornment: classified.has("product") && <ClassificationBadge label="Auto Detected" /> },
            }}
            onChange={(e) =>
              setClassified((prev) => {
                const next = new Set(prev);
                next.delete(e.target.name);
                return next;
              })
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
            slots={{
              label: { endAdornment: classified.has("title") && <ClassificationBadge label="Generated from Chat" /> },
            }}
            onChange={(e) =>
              setClassified((prev) => {
                const next = new Set(prev);
                next.delete(e.target.name);
                return next;
              })
            }
          />

          <TextField
            required
            multiline
            name="description"
            label="Case Description"
            placeholder="Explain the issue, including any relevant details"
            slots={{
              label: {
                endAdornment: classified.has("description") && <ClassificationBadge label="From Conversation" />,
              },
            }}
            onChange={(e) =>
              setClassified((prev) => {
                const next = new Set(prev);
                next.delete(e.target.name);
                return next;
              })
            }
          />

          <SelectField
            required
            name="type"
            label="Issue Type"
            placeholder="Select Issue Type"
            options={issueTypeOptions ?? []}
            slots={{ label: { endAdornment: classified.has("type") && <ClassificationBadge label="AI Classified" /> } }}
            onChange={(e) =>
              setClassified((prev) => {
                const next = new Set(prev);
                next.delete(e.target.name);
                return next;
              })
            }
          />

          <SelectField
            required
            name="severity"
            label="Severity Levels"
            placeholder="Select Severity"
            options={severityLevelOptions ?? []}
            slots={{
              label: { endAdornment: classified.has("severity") && <ClassificationBadge label="AI Classified" /> },
            }}
            onChange={(e) =>
              setClassified((prev) => {
                const next = new Set(prev);
                next.delete(e.target.name);
                return next;
              })
            }
          />
        </Stack>

        {messages.length > 0 && <ConversationSummary messages={messages} />}

        <Button
          type="submit"
          variant="contained"
          startIcon={isSubmitting || mutation.isPending ? <CircularProgress size={16} color="inherit" /> : undefined}
          sx={{ textTransform: "initial" }}
        >
          {isSubmitting ? "Saving..." : "Create Case"}
        </Button>
      </Stack>
    </Form>
  );
}

const validationSchema = Yup.object({
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
