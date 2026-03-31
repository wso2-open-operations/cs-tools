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

import { colors } from "@wso2/oxygen-ui";
import type {
  UsageAggregatedMetricDefinition,
  UsageEnvironmentBreakdownRow,
  UsageEnvironmentKind,
  UsageEnvironmentProduct,
  UsageOverviewSummary,
  UsageProductInstanceRow,
} from "@models/usageMetrics.types";

const ORANGE_STROKE = colors.orange?.[600] ?? "#EA580C";
const VIOLET_STROKE = colors.purple?.[500] ?? "#8B5CF6";
const GREEN_STROKE = colors.green?.[500] ?? "#22C55E";
const CYAN_STROKE = colors.cyan?.[500] ?? "#06B6D4";
const AMBER_STROKE = colors.amber?.[500] ?? "#F59E0B";
const INDIGO_STROKE = colors.indigo?.[600] ?? "#4F46E5";
const EMERALD_STROKE = colors.green?.[600] ?? "#10B981";

export const USAGE_OVERVIEW_SUMMARY: UsageOverviewSummary = {
  environments: 3,
  products: 8,
  instances: 24,
};

export const USAGE_ENVIRONMENT_BREAKDOWN: UsageEnvironmentBreakdownRow[] = [
  {
    id: "env-production",
    kind: "production",
    title: "Production",
    subtitle: "3 products • 12 instances",
    totalCores: 96,
    transactionsLabel: "35.4M",
    products: [
      {
        id: "ov-prod-apim",
        name: "API Manager",
        version: "Version 4.2.0",
        instances: 6,
        cores: 48,
        transactionsLabel: "28.9M",
      },
      {
        id: "ov-prod-is",
        name: "Identity Server",
        version: "Version 6.1.0",
        instances: 4,
        cores: 32,
        transactionsLabel: "6.2M",
      },
      {
        id: "ov-prod-mi",
        name: "Micro Integrator",
        version: "Version 4.2.0",
        instances: 2,
        cores: 16,
        transactionsLabel: "239.7K",
      },
    ],
  },
  {
    id: "env-test",
    kind: "test",
    title: "Test",
    subtitle: "3 products • 8 instances",
    totalCores: 48,
    transactionsLabel: "8.4M",
    products: [
      {
        id: "ov-test-apim",
        name: "API Manager",
        version: "Version 4.2.0",
        instances: 4,
        cores: 24,
        transactionsLabel: "5.1M",
      },
      {
        id: "ov-test-is",
        name: "Identity Server",
        version: "Version 6.1.0",
        instances: 3,
        cores: 16,
        transactionsLabel: "2.8M",
      },
      {
        id: "ov-test-mi",
        name: "Micro Integrator",
        version: "Version 4.2.0",
        instances: 1,
        cores: 8,
        transactionsLabel: "512K",
      },
    ],
  },
  {
    id: "env-development",
    kind: "development",
    title: "Development",
    subtitle: "2 products • 4 instances",
    totalCores: 12,
    transactionsLabel: "1.9M",
    products: [
      {
        id: "ov-dev-apim",
        name: "API Manager",
        version: "Version 4.2.0",
        instances: 2,
        cores: 6,
        transactionsLabel: "1.2M",
      },
      {
        id: "ov-dev-is",
        name: "Identity Server",
        version: "Version 6.1.0",
        instances: 2,
        cores: 6,
        transactionsLabel: "720K",
      },
    ],
  },
];

export const USAGE_AGGREGATED_METRICS: UsageAggregatedMetricDefinition[] = [
  {
    id: "total-transactions",
    title: "Total Transactions",
    caption: "Across all environments and products",
    headlineValue: "45.7M",
    deltaLabel: "+8.2%",
    stroke: ORANGE_STROKE,
    data: [
      { name: "Jan 26", value: 12.2 },
      { name: "Feb 26", value: 13.8 },
      { name: "Mar 26", value: 15.2 },
    ],
  },
  {
    id: "total-cores",
    title: "Total Core Count",
    caption: "Combined cores across environments",
    headlineValue: "156",
    deltaLabel: "+4.0%",
    stroke: VIOLET_STROKE,
    data: [
      { name: "Jan 26", value: 148 },
      { name: "Feb 26", value: 152 },
      { name: "Mar 26", value: 156 },
    ],
  },
  {
    id: "total-apis",
    title: "Total API Count",
    caption: "APIs deployed across all instances",
    headlineValue: "847",
    deltaLabel: "+12.5%",
    stroke: GREEN_STROKE,
    data: [
      { name: "Jan 26", value: 720 },
      { name: "Feb 26", value: 780 },
      { name: "Mar 26", value: 847 },
    ],
  },
  {
    id: "total-users",
    title: "Total User Count",
    caption: "Active users across all systems",
    headlineValue: "24.5K",
    deltaLabel: "+6.8%",
    stroke: CYAN_STROKE,
    data: [
      { name: "Jan 26", value: 21.2 },
      { name: "Feb 26", value: 22.9 },
      { name: "Mar 26", value: 24.5 },
    ],
  },
  {
    id: "total-orgs",
    title: "Total Organization Count",
    caption: "Organizations across all systems",
    headlineValue: "37",
    deltaLabel: "+0.0%",
    stroke: AMBER_STROKE,
    data: [
      { name: "Jan 26", value: 37 },
      { name: "Feb 26", value: 37 },
      { name: "Mar 26", value: 37 },
    ],
  },
];

