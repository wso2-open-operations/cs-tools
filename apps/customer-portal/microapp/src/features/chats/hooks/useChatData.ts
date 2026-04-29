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

import { useEffect, useState } from "react";
import { useMutation, useQueries, useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { cases } from "@features/cases/api/cases.queries";
import { projects } from "@features/projects/api/projects.queries";
import { useProject } from "@context/project";
import { buildEnvProducts, messagesToString } from "@features/chats/services/chatHistory.service";
import type { ChatMessage } from "@features/chats/components";
import type { Product } from "@features/projects/types/project.model";

export function useChatData(messages: ChatMessage[]) {
  const navigate = useNavigate();
  const { projectId, projectTypeId } = useProject();

  const { data: deployments = [], isLoading: deploymentsLoading } = useQuery(projects.deployments(projectId!));

  const productQueries = useQueries({
    queries: deployments.map((deployment) => ({
      ...projects.products(deployment.id),
      enabled: !!deployment.id,
    })),
  });

  const productsLoading = productQueries.every((query) => query.isLoading);

  const envProducts = buildEnvProducts(
    deployments,
    productQueries as { data?: { name: string; version: string }[] }[],
  );

  const classifyMutation = useMutation({
    ...cases.classify,
    onSuccess: (response) => {
      setTimeout(() => {
        navigate("/create", { state: { messages, classifications: response } });
      }, 500);
    },
    onSettled: () => setIsAwaitingCreateCase(false),
  });

  const [isAwaitingCreateCase, setIsAwaitingCreateCase] = useState(false);

  useEffect(() => {
    if (!isAwaitingCreateCase) return;

    classifyMutation.mutate({
      projectTypeId,
      chatHistory: messagesToString(messages),
      envProducts: deployments.reduce(
        (acc, deployment, index) => {
          const products = (productQueries[index]?.data ?? []) as Pick<Product, "name">[];
          return { ...acc, [deployment.name]: products.map((p) => p.name) };
        },
        {} as Record<string, string[]>,
      ),
    });
  }, [isAwaitingCreateCase, deploymentsLoading, productsLoading]);

  const handleCreateCase = () => setIsAwaitingCreateCase(true);

  return {
    deployments,
    deploymentsLoading,
    productsLoading,
    envProducts,
    classifyMutation,
    isAwaitingCreateCase,
    handleCreateCase,
  };
}
