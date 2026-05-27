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
import * as Yup from "yup";
import { useQueryClient } from "@tanstack/react-query";
import { Button, CircularProgress, colors, Stack } from "@wso2/oxygen-ui";
import { Clock, Phone } from "@wso2/oxygen-ui-icons-react";
import { Form, FormikContext, useFormik } from "formik";

import { useDeclareLayout } from "@context/layout";
import { useNotify } from "@context/snackbar";

import { SelectField, TextField } from "@features/cases/components";
import { ProfileEditCallout } from "@features/profile/components";
import { useMe, useProfileMutations } from "@features/profile/hooks";
import { useMetadata } from "@features/profile/hooks";
import { users } from "@features/users/api/users.queries";
import type { EditMeDto } from "@features/users/types";

import { SectionCard } from "@shared/components/common";

import { Tab } from "@shared/constants";
import { useNavigation } from "@shared/hooks";

export default function ProfileEditPage() {
  useDeclareLayout({
    tabIndex: Tab.Profile,
    title: "Update Profile",
    slots: { subtitle: "Update your contact information" },
    visibility: { backAction: true },
  });

  const queryClient = useQueryClient();
  const notify = useNotify();
  const { back } = useNavigation();
  const { data: me, isPending: fetchingUser } = useMe();
  const { data: metadata, isPending: fetchingMetadata } = useMetadata();
  const { editMe: edit } = useProfileMutations();

  const { handleSubmit, ...formik } = useFormik<EditMeDto>({
    initialValues: {
      phoneNumber: me?.phoneNumber ?? "",
      timeZone: me?.timezone ?? "",
    },
    validateOnBlur: true,
    validateOnChange: true,
    validationSchema: validationSchema,
    enableReinitialize: true,
    onSubmit: async (values) => {
      const payload = Object.fromEntries(
        Object.entries(values).filter(([key, value]) => value !== formik.initialValues[key as keyof EditMeDto]),
      );

      const response = await edit.mutateAsync(payload);
      const isUpdated = Object.entries(payload).every(
        ([key, value]) => response[key as keyof typeof response] === value,
      );

      if (!isUpdated) {
        notify.error("Failed to update profile. Please try again.");
        return;
      }

      queryClient.invalidateQueries({ queryKey: users.me().queryKey });
      back(); // navigate back on success
    },
  });

  return (
    <FormikContext value={{ handleSubmit, ...formik }}>
      <Form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Stack gap={2}>
          <ProfileEditCallout />

          <SectionCard sx={{ gap: 4 }}>
            <TextField
              name="phoneNumber"
              label="Phone Number"
              placeholder="+0 (00) 0000 0000"
              helperText="Include country code for international numbers"
              slots={{ label: { startAdornment: <Phone size={16} color={colors.blue[500]} /> } }}
              disabled={fetchingUser}
            />

            <SelectField
              name="timeZone"
              label="Timezone"
              placeholder="No Timezone Selected"
              options={metadata?.timeZones.map((tz) => ({ value: tz.label, label: tz.label })) ?? []}
              helperText="Select your preferred timezone"
              slots={{ label: { startAdornment: <Clock size={16} color={colors.purple[500]} /> } }}
              disabled={fetchingUser || fetchingMetadata}
            />
          </SectionCard>

          <Button
            type="submit"
            variant="contained"
            startIcon={formik.isSubmitting ? <CircularProgress size={16} color="inherit" /> : undefined}
            disabled={!formik.dirty}
          >
            {formik.isSubmitting ? "Saving..." : "Save Changes"}
          </Button>

          <Button variant="outlined" sx={{ textTransform: "initial", bgcolor: "background.paper" }} onClick={back}>
            Cancel
          </Button>
        </Stack>
      </Form>
    </FormikContext>
  );
}

const validationSchema = Yup.object({
  phoneNumber: Yup.string()
    .required("Phone number is required")
    .matches(/^\+[1-9]\d{1,14}$/, "Enter a valid phone number"),

  timeZone: Yup.string().required("Please select a timezone").notOneOf([""], "Please select a timezone"),
});
