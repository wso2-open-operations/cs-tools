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
import { alpha, Chip, type ChipProps, Skeleton } from "@wso2/oxygen-ui";

import { CASE_TYPES } from "@shared/constants";
import { usePriorityChip, useStatusChip } from "@shared/hooks";
import type { CaseType } from "@shared/types";
import { overrideOrDefault } from "@shared/utils";

interface PriorityChipProps extends Omit<ChipProps, "label"> {
  prefix?: string;
  id?: string;
  type?: CaseType;
}

export function PriorityChip({ prefix, id, type = CASE_TYPES.DEFAULT, ...props }: PriorityChipProps) {
  const { label, color } = usePriorityChip(type, id) ?? {};

  if (!id || !label || !color) return <SkeletonChip />;

  return (
    <Chip
      label={`${prefix ? `${prefix}: ` : ""}${overrideOrDefault(label)}`}
      {...props}
      sx={
        color !== "default"
          ? (theme) => ({
              bgcolor: alpha(theme.palette[color].light, 0.1),
              color: theme.palette[color].light,
            })
          : undefined
      }
    />
  );
}

interface StatusChipProps extends Omit<ChipProps, "label"> {
  id?: string;
  type?: CaseType;
}

export function StatusChip({ id, type = CASE_TYPES.DEFAULT, ...props }: StatusChipProps) {
  const { label, color } = useStatusChip(type, id) ?? {};

  if (!id || !label || !color) return <SkeletonChip />;

  return (
    <Chip
      label={label}
      {...props}
      sx={
        color !== "default"
          ? (theme) => ({
              bgcolor: alpha(theme.palette[color].light, 0.1),
              color: theme.palette[color].light,
            })
          : undefined
      }
    />
  );
}

function SkeletonChip() {
  return <Skeleton variant="text" width={50} height={30} />;
}
