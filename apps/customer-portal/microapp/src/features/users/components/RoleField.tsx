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
import { Chip, FormControlLabel, Radio, RadioGroup, Stack, useRadioGroup } from "@wso2/oxygen-ui";

import type { Role } from "@features/users/types";

import { ROLES } from "@shared/constants";

interface RoleFieldProps {
  value: Role[];
  onChange: (value: Role[]) => void;
}

export function RoleField({ value, onChange }: RoleFieldProps) {
  return (
    <RadioGroup value={value ? value[0] : undefined} onChange={(event) => onChange([event.target.value as Role])}>
      <Stack gap={0.5}>
        <RoleOption>{ROLES.PORTAL_USER}</RoleOption>
        <RoleOption>{ROLES.SYSTEM_USER}</RoleOption>
      </Stack>
    </RadioGroup>
  );
}

export function RoleOption({ children: role }: { children: string }) {
  const radioGroup = useRadioGroup();
  const checked = radioGroup?.value === role;

  return (
    <FormControlLabel
      value={role}
      control={<Radio />}
      labelPlacement="start"
      label={<Chip size="small" label={role} color={checked ? "primary" : "default"} />}
      sx={{
        m: 0,
        justifyContent: "space-between",
      }}
    />
  );
}
