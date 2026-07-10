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

import type { DeploymentDto, DeploymentType, DeployedProductSearchItemDto } from "./deployment.dto";

export interface DeploymentOption {
  id: string;
  name: string;
  type: DeploymentType;
}

export function toDeploymentOption(dto: DeploymentDto): DeploymentOption {
  return { id: dto.id, name: dto.name, type: dto.type };
}

export interface DeployedProductOption {
  id: string;
  label: string;
}

export function toDeployedProductOption(dto: DeployedProductSearchItemDto): DeployedProductOption {
  const name = dto.product?.name || dto.product?.id || "Product";
  const version = dto.version?.name ?? "";
  return { id: dto.id, label: version ? `${name} ${version}` : name };
}
