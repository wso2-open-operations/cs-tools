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
import { useQueries, useQuery } from "@tanstack/react-query";

import { useProject } from "@context/project";

import { buildEnvProducts } from "@features/chats/services/chatHistory.service";
import { projects } from "@features/projects/api/projects.queries";

export function useEnvProducts() {
  const { projectId } = useProject();
  const { data: deployments = [], isLoading: deploymentsLoading } = useQuery(projects.deployments(projectId!));

  const productQueries = useQueries({
    queries: deployments.map((deployment) => ({
      ...projects.products(deployment.id),
      enabled: !!deployment.id,
    })),
  });

  const productsLoading = productQueries.every((query) => query.isLoading);
  const envProducts = buildEnvProducts(deployments, productQueries as { data?: { name: string; version: string }[] }[]);

  return {
    envProducts,
    isLoading: deploymentsLoading || productsLoading,
  };
}
