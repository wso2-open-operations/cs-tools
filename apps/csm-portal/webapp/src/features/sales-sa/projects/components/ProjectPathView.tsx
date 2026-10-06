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

import { Breadcrumbs, Link, Typography } from "@wso2/oxygen-ui";
import { HomeIcon } from "@wso2/oxygen-ui-icons-react";
import { useNavigate } from "react-router";

export default function ProjectPathView({
  accountName,
  accountNumber,
  projectKey,
}: {
  accountName?: string;
  accountNumber?: string;
  projectKey?: string;
}) {
  const navigate = useNavigate();

  return (
    <Breadcrumbs sx={{ mb: 1.5 }}>
      <Link
        component="button"
        underline="hover"
        color="inherit"
        onClick={() => navigate("/cases")}
        sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
      >
        <HomeIcon size={16} /> Home
      </Link>
      {accountNumber ? (
        <Link
          component="button"
          underline="hover"
          color="inherit"
          onClick={() => navigate(`/spl/accounts/${accountNumber}`)}
        >
          {accountName || "Account"}
        </Link>
      ) : (
        <Typography color="text.secondary">{accountName || "Account"}</Typography>
      )}
      {projectKey && <Typography color="text.primary">{projectKey}</Typography>}
    </Breadcrumbs>
  );
}
