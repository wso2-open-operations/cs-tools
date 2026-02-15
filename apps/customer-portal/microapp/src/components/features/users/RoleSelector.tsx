import { Stack, Chip, Radio, RadioGroup, FormControlLabel, Box, pxToRem } from "@wso2/oxygen-ui";
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
          <Chip size="small" label={role} color={admin ? "primary" : "default"} />
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
