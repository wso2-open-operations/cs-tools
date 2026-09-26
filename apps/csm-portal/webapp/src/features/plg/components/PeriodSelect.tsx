import { MenuItem, TextField } from "@wso2/oxygen-ui";

import { RANGE_OPTIONS } from "@features/plg/components/period";
import { selectLabelProps } from "@features/plg/components/selectLabelProps";

/**
 * The period control shared by every analytics surface (dashboard, outreach,
 * funnel). It scopes cohort measures only — queue-style counts stay current
 * whatever is selected, so a backlog can never be hidden by changing a dropdown.
 */
export function PeriodSelect({ value, onChange }: { value: number; onChange: (days: number) => void }) {
  return (
    <TextField
      select
      {...selectLabelProps(value)}
      size="small"
      label="Period"
      value={value}
      onChange={(e) => onChange(Number(e.target.value))}
      sx={{ minWidth: 180 }}
    >
      {RANGE_OPTIONS.map((o) => (
        <MenuItem key={o.days} value={o.days}>
          {o.label}
        </MenuItem>
      ))}
    </TextField>
  );
}
