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

import type { CsmDeploymentRow } from "@features/csm-projects/types/csmProjects";

/**
 * Per-project deployment seeds. Each project usually has 2-3 deployments
 * (prod + staging + qa). The set is intentionally small so the UI demo
 * stays predictable; expand here as new projects are added to the
 * cases/projects seed.
 */
const DEPLOYMENTS_BY_PROJECT: Record<string, CsmDeploymentRow[]> = {
  "prj-acme-iam-prod": [
    {
      id: "dep-acme-iam-prod-east",
      projectId: "prj-acme-iam-prod",
      name: "Production US-East",
      environment: "prod",
      region: "us-east-1",
      lastUpdatedAt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Identity Server",
          version: "7.0.0",
          updateLevel: "U231",
          supportStatus: "available",
        },
      ],
    },
    {
      id: "dep-acme-iam-staging",
      projectId: "prj-acme-iam-prod",
      name: "Staging",
      environment: "staging",
      region: "us-east-1",
      lastUpdatedAt: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Identity Server",
          version: "7.1.0",
          updateLevel: "U042",
          supportStatus: "available",
        },
      ],
    },
  ],
  "prj-acme-openbanking": [
    {
      id: "dep-acme-ob-prod",
      projectId: "prj-acme-openbanking",
      name: "Open Banking Production",
      environment: "prod",
      region: "eu-west-1",
      lastUpdatedAt: new Date(Date.now() - 14 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Open Banking",
          version: "4.0.0",
          updateLevel: "U118",
          supportStatus: "available",
        },
        {
          product: "WSO2 API Manager",
          version: "4.3.0",
          updateLevel: "U92",
          supportStatus: "available",
        },
      ],
    },
  ],
  "prj-initech-apim": [
    {
      id: "dep-initech-apim-prod",
      projectId: "prj-initech-apim",
      name: "API Gateway Production",
      environment: "prod",
      region: "ap-southeast-1",
      lastUpdatedAt: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 API Manager",
          version: "4.3.0",
          updateLevel: "U92",
          supportStatus: "available",
        },
      ],
    },
    {
      id: "dep-initech-apim-qa",
      projectId: "prj-initech-apim",
      name: "QA",
      environment: "qa",
      region: "ap-southeast-1",
      lastUpdatedAt: new Date(Date.now() - 5 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 API Manager",
          version: "4.3.0",
          updateLevel: "U92",
          supportStatus: "available",
        },
      ],
    },
  ],
  "prj-initech-mi": [
    {
      id: "dep-initech-mi-prod",
      projectId: "prj-initech-mi",
      name: "MI Production",
      environment: "prod",
      region: "ap-southeast-1",
      lastUpdatedAt: new Date(Date.now() - 10 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Micro Integrator",
          version: "4.3.0",
          updateLevel: "U67",
          supportStatus: "available",
        },
      ],
    },
  ],
  "prj-initech-si": [
    {
      id: "dep-initech-si-prod",
      projectId: "prj-initech-si",
      name: "Streaming Production",
      environment: "prod",
      region: "ap-southeast-1",
      lastUpdatedAt: new Date(Date.now() - 20 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Streaming Integrator",
          version: "4.2.0",
          updateLevel: "U43",
          supportStatus: "extended",
        },
      ],
    },
  ],
  "prj-initech-choreo": [
    {
      id: "dep-initech-choreo-prod",
      projectId: "prj-initech-choreo",
      name: "Choreo Cloud (Prod)",
      environment: "prod",
      region: "us-east-1",
      lastUpdatedAt: new Date(Date.now() - 1 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Choreo",
          version: "Cloud",
          updateLevel: "rolling",
          supportStatus: "available",
        },
      ],
    },
  ],
  "prj-umbrella-choreo": [
    {
      id: "dep-umbrella-choreo-prod",
      projectId: "prj-umbrella-choreo",
      name: "Choreo Cloud (Prod)",
      environment: "prod",
      region: "us-east-1",
      lastUpdatedAt: new Date(Date.now() - 4 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Choreo",
          version: "Cloud",
          updateLevel: "rolling",
          supportStatus: "available",
        },
      ],
    },
  ],
  "prj-umbrella-asgardeo": [
    {
      id: "dep-umbrella-asgardeo-prod",
      projectId: "prj-umbrella-asgardeo",
      name: "Asgardeo Production",
      environment: "prod",
      region: "us-east-1",
      lastUpdatedAt: new Date(Date.now() - 6 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Asgardeo",
          version: "Cloud",
          updateLevel: "rolling",
          supportStatus: "available",
        },
      ],
    },
  ],
  "prj-globex-choreo": [
    {
      id: "dep-globex-choreo-prod",
      projectId: "prj-globex-choreo",
      name: "Choreo Production",
      environment: "prod",
      region: "eu-west-1",
      lastUpdatedAt: new Date(Date.now() - 8 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Choreo",
          version: "Cloud",
          updateLevel: "rolling",
          supportStatus: "available",
        },
      ],
    },
  ],
  "prj-globex-iam": [
    {
      id: "dep-globex-iam-prod",
      projectId: "prj-globex-iam",
      name: "IAM Production",
      environment: "prod",
      region: "eu-west-1",
      lastUpdatedAt: new Date(Date.now() - 12 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Identity Server",
          version: "6.1.0",
          updateLevel: "U195",
          supportStatus: "extended",
        },
      ],
    },
  ],
  "prj-soylent-apim": [
    {
      id: "dep-soylent-apim-prod",
      projectId: "prj-soylent-apim",
      name: "API Manager Production",
      environment: "prod",
      region: "us-west-2",
      lastUpdatedAt: new Date(Date.now() - 11 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 API Manager",
          version: "4.2.0",
          updateLevel: "U88",
          supportStatus: "available",
        },
      ],
    },
  ],
  "prj-soylent-iam": [
    {
      id: "dep-soylent-iam-onboard",
      projectId: "prj-soylent-iam",
      name: "Initial Onboarding",
      environment: "dev",
      region: "us-west-2",
      lastUpdatedAt: new Date(Date.now() - 0.5 * 24 * 60 * 60 * 1000).toISOString(),
      products: [
        {
          product: "WSO2 Identity Server",
          version: "7.0.0",
          updateLevel: "U231",
          supportStatus: "available",
        },
      ],
    },
  ],
};

/** Get the seeded deployments for a project. Empty when nothing seeded. */
export function getMockDeploymentsForProject(
  projectId: string,
): CsmDeploymentRow[] {
  return DEPLOYMENTS_BY_PROJECT[projectId] ?? [];
}
