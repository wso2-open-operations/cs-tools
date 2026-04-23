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

import { Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import type { ItemCardProps } from "../support";
import { TYPE_CONFIG } from "../support/config";

export function OverlineSlot({ type, id }: { type: ItemCardProps["type"]; id: string }) {
  const { icon: Icon, color } = TYPE_CONFIG[type];

  return (
    <Stack direction="row" alignItems="center" gap={1}>
      <Icon color={color} size={pxToRem(18)} />
      <Typography variant="body2">{id}</Typography>
    </Stack>
  );
}
