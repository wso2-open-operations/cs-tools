import { FormControl, InputBase, MenuItem, Select, Stack, Typography } from "@mui/material";

export function SelectField({
  label,
  options,
  value = 0,
  startAdornment,
}: {
  label: string;
  options: { value: number; label: string }[];
  value?: number;
  startAdornment?: React.ReactNode;
}) {
  return (
    <FormControl component={Stack} gap={1} fullWidth>
      <Typography component="label" variant="subtitle2" fontWeight="regular">
        {label}
      </Typography>
      <Select value={value} sx={{ bgcolor: "background.paper" }} startAdornment={startAdornment}>
        {options.map((option) => (
          <MenuItem key={option.value} value={option.value}>
            {option.label}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}

export function TextField({
  label,
  value,
  multiline = false,
  rows = 10,
  placeholder,
  startAdornment,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  multiline?: boolean;
  rows?: number;
  startAdornment?: React.ReactNode;

  onChange?: (event: React.ChangeEvent<HTMLInputElement>) => void;
}) {
  return (
    <FormControl component={Stack} gap={1} fullWidth>
      <Typography component="label" variant="subtitle2" fontWeight="regular">
        {label}
      </Typography>
      <InputBase
        value={value}
        placeholder={placeholder}
        multiline={multiline}
        rows={rows}
        sx={{ bgcolor: "background.paper", lineHeight: multiline ? 1.65 : undefined }}
        startAdornment={startAdornment}
        onChange={onChange}
      />
    </FormControl>
  );
}
