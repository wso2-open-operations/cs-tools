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

import { capitalize } from "@utils/others";
import { Chip, type ChipProps } from "@wso2/oxygen-ui";
import type { Priority, Status } from "./ItemCard";

import { PRIORITY_CHIP_COLOR_CONFIG, STATUS_CHIP_COLOR_CONFIG } from "./config";

interface PriorityChipProps extends Omit<ChipProps, "label"> {
  prefix?: string;
  priority: Priority;
}

export function PriorityChip({ prefix, priority, ...props }: PriorityChipProps) {
  const color = PRIORITY_CHIP_COLOR_CONFIG[priority];

  return <Chip color={color} label={`${prefix ? `${prefix}: ` : ""}${capitalize(priority)}`} {...props} />;
}

interface StatusChipProps extends Omit<ChipProps, "label"> {
  status: Status;
}

export function StatusChip({ status, ...props }: StatusChipProps) {
  const color = STATUS_CHIP_COLOR_CONFIG[status];

  return <Chip color={color} label={capitalize(status)} {...props} />;
}
