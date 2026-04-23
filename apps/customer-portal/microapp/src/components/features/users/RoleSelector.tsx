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

import { Stack, Chip, Radio, RadioGroup, FormControlLabel, Box, pxToRem, useRadioGroup } from "@wso2/oxygen-ui";
import { ShieldUser } from "@wso2/oxygen-ui-icons-react";
import { MOCK_ROLES } from "@src/mocks/data/users";

export type RoleName = (typeof MOCK_ROLES)[number]["name"];

const ROLE_NAMES = MOCK_ROLES.map((role) => role.name);

interface RoleSelectorProps {
  value: RoleName;
  onChange: (value: RoleName) => void;
}

export function RoleSelector({ value, onChange }: RoleSelectorProps) {
  return (
    <RadioGroup value={value} onChange={(event) => onChange(event.target.value as RoleName)}>
      <Stack gap={0.5}>
        {ROLE_NAMES.map((role) => (
          <RoleOption key={role} role={role} />
        ))}
      </Stack>
    </RadioGroup>
  );
}

export function RoleOption({ role }: { role: RoleName }) {
  const radioGroup = useRadioGroup();
  const checked = radioGroup?.value === role;
  const admin = role === "Admin";

  return (
    <FormControlLabel
      value={role}
      control={<Radio />}
      labelPlacement="start"
      sx={{
        m: 0,
        justifyContent: "space-between",
      }}
      label={
        <Stack direction="row" alignItems="center" gap={1}>
          <Chip size="small" label={role} color={checked ? "primary" : "default"} />
          {admin && (
            <Box color="primary.main">
              <ShieldUser size={pxToRem(18)} />
            </Box>
          )}
        </Stack>
      }
    ></FormControlLabel>
  );
}
