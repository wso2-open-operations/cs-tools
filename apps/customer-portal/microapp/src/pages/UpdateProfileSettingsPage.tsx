import {
  Button,
  Stack,
  Typography,
  TextField,
  pxToRem,
  Card,
  colors,
  Select,
  MenuItem,
  FormHelperText,
  FormControl,
} from "@wso2/oxygen-ui";
import { Link } from "react-router-dom";
import { SectionCard } from "@components/shared";
import { Clock, Phone } from "@wso2/oxygen-ui-icons-react";

export default function UpdateProfileSettingsPage() {
  return (
    <Stack gap={2}>
      <Card component={Stack} direction="row" gap={2} sx={{ bgcolor: colors.blue[50], p: 1.5 }}>
        <Typography variant="subtitle2" color="text.secondary">
          Keep your contact information up to date for better communication with our support team.
        </Typography>
      </Card>
      <SectionCard>
        <Stack gap={3}>
          <Stack gap={1}>
            <Stack direction="row" gap={1}>
              <Phone size={pxToRem(16)} color={colors.blue[500]} />
              <Typography variant="subtitle2">Phone Number</Typography>
            </Stack>
            <FormControl>
              <TextField
                size="small"
                placeholder="+0 (00) 0000 0000"
                onChange={(event) => console.log(event.target.value)}
              />
              <FormHelperText>Include country code for international numbers</FormHelperText>
            </FormControl>
          </Stack>
          <Stack gap={1}>
            <Stack direction="row" gap={1}>
              <Clock size={pxToRem(16)} color={colors.purple[500]} />
              <Typography variant="subtitle2">Timezone</Typography>
            </Stack>
            <FormControl>
              <Select size="small" onChange={(event) => console.log(event.target.value)} value={1}>
                <MenuItem value={1}>Atlantic Standard Time (AST) UTC-4</MenuItem>
                <MenuItem value={2}>Atlantic Daylight Time (ADT) UTC-3</MenuItem>
              </Select>
              <FormHelperText>Select your preferred timezone</FormHelperText>
            </FormControl>
          </Stack>
        </Stack>
      </SectionCard>
      <Button variant="contained" component={Link} to="/users" sx={{ textTransform: "initial" }}>
        Save Changes
      </Button>
      <Button
        variant="outlined"
        component={Link}
        sx={{ textTransform: "initial", bgcolor: "background.paper" }}
        to="/users"
      >
        Cancel
      </Button>
    </Stack>
  );
}