function instanceChartBlocks(
  index: number,
): UsageProductInstanceRow["charts"] {
  const txBase = 280 + index * 25;
  const deltaNeg = index === 0;
  return {
    transactions: {
      title: "Instance Transactions",
      caption: "Requests processed by this instance",
      headlineValue: `${(4.8 - index * 0.12).toFixed(1)}M`,
      deltaLabel: deltaNeg ? "-1.4%" : "+0.8%",
      deltaPositive: !deltaNeg,
      stroke: INDIGO_STROKE,
      data: [
        { name: "Jan 26", value: txBase },
        { name: "Feb 26", value: txBase + 20 },
        { name: "Mar 26", value: txBase + 45 },
      ],
    },
    cores: {
      title: "Instance Core Count",
      caption: "CPU cores for this instance",
      headlineValue: "8",
      deltaLabel: "+0.0%",
      deltaPositive: true,
      stroke: EMERALD_STROKE,
      data: [
        { name: "Jan 26", value: 8 },
        { name: "Feb 26", value: 8 },
        { name: "Mar 26", value: 8 },
      ],
    },
  };
}

function makeInstances(prefix: string): UsageProductInstanceRow[] {
  const rows: Array<{
    idSuffix: string;
    hostSuffix: string;
    javaPatch: string;
    u2Patch: string;
    tx: string;
  }> = [
    {
      idSuffix: "1",
      hostSuffix: "01",
      javaPatch: "18",
      u2Patch: "0",
      tx: "4,824,270",
    },
    {
      idSuffix: "2",
      hostSuffix: "02",
      javaPatch: "19",
      u2Patch: "1",
      tx: "4,120,100",
    },
    {
      idSuffix: "3",
      hostSuffix: "03",
      javaPatch: "20",
      u2Patch: "2",
      tx: "3,980,400",
    },
  ];

  return rows.map((row, i) => ({
    id: `${prefix}-${row.idSuffix}`,
    hostName: `${prefix}-${row.hostSuffix}`,
    javaVersion: `11.0.${row.javaPatch}`,
    u2Level: `U2-4.2.0.${row.u2Patch}`,
    transactionsLabel: row.tx,
    coreCount: 8,
    charts: instanceChartBlocks(i),
  }));
}

const TX_TREND = [
  { name: "Jan 26", value: 1.8 },
  { name: "Feb 26", value: 2.0 },
  { name: "Mar 26", value: 2.2 },
];

const CORE_TREND = [
  { name: "Jan 26", current: 46, average: 50 },
  { name: "Feb 26", current: 47, average: 50 },
  { name: "Mar 26", current: 48, average: 50 },
];

export const USAGE_ENVIRONMENT_PRODUCTS: Record<
  UsageEnvironmentKind,
  UsageEnvironmentProduct[]
