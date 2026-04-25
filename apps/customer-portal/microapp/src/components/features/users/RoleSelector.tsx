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

import { useState } from "react";
import { Stack, Checkbox, FormControlLabel, Typography, Box, pxToRem } from "@wso2/oxygen-ui";
import { Code, Crown, Monitor, Shield } from "@wso2/oxygen-ui-icons-react";
import { MOCK_ROLES } from "@src/mocks/data/users";
import type { Role } from "@root/src/types";

export type RoleName = Role;

interface RoleSelectorProps {
  value: Role[];
  onChange: (value: Role[]) => void;
  readOnly?: boolean;
}

export function RoleSelector({ value, onChange, readOnly = false }: RoleSelectorProps) {
  const hasSystemUser = value.includes("System User");
  const [roleRequiredError, setRoleRequiredError] = useState(false);

  const toggleRole = (role: Role) => {
    if (readOnly) return;

    if (role === "System User") {
      setRoleRequiredError(false);
      onChange(hasSystemUser ? ["Portal User"] : ["System User"]);
      return;
    }

    if (hasSystemUser) return;

    if (value.includes(role)) {
      if (value.length === 1) {
        setRoleRequiredError(true);
        return;
      }

      setRoleRequiredError(false);
      const nextRoles = value.filter((existingRole) => existingRole !== role);
      onChange(nextRoles);
      return;
    }

    setRoleRequiredError(false);
    onChange([...value, role]);
  };

  return (
    <Stack gap={1}>
      {MOCK_ROLES.map((role) => (
        <RoleOption
          key={role.name}
          role={role.name as RoleName}
          description={role.description}
          selected={value.includes(role.name as Role)}
          disabled={readOnly || (hasSystemUser && role.name !== "System User")}
          onClick={toggleRole}
        />
      ))}

      {hasSystemUser && (
        <Box
          sx={{
            px: 1.5,
            py: 1,
            bgcolor: "warning.50",
          }}
        >
          <Typography variant="caption" color="warning.dark" fontWeight="medium">
            System Users are used for machine-to-machine integrations and cannot hold additional roles.
          </Typography>
        </Box>
      )}
      {roleRequiredError && (
        <Typography variant="caption" color="error.main" fontWeight="medium">
          At least one role is required.
        </Typography>
      )}
    </Stack>
  );
}

interface RoleOptionProps {
  role: RoleName;
  description: string;
  selected: boolean;
  disabled?: boolean;
  onClick: (role: RoleName) => void;
}

export function RoleOption({ role, description, selected, disabled = false, onClick }: RoleOptionProps) {
  const roleIcon =
    role === "Admin User" ? (
      <Crown size={pxToRem(18)} />
    ) : role === "Portal User" ? (
      <Monitor size={pxToRem(18)} />
    ) : role === "Security User" ? (
      <Shield size={pxToRem(18)} />
    ) : (
      <Code size={pxToRem(18)} />
    );

  return (
    <Box
      sx={{
        px: 1.5,
        py: 0.5,
        opacity: disabled ? 0.6 : 1,
      }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1.5}>
        <FormControlLabel
          sx={{ m: 0, width: "100%" }}
          disabled={disabled}
          control={<Checkbox checked={selected} onChange={() => onClick(role)} disabled={disabled} />}
          label={
            <Stack>
              <Typography variant="body2" fontWeight="medium">
                {role}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {description}
              </Typography>
            </Stack>
          }
        />
        <Box color="text.secondary">{roleIcon}</Box>
      </Stack>
    </Box>
  );
}
