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
import { isCloudSupportSubscription } from "./subscriptionType";

describe("isCloudSupportSubscription", () => {
  it.each(["cloud_support", "cloud_evaluation_support"] as const)(
    "is true for %s",
    (type) => {
      expect(isCloudSupportSubscription(type)).toBe(true);
    },
  );

  it("is false for other subscription types", () => {
    expect(isCloudSupportSubscription("managed_cloud_subscription")).toBe(false);
  });

  it("is false when the subscription type is unknown", () => {
    expect(isCloudSupportSubscription(undefined)).toBe(false);
    expect(isCloudSupportSubscription(null)).toBe(false);
  });
});