> = {
  production: [
    {
      id: "prod-apim",
      name: "API Manager",
      version: "Version 4.2.0",
      runningInstances: 6,
      transactionsLabel: "28.9M",
      thirdMetricLabel: "API Count",
      thirdMetricValue: "150",
      coreMetrics: [
        { label: "Current", value: "48" },
        { label: "Average", value: "50" },
        { label: "Minimum", value: "45" },
        { label: "Maximum", value: "55" },
      ],
      transactionTrend: TX_TREND,
      coreUsageTrend: CORE_TREND,
      instances: makeInstances("apimanager-prod"),
    },
    {
      id: "prod-is",
      name: "Identity Server",
      version: "Version 6.1.0",
      runningInstances: 4,
      transactionsLabel: "6.2M",
      thirdMetricLabel: "Organization Count",
      thirdMetricValue: "3",
      coreMetrics: [
        { label: "Current", value: "32" },
        { label: "Average", value: "35" },
        { label: "Minimum", value: "30" },
        { label: "Maximum", value: "40" },
      ],
      transactionTrend: [
        { name: "Jan 26", value: 1.9 },
        { name: "Feb 26", value: 2.1 },
        { name: "Mar 26", value: 2.0 },
      ],
      coreUsageTrend: [
        { name: "Jan 26", current: 30, average: 34 },
        { name: "Feb 26", current: 31, average: 34 },
        { name: "Mar 26", current: 32, average: 34 },
      ],
      instances: makeInstances("identity-prod"),
    },
    {
      id: "prod-mi",
      name: "Micro Integrator",
      version: "Version 4.2.0",
      runningInstances: 2,
      transactionsLabel: "239.7K",
      thirdMetricLabel: "API Count",
      thirdMetricValue: "50",
      coreMetrics: [
        { label: "Current", value: "16" },
        { label: "Average", value: "18" },
        { label: "Minimum", value: "15" },
        { label: "Maximum", value: "20" },
      ],
      transactionTrend: [
        { name: "Jan 26", value: 0.2 },
        { name: "Feb 26", value: 0.22 },
        { name: "Mar 26", value: 0.24 },
      ],
      coreUsageTrend: [
        { name: "Jan 26", current: 15, average: 17 },
        { name: "Feb 26", current: 16, average: 17 },
        { name: "Mar 26", current: 16, average: 17 },
      ],
      instances: makeInstances("mi-prod"),
    },
  ],
  test: [
    {
      id: "test-apim",
      name: "API Manager",
      version: "Version 4.2.0",
      runningInstances: 4,
      transactionsLabel: "5.1M",
      thirdMetricLabel: "API Count",
      thirdMetricValue: "82",
      coreMetrics: [
        { label: "Current", value: "24" },
        { label: "Average", value: "26" },
        { label: "Minimum", value: "22" },
        { label: "Maximum", value: "28" },
      ],
      transactionTrend: [
        { name: "Jan 26", value: 1.4 },
        { name: "Feb 26", value: 1.5 },
        { name: "Mar 26", value: 1.55 },
      ],
      coreUsageTrend: [
        { name: "Jan 26", current: 22, average: 25 },
        { name: "Feb 26", current: 23, average: 25 },
        { name: "Mar 26", current: 24, average: 25 },
      ],
      instances: makeInstances("apimanager-test"),
    },
    {
      id: "test-is",
      name: "Identity Server",
      version: "Version 6.1.0",
      runningInstances: 3,
      transactionsLabel: "2.8M",
      thirdMetricLabel: "Organization Count",
      thirdMetricValue: "2",
      coreMetrics: [
        { label: "Current", value: "18" },
        { label: "Average", value: "20" },
        { label: "Minimum", value: "16" },
        { label: "Maximum", value: "22" },
      ],
      transactionTrend: [
        { name: "Jan 26", value: 0.85 },
        { name: "Feb 26", value: 0.9 },
        { name: "Mar 26", value: 0.93 },
      ],
      coreUsageTrend: [
        { name: "Jan 26", current: 17, average: 19 },
        { name: "Feb 26", current: 18, average: 19 },
        { name: "Mar 26", current: 18, average: 19 },
      ],
      instances: makeInstances("identity-test"),
    },
    {
      id: "test-mi",
      name: "Micro Integrator",
      version: "Version 4.2.0",
      runningInstances: 1,
      transactionsLabel: "512K",
      thirdMetricLabel: "API Count",
      thirdMetricValue: "24",
      coreMetrics: [
        { label: "Current", value: "6" },
        { label: "Average", value: "8" },
        { label: "Minimum", value: "6" },
        { label: "Maximum", value: "9" },
      ],
      transactionTrend: [
        { name: "Jan 26", value: 0.14 },
        { name: "Feb 26", value: 0.15 },
        { name: "Mar 26", value: 0.16 },
      ],
      coreUsageTrend: [
        { name: "Jan 26", current: 6, average: 7 },
        { name: "Feb 26", current: 6, average: 7 },
        { name: "Mar 26", current: 6, average: 7 },
      ],
      instances: makeInstances("mi-test"),
    },
  ],
  development: [
    {
      id: "dev-apim",
      name: "API Manager",
      version: "Version 4.2.0",
      runningInstances: 2,
      transactionsLabel: "1.2M",
      thirdMetricLabel: "API Count",
      thirdMetricValue: "18",
      coreMetrics: [
        { label: "Current", value: "8" },
        { label: "Average", value: "9" },
        { label: "Minimum", value: "7" },
        { label: "Maximum", value: "10" },
      ],
      transactionTrend: [
        { name: "Jan 26", value: 0.35 },
        { name: "Feb 26", value: 0.38 },
        { name: "Mar 26", value: 0.4 },
      ],
      coreUsageTrend: [
        { name: "Jan 26", current: 7, average: 8 },
        { name: "Feb 26", current: 8, average: 8 },
        { name: "Mar 26", current: 8, average: 8 },
      ],
      instances: makeInstances("apimanager-dev"),
    },
    {
      id: "dev-is",
      name: "Identity Server",
      version: "Version 6.1.0",
      runningInstances: 2,
      transactionsLabel: "720K",
      thirdMetricLabel: "Organization Count",
      thirdMetricValue: "1",
      coreMetrics: [
        { label: "Current", value: "4" },
        { label: "Average", value: "5" },
        { label: "Minimum", value: "4" },
        { label: "Maximum", value: "6" },
      ],
      transactionTrend: [
        { name: "Jan 26", value: 0.2 },
        { name: "Feb 26", value: 0.22 },
        { name: "Mar 26", value: 0.24 },
      ],
      coreUsageTrend: [
        { name: "Jan 26", current: 4, average: 5 },
        { name: "Feb 26", current: 4, average: 5 },
        { name: "Mar 26", current: 4, average: 5 },
      ],
      instances: makeInstances("identity-dev"),
    },
  ],
};
