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
import { Button, CircularProgress, InputAdornment, Stack, Typography } from "@wso2/oxygen-ui";
import { Folder } from "@wso2/oxygen-ui-icons-react";
import { Form, useFormikContext } from "formik";

import { useProject } from "@context/project";

import {
  AttachmentField,
  CaseReference,
  ClassificationBadge,
  ConversationSummary,
  SelectField,
  TextField,
} from "@features/case-types/cases/components";
import { useClassification } from "@features/case-types/cases/context";
import {
  type CreateCaseFormValues,
  useAutoFill,
  useCreateCase,
  useCreateCaseFormOptions,
} from "@features/case-types/cases/hooks";

import { DEPLOYMENT_DISABLED_PROJECT_TYPES } from "@shared/constants";

export function CreateCaseForm() {
  useAutoFill();

  const { state } = useCreateCase();
  const { projectName } = useProject();
  const { deployments, products, issueTypes, severities } = useCreateCaseFormOptions();
  const { values, setFieldValue, ...formik } = useFormikContext<CreateCaseFormValues>();
  const { classified, remove } = useClassification();

  return (
    <Form onSubmit={formik.handleSubmit}>
      {state.case && <CaseReference />}

      <Stack pb={5} gap={5}>
        <Stack gap={2}>
          <TextField
            required
            disabled
            name="project"
            label="Project"
            value={projectName || "No project name available"}
            slots={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Folder size={20} />
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
            options={deployments.options}
            disabled={!!state.case || deployments.pending || DEPLOYMENT_DISABLED_PROJECT_TYPES.includes(values.project)}
            slots={{
              label: { endAdornment: classified.has("deployment") && <ClassificationBadge label="Auto Detected" /> },
            }}
            onChange={(e) => {
              setFieldValue("product", "");
              remove([e.target.name, "product"]);
            }}
          />

          <SelectField
            required
            name="product"
            label="Product & Version"
            placeholder="Select Product & Version"
            options={products.options}
            disabled={!!state.case || !values.deployment || products.pending}
            slots={{
              label: { endAdornment: classified.has("product") && <ClassificationBadge label="Auto Detected" /> },
            }}
            onChange={(e) => remove([e.target.name])}
          />
        </Stack>

        <Stack gap={2}>
          <Typography variant="body1" fontWeight="medium">
            Case Details
          </Typography>

          {state.case && <TextField required disabled name="relatedCaseId" label="Related Case ID" />}

          <TextField
            required
            disabled={!!state.case}
            name="title"
            label="Issue Title"
            placeholder="Briefly describe the issue"
            slots={{
              label: { endAdornment: classified.has("title") && <ClassificationBadge label="Generated from Chat" /> },
            }}
            onChange={(e) => remove([e.target.name])}
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
            onChange={(e) => remove([e.target.name])}
          />

          <SelectField
            required
            name="type"
            label="Issue Type"
            placeholder="Select Issue Type"
            options={issueTypes.options}
            slots={{ label: { endAdornment: classified.has("type") && <ClassificationBadge label="AI Classified" /> } }}
            onChange={(e) => remove([e.target.name])}
          />

          <SelectField
            required
            name="severity"
            label="Severity Levels"
            placeholder="Select Severity"
            options={severities.options}
            slots={{
              label: { endAdornment: classified.has("severity") && <ClassificationBadge label="AI Classified" /> },
            }}
            onChange={(e) => remove([e.target.name])}
          />

          <AttachmentField onChange={(attachments) => setFieldValue("attachments", attachments)} />
        </Stack>

        {state.messages.length > 0 && <ConversationSummary />}

        <Button
          type="submit"
          variant="contained"
          sx={{ textTransform: "initial" }}
          startIcon={formik.isSubmitting ? <CircularProgress size={16} color="inherit" /> : undefined}
          disabled={formik.isSubmitting}
        >
          {formik.isSubmitting ? "Creating..." : "Create Case"}
        </Button>
      </Stack>
    </Form>
  );
}
