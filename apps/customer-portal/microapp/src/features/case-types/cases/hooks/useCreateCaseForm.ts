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
import { useNavigate } from "react-router-dom";

import * as Yup from "yup";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useFormik } from "formik";

import { useProject } from "@context/project";

import { cases } from "@features/case-types/cases/api/cases.queries";
import type { AttachmentFile } from "@features/case-types/cases/components";
import { useCreateCase } from "@features/case-types/cases/hooks";

import { toBase64 } from "@shared/utils/attachments.utils";

export interface CreateCaseFormValues {
  project: string;
  product: string;
  deployment: string;
  type: string;
  severity: string;
  title: string;
  description: string;
  attachments: AttachmentFile[];
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

export function useCreateCaseForm() {
  const { projectId } = useProject();
  const { state, create } = useCreateCase();
  const createAttachmentMutation = useMutation(cases.createAttachment);
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const formik = useFormik<CreateCaseFormValues>({
    initialValues: {
      project: projectId!,
      product: "",
      deployment: "",
      title: "",
      description: "",
      type: "",
      severity: "",
      attachments: [],
    },
    validationSchema,
    validateOnBlur: true,
    validateOnChange: true,
    onSubmit: async (values) => {
      const response = await create.mutateAsync({
        type: "default_case",
        projectId: values.project,
        deploymentId: values.deployment,
        deployedProductId: values.product,
        title: values.title,
        description: values.description,
        issueTypeKey: Number(values.type),
        severityKey: Number(values.severity),
        relatedCaseId: state?.case?.id,
      });

      await Promise.all(
        values.attachments.map(async (attachment) => {
          const content = await toBase64(attachment.raw);
          await createAttachmentMutation.mutateAsync({
            caseId: response.id,
            type: attachment.type,
            name: attachment.name,
            content,
          });
        }),
      );

      queryClient.invalidateQueries({ queryKey: ["cases"] });
      setTimeout(() => {
        navigate(`/cases/${response.id}`);
      }, 500);
    },
  });

  return formik;
}
