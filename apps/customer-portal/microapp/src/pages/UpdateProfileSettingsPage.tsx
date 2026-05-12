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
  alpha,
} from "@wso2/oxygen-ui";
import { Link, useNavigate } from "react-router-dom";
import { SectionCard } from "@components/shared";
import { Clock, Phone } from "@wso2/oxygen-ui-icons-react";
import { useMutation, useQueryClient, useSuspenseQueries } from "@tanstack/react-query";
import { users } from "../services/users";
import { useFormik } from "formik";
import { useNotify } from "../context/snackbar";
import * as Yup from "yup";
import { metadata } from "../services/metadata";
import type { EditMeDto } from "../types";

type UpdateProfileFormValues = {
  phoneNumber: string;
  timeZone: string;
};

export default function UpdateProfileSettingsPage() {
  const notify = useNotify();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [{ data: me }, { data: meta }] = useSuspenseQueries({
    queries: [users.me(), metadata.get()],
  });

  const mutation = useMutation({
    ...users.editMe(),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: users.me().queryKey });

      const updatedMe = queryClient.getQueryData(users.me().queryKey);

      const normalizedTimezone = updatedMe?.timezone === "--None--" ? "" : (updatedMe?.timezone ?? "");
      const normalizedPhone = updatedMe?.phoneNumber ?? "";

      const isUpdated = normalizedPhone === formik.values.phoneNumber && normalizedTimezone === formik.values.timeZone;

      if (isUpdated) navigate("/profile");
      else notify.error("Failed to update profile. Please try again.");
    },
    onError: () => {
      notify.error("Failed to update profile. Please try again.");
    },
  });

  const formik = useFormik<UpdateProfileFormValues>({
    initialValues: {
      phoneNumber: me.phoneNumber ?? "",
      timeZone: me.timezone !== "--None--" ? me.timezone : "",
    },
    validationSchema: updatetProfileValidationSchema,
    validateOnBlur: true,
    validateOnChange: true,
    onSubmit: async (values, { setSubmitting }) => {
      try {
        const payload: Partial<EditMeDto> = {};

        if (values.phoneNumber !== me.phoneNumber) {
          payload.phoneNumber = values.phoneNumber;
        }

        if (values.timeZone !== me.timezone) {
          payload.timeZone = values.timeZone;
        }

        await mutation.mutateAsync(payload);
      } finally {
        setSubmitting(false);
      }
    },
  });

  return (
    <form onSubmit={formik.handleSubmit}>
      <Stack gap={2}>
        <Card
          component={Stack}
          direction="row"
          gap={2}
          sx={(theme) => ({ bgcolor: alpha(theme.palette.info.main, 0.2), p: 1.5 })}
        >
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
                  {meta.timeZones.map((item) => (
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
