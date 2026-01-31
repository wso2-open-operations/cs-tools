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

import { useParams } from "react-router";
import { type JSX } from "react";
import CasesOverviewStats from "@/components/support/CasesOverviewStats";
import { useGetProjectSupportStats } from "@/api/useGetProjectSupportStats";

/**
 * SupportPage component to display case details for a project.
 *
 * @returns {JSX.Element} The rendered Support page.
 */
export default function SupportPage(): JSX.Element {
  /**
   * Get the project ID from the URL.
   */
  const { projectId } = useParams<{ projectId: string }>();

  /**
   * Fetch support statistics for the project.
   */
  const { data: stats, isLoading } = useGetProjectSupportStats(projectId || "");

  return <CasesOverviewStats isLoading={isLoading} stats={stats} />;
}
