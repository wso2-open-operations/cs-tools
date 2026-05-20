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
import { Chip, Stack, Typography } from "@wso2/oxygen-ui";
import { useFormikContext } from "formik";

import { useProject } from "@context/project";

import type { EditUserFormValues } from "@pages/UserEditPage";

export function InvitationOverview() {
  const { projectId } = useProject();
  const { values } = useFormikContext<EditUserFormValues>();

  const summary = [
    { label: "Project", value: projectId },
    { label: "User Email", value: values.email || "-" },
    { label: "User Name", value: `${values.firstName} ${values.lastName}` || "-" },
    { label: "Role", value: <Chip size="small" label={values.roles[0]} /> },
    { label: "Delivery Method", value: "Email" },
  ];

  return (
    <Stack gap={1}>
      {summary.map((item) => (
        <Stack direction="row" justifyContent="space-between" gap={5}>
          <Typography variant="body2" color="text.secondary">
            {item.label}
          </Typography>
          <Typography component="span" variant="body2" textAlign="right">
            {item.value}
          </Typography>
        </Stack>
      ))}
    </Stack>
  );
}
