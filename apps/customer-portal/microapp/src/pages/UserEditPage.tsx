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
import { Button, CircularProgress, Stack, TextField } from "@wso2/oxygen-ui";
import { Form, FormikContext, useFormik } from "formik";

import { useDeclareLayout } from "@context/layout";

import {
  InvitationCallout,
  InvitationExpiryCallout,
  RoleField,
  UserDeleteActions,
  UserOverview,
} from "@features/users/components";
import { useMode, useUserMutations } from "@features/users/hooks";
import type { Role } from "@features/users/types";

import { SectionCard } from "@shared/components/common";

import { DEFAULT_USER_ROLE, ROLES, Tab, USER_EDIT_MODES } from "@shared/constants";
import { useNavigation } from "@shared/hooks";

export interface EditUserFormValues {
  email: string;
  firstName: string;
  lastName: string;
  roles: Role[];
}

export default function UserEditPage() {
  const { back } = useNavigation();
  const { mode, initial } = useMode();
  const { create, edit } = useUserMutations();
  const formik = useFormik<EditUserFormValues>({
    initialValues: {
      email: initial?.email ?? "",
      firstName: initial?.firstName ?? "",
      lastName: initial?.lastName ?? "",
      roles: initial?.roles ?? [DEFAULT_USER_ROLE],
    },

    onSubmit: () => {
      if (mode === USER_EDIT_MODES.INVITE) {
        create.mutate({
          contactEmail: values.email,
          contactFirstName: values.firstName,
          contactLastName: values.lastName,
          isCsAdmin: values.roles.includes(ROLES.ADMIN),
          isCsIntegrationUser: values.roles.includes(ROLES.SYSTEM_USER),
          isPortalUser: values.roles.includes(ROLES.PORTAL_USER),
          isSecurityContact: values.roles.includes(ROLES.SECURITY_CONTACT),
        });
      } else {
        edit!.mutate({ isSecurityContact: values.roles[0] === ROLES.SYSTEM_USER });
      }
    },
  });

  useDeclareLayout({
    tabIndex: Tab.Users,
    title: initial ? "Edit User" : "Invite New User",
    visibility: { backAction: true },
  });

  const { values, dirty, getFieldProps, setFieldValue } = formik;

  return (
    <FormikContext value={formik}>
      <Form>
        <Stack gap={2}>
          {mode === USER_EDIT_MODES.EDIT && <UserOverview />}
          {mode === USER_EDIT_MODES.INVITE && <InvitationCallout />}

          <SectionCard title="User Details">
            <Stack gap={2}>
              <TextField
                size="small"
                label="Email Address"
                {...getFieldProps("email")}
                helperText={mode === USER_EDIT_MODES.EDIT ? "Email cannot be edited" : undefined}
                slotProps={{
                  htmlInput: { readOnly: mode === USER_EDIT_MODES.EDIT },
                }}
              />

              <TextField
                size="small"
                label="First Name"
                {...getFieldProps("firstName")}
                helperText={mode === USER_EDIT_MODES.EDIT ? "First Name cannot be edited" : undefined}
                slotProps={{
                  htmlInput: { readOnly: mode === USER_EDIT_MODES.EDIT },
                }}
              />

              <TextField
                size="small"
                label="Last Name"
                {...getFieldProps("lastName")}
                helperText={mode === USER_EDIT_MODES.EDIT ? "Last Name cannot be edited" : undefined}
                slotProps={{
                  htmlInput: { readOnly: mode === USER_EDIT_MODES.EDIT },
                }}
              />
            </Stack>
          </SectionCard>

          <SectionCard title="User Role">
            <RoleField value={values.roles} onChange={(updated) => setFieldValue("roles", updated)} />
          </SectionCard>

          {mode === USER_EDIT_MODES.INVITE && <InvitationExpiryCallout />}

          {mode === USER_EDIT_MODES.EDIT && <UserDeleteActions />}

          {mode === USER_EDIT_MODES.INVITE && (
            <Button
              type="submit"
              variant="contained"
              startIcon={create.isPending && <CircularProgress size={16} color="inherit" />}
              disabled={!dirty || create.isPending}
            >
              {create.isPending ? "Sending..." : "Send Invitation"}
            </Button>
          )}

          {mode === USER_EDIT_MODES.EDIT && (
            <Button
              type="submit"
              variant="contained"
              startIcon={edit!.isPending && <CircularProgress size={16} color="inherit" />}
              disabled={!dirty || edit!.isPending}
            >
              {edit!.isPending ? "Saving..." : "Save Changes"}
            </Button>
          )}

          <Button variant="outlined" sx={{ textTransform: "initial", bgcolor: "background.paper" }} onClick={back}>
            Cancel
          </Button>
        </Stack>
      </Form>
    </FormikContext>
  );
}
