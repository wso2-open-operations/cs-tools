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

import { describe, expect, it } from "vitest";
import {
  buildUsageProductInstanceAccordionKey,
  deriveUsageEnvironmentProducts,
} from "@features/usage-metrics/utils/usageMetricsEnvironmentProducts";

describe("usageMetricsEnvironmentProducts", () => {
  it("groups entries into environment products and builds accordion keys", () => {
    const products = deriveUsageEnvironmentProducts(
      [
        {
          instanceId: "i-1",
          instanceKey: "i-1",
          project: null,
          deployment: null,
          product: { id: "p-1", label: "Gateway" },
          deployedProduct: { id: "dp-1", label: "Gateway" },
          periodSummaries: [
            { period: "2026-01-01", counts: { TRANSACTION_COUNT: 20, API_COUNT: 3 } },
          ],
        },
      ],
      [],
    );

    expect(products).toHaveLength(1);
    expect(products[0].id).toBe("dp-1");
    expect(buildUsageProductInstanceAccordionKey("p-1", "i-1")).toBe("p-1::i-1");
  });
});

