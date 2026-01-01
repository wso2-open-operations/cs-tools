// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { ButtonBase as Button, Stack, Typography, InputAdornment } from "@mui/material";
import { Folder } from "@mui/icons-material";
import { Link, useLocation } from "react-router-dom";
import { SelectField, TextField, ConversationSummary } from "@components/features/create";

export default function CreateCasePage() {
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
    { value: 0, label: "S1 Critical" },
    { value: 1, label: "S2 Medium" },
    { value: 2, label: "S3 Low" },
  ];

  return (
    <Stack pb={5} gap={5}>
      <Stack gap={2}>
        <SelectField
          label="Project"
          options={projects}
          startAdornment={
            <InputAdornment position="start">
              <Folder />
            </InputAdornment>
          }
        />
        <SelectField label="Product & Version" options={products} />
        <SelectField label="Deployment Type" options={deploymentTypes} />
      </Stack>
      <Stack gap={2}>
        <Typography variant="body1" fontWeight="bold">
          Case Details
        </Typography>
        <TextField label="Issue Title" value="API Gateway timeout issues in production" />
        <TextField
          multiline
          label="Case Description"
          value="
            Novera: Hi! I'm Novera, your AI-powered support assistant. How can I help you today? Please describe the issue you're experiencing.
            Customer: fadfad
            Novera: Thanks for those details. Based on what you've shared, here are a few things to check:
          "
        />
        <SelectField label="Issue Type" options={issueTypes} />
        <SelectField label="Severity Levels" options={severityLevels} />
      </Stack>
      <ConversationSummary messages={messages} />
      <Button component={Link} to="/support" variant="contained" sx={{ fontWeight: "bold" }}>
        Create Case
      </Button>
    </Stack>
  );
}
