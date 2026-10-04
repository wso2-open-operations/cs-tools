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

// Routed at both /spl/projects/:projectId and
// /spl/accounts/:accountId/projects/:projectId (same two entry points the
// source app has) — only `projectId` is ever read; ProjectDetailView
// resolves the account context itself from the project record.
import { useParams } from "react-router";
import { safeRouteId } from "@features/spl/utils/routeId";
import ProjectDetailView from "@features/spl/projects/components/ProjectDetailView";

export default function ProjectDetailPage() {
  const { projectId } = useParams<{ projectId: string }>();
  const id = safeRouteId(projectId);
  return <ProjectDetailView id={id} />;
}
