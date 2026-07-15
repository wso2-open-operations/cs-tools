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

// Mirrors backend openapi.yaml's TaskSlaView / SearchTaskSlasPayload / SearchTaskSlasResponse.
// A case is a "task" upstream — SLAs are searched by task (case) id, ServiceNow data source only.

export interface TaskSlaDefinitionRefDto {
  id: string | null;
  name: string | null;
  type: string | null;
}

export interface SearchTaskSlasPayloadDto {
  filters?: {
    taskIds?: string[];
  };
  pagination?: {
    offset?: number;
    limit?: number;
  };
}

export interface TaskSlaViewDto {
  id: string;
  slaDefinition: TaskSlaDefinitionRefDto | null;
  stage: string | null;
  businessTimeLeft: string | null;
  businessElapsedTime: string | null;
  businessElapsedPercentage: number | null;
  startTime: string | null;
  endTime: string | null;
}

export interface SearchTaskSlasResponseDto {
  slas: TaskSlaViewDto[];
  total: number;
  limit: number;
  offset: number;
}
