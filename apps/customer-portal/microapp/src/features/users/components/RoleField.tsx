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
import { Checkbox, Stack, Typography } from "@wso2/oxygen-ui";

import type { Role } from "@features/users/types";

import { DEFAULT_USER_ROLE, ROLE_OPTIONS, ROLES } from "@shared/constants";

interface RoleFieldProps {
  value: Role[];
  onChange: (value: Role[]) => void;
}

export function RoleField({ value, onChange }: RoleFieldProps) {
  const handleSelection = (role: Role) => {
    if (role === ROLES.SYSTEM_USER) {
      // If SYSTEM_USER is being selected, it becomes the only active role.
      // If it's already selected, deselect it and set the default role as the current value.
      onChange(value.includes(role) ? [DEFAULT_USER_ROLE] : [ROLES.SYSTEM_USER]);
    } else {
      // If any other role is selected, strip out SYSTEM_USER and toggle the chosen role.
      const current = value.filter((r) => r !== ROLES.SYSTEM_USER);
      const updated = current.includes(role) ? current.filter((r) => r !== role) : [...current, role];
      onChange(updated);
    }
  };

  return (
    <Stack gap={1}>
      {ROLE_OPTIONS.map(({ role, description }) => (
        <RoleOption
          key={role}
          role={role}
          description={description}
          checked={value.includes(role)}
          onChange={() => handleSelection(role)}
        />
      ))}
    </Stack>
  );
}

export interface RoleOptionProps {
  role: Role;
  description: string;
  checked?: boolean;
  onChange?: () => void;
}

export function RoleOption({ role, description, checked = false, onChange }: RoleOptionProps) {
  return (
    <Stack direction="row" alignItems="start" gap={1}>
      <Checkbox value={role} checked={checked} onChange={onChange} />
      <Stack>
        <Typography variant="body2">{role}</Typography>
        <Typography variant="caption" color="text.secondary" sx={{ opacity: 0.8 }}>
          {description}
        </Typography>
      </Stack>
    </Stack>
  );
}
