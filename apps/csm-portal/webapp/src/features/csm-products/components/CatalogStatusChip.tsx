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


import { Chip } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { CatalogSupportStatus } from "@features/csm-products/types/productCatalog";
import {
  EMPTY_VALUE,
  SUPPORT_STATUS_COLOR,
  SUPPORT_STATUS_LABEL,
} from "@features/csm-products/utils/catalogFormat";

/** A version's support status as an outlined chip; a dash when unset. */
export default function CatalogStatusChip({ status }: { status: CatalogSupportStatus | null }): JSX.Element {
  if (!status) return <>{EMPTY_VALUE}</>;
  return (
    <Chip
      size="small"
      variant="outlined"
      color={SUPPORT_STATUS_COLOR[status]}
      label={SUPPORT_STATUS_LABEL[status] ?? status}
    />
  );
}
