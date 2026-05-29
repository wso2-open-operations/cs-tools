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
  ADD_PRODUCT_MODAL_INITIAL_FORM,
  parseValidNumber,
} from "@features/project-details/utils/addProductModal";

describe("addProductModal utils", () => {
  it("exports empty initial form state", () => {
    expect(ADD_PRODUCT_MODAL_INITIAL_FORM.productId).toBe("");
    expect(ADD_PRODUCT_MODAL_INITIAL_FORM.cores).toBe("");
  });

  it("parseValidNumber returns undefined for empty or invalid input", () => {
    expect(parseValidNumber("")).toBeUndefined();
    expect(parseValidNumber("  ")).toBeUndefined();
    expect(parseValidNumber("abc")).toBeUndefined();
  });

  it("parseValidNumber returns finite numbers", () => {
    expect(parseValidNumber("4")).toBe(4);
    expect(parseValidNumber(" 2.5 ")).toBe(2.5);
  });
});
