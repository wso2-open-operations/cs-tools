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
import { Button, CircularProgress, colors } from "@wso2/oxygen-ui";
import { Phone } from "@wso2/oxygen-ui-icons-react";
import { Form, useFormik } from "formik";

import { SelectField, TextField } from "@features/cases/components";
import { ProfileEditCallout } from "@features/profile/components";
import { useMe, useProfileMutations } from "@features/profile/hooks";
import { useMetadata } from "@features/profile/hooks";
import type { EditMeDto } from "@features/users/types";

import { SectionCard } from "@shared/components/common";

import { useNavigation } from "@shared/hooks";

export default function ProfileEditPage() {
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
    onSubmit: async (values) => {
      const payload = Object.fromEntries(
        Object.entries(values).filter(([key, value]) => value !== formik.initialValues[key as keyof EditMeDto]),
      );
      await edit.mutateAsync(payload);
      back(); // navigate back on success
    },
  });

  return (
    <Form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 2 }}>
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
          slots={{ label: { startAdornment: <Phone size={16} color={colors.blue[500]} /> } }}
          disabled={fetchingUser || fetchingMetadata}
        />
      </SectionCard>

      <Button
        type="submit"
        variant="contained"
        startIcon={formik.isSubmitting ? <CircularProgress size={16} color="inherit" /> : undefined}
      >
        {formik.isSubmitting ? "Saving..." : "Save Changes"}
      </Button>

      <Button variant="outlined" sx={{ textTransform: "initial", bgcolor: "background.paper" }} onClick={back}>
        Cancel
      </Button>
    </Form>
  );
}

const validationSchema = Yup.object({
  phoneNumber: Yup.string()
    .required("Phone number is required")
    .matches(/^\+[1-9]\d{1,14}$/, "Enter a valid phone number"),

  timeZone: Yup.string().required("Please select a timezone").notOneOf([""], "Please select a timezone"),
});
