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

import type { ProjectDto, ProjectSubscriptionType } from "./project.dto";

export interface Project {
  id: string;
  name: string;
  subscriptionType: ProjectSubscriptionType;
}

// Cloud-support subscriptions have exactly one (primary_production) deployment, so the webapp's
// case-create form hides the deployment picker and auto-selects it for these subscription types
// (apps/csm-portal/webapp/src/features/csm-cases/pages/CsmCaseCreatePage.tsx).
const CLOUD_SUBSCRIPTION_TYPES: ProjectSubscriptionType[] = ["cloud_support", "cloud_evaluation_support"];

export function isCloudSupportProject(project: Project): boolean {
  return CLOUD_SUBSCRIPTION_TYPES.includes(project.subscriptionType);
}

export function toProject(dto: ProjectDto): Project {
  return { id: dto.id, name: dto.name, subscriptionType: dto.subscriptionType };
}
