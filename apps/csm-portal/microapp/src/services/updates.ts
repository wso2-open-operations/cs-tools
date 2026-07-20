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

import { queryOptions } from "@tanstack/react-query";
import { PRODUCT_UPDATE_LEVELS_ENDPOINT, UPDATE_LEVELS_SEARCH_ENDPOINT } from "@config/endpoints";
import type {
  ProductUpdateLevelDto,
  SearchUpdatesInput,
  UpdateLevelGroup,
  UpdateLevelsSearchResponseDto,
} from "@src/types";
import { toUpdateLevelGroups, type ProductUpdateLevel } from "@src/types";
import apiClient from "./apiClient";

const getProductUpdateLevels = async (): Promise<ProductUpdateLevel[]> => {
  const { data } = await apiClient.get<ProductUpdateLevelDto[]>(PRODUCT_UPDATE_LEVELS_ENDPOINT);
  return data;
};

const searchUpdateLevels = async (input: SearchUpdatesInput): Promise<UpdateLevelGroup[]> => {
  const { data } = await apiClient.post<UpdateLevelsSearchResponseDto>(UPDATE_LEVELS_SEARCH_ENDPOINT, input);
  return toUpdateLevelGroups(data);
};

export const updates = {
  productLevels: () =>
    queryOptions({
      queryKey: ["updates", "product-update-levels"],
      queryFn: () => getProductUpdateLevels(),
      staleTime: 5 * 60_000,
    }),

  search: (input: SearchUpdatesInput | null) =>
    queryOptions({
      queryKey: ["updates", "levels-search", input],
      queryFn: () => searchUpdateLevels(input!),
      enabled: input !== null,
    }),
};
