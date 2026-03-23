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
  CircularProgress,
} from "@wso2/oxygen-ui";
import { Link, useNavigate } from "react-router-dom";
import { SectionCard } from "@components/shared";
import { Clock, Phone } from "@wso2/oxygen-ui-icons-react";
import { useMutation, useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "../services/cases";
import { useProject } from "../context/project";
import { users } from "../services/users";
import { useFormik } from "formik";
import { useNotify } from "../context/snackbar";
import * as Yup from "yup";

type UpdateProfileFormValues = {
  phoneNumber: string;
  timeZone: string;
};

export default function UpdateProfileSettingsPage() {
  const notify = useNotify();
  const navigate = useNavigate();
  const { projectId } = useProject();
  const { data: filters } = useSuspenseQuery(cases.filters(projectId!));
  const { data: me } = useSuspenseQuery(users.me());

  const mutation = useMutation({
    ...users.editMe(),
    onSuccess: () => {
      setTimeout(() => {
        navigate("/profile");
      }, 500);
    },
    onError: () => {
      notify.error("Failed to update profile. Please try again.");
    },
  });

  const formik = useFormik<UpdateProfileFormValues>({
    initialValues: {
      phoneNumber: "",
      timeZone: me.timezone !== "--None--" ? me.timezone : "",
    },
    validationSchema: updatetProfileValidationSchema,
    validateOnBlur: true,
    validateOnChange: true,
    onSubmit: (values) => {
      mutation.mutate({ phoneNumber: values.phoneNumber, timeZone: values.timeZone });
    },
  });

  return (
    <form onSubmit={formik.handleSubmit}>
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
                  name="phoneNumber"
                  size="small"
                  placeholder="+0 (00) 0000 0000"
                  value={formik.values.phoneNumber}
                  onChange={formik.handleChange}
                  error={formik.touched.phoneNumber && Boolean(formik.errors.phoneNumber)}
                  helperText={
                    formik.touched.phoneNumber && formik.errors.phoneNumber ? formik.errors.phoneNumber : undefined
                  }
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
                <Select
                  displayEmpty
                  name="timeZone"
                  size="small"
                  onChange={formik.handleChange}
                  value={formik.values.timeZone}
                  renderValue={(selected) => {
                    if (!selected) {
                      return (
                        <Typography variant="body2" color="text.secondary">
                          No Timezone Selected
                        </Typography>
                      );
                    }
                    return selected;
                  }}
                  error={formik.touched.timeZone && Boolean(formik.errors.timeZone)}
                >
                  {filters.timeZones.map((item) => (
                    <MenuItem value={item.label}>{item.label}</MenuItem>
                  ))}
                </Select>
                <FormHelperText>Select your preferred timezone</FormHelperText>
              </FormControl>
            </Stack>
          </Stack>
        </SectionCard>
        <Button
          type="submit"
          variant="contained"
          startIcon={formik.isSubmitting ? <CircularProgress size={16} color="inherit" /> : undefined}
        >
          {formik.isSubmitting ? "Saving..." : "Save Changes"}
        </Button>
        <Button
          variant="outlined"
          component={Link}
          sx={{ textTransform: "initial", bgcolor: "background.paper" }}
          to="/profile"
        >
          Cancel
        </Button>
      </Stack>
    </form>
  );
}

const updatetProfileValidationSchema = Yup.object({
  phoneNumber: Yup.string()
    .required("Phone number is required")
    .matches(/^\+[1-9]\d{1,14}$/, "Enter a valid phone number"),

  timeZone: Yup.string().required("Please select a timezone").notOneOf([""], "Please select a timezone"),
});
