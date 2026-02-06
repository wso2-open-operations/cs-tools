import {
  FormControl,
  TextField as MuiTextField,
  MenuItem,
  Select,
  Stack,
  InputLabel,
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";

export function SelectField({
  name,
  label,
  options,
  value = 0,
  startAdornment,
  onChange,
}: {
  name: string;
  label: string;
  options: { value: number; label: string }[];
  value?: number;
  startAdornment?: React.ReactNode;
  onChange?: (event: SelectChangeEvent<number>) => void;
}) {
  return (
    <FormControl component={Stack} gap={1} fullWidth>
      <InputLabel>{label}</InputLabel>
      <Select
        name={name}
        label={label}
        value={value}
        sx={{ bgcolor: "background.paper" }}
        startAdornment={startAdornment}
        onChange={onChange}
      >
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
  name,
  label,
  value,
  multiline = false,
  rows = 10,
  placeholder,
  startAdornment,
  onChange,
}: {
  name: string;
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
      <MuiTextField
        name={name}
        label={label}
        value={value}
        placeholder={placeholder}
        multiline={multiline}
        rows={rows}
        sx={{ bgcolor: "background.paper", lineHeight: multiline ? 1.65 : undefined }}
        slotProps={{
          input: {
            startAdornment: startAdornment,
          },
        }}
        onChange={onChange}
      />
    </FormControl>
  );
}
