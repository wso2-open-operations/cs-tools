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
import { useEffect } from "react";

import { useNavigate } from "react-router-dom";

import { openUrl } from "@src/bridge";
import { useQueryClient } from "@tanstack/react-query";
import { colors, Stack, Switch, Typography } from "@wso2/oxygen-ui";
import { BookOpen, Bot, Clock4, Lock, Mail, Phone, User } from "@wso2/oxygen-ui-icons-react";

import { useDeclareLayout } from "@context/layout";

import { CHANGE_PASSWORD_URL } from "@config/endpoints";

import { metadata } from "@features/metadata/api/metadata.queries";
import { ProfileTitleSlot, SettingsItem, SettingsSection } from "@features/profile/components";
import { useAppVersion, useMe, useProfileMutations, useProject } from "@features/profile/hooks";

import { Tab } from "@shared/constants";

export default function ProfilePage() {
  useDeclareLayout({
    tabIndex: Tab.Profile,
    slots: {
      bottom: <ProfileTitleSlot />,
    },
  });

  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const version = useAppVersion();
  const { data: project } = useProject();
  const { data: me, isLoading } = useMe();
  const { editProject: edit } = useProfileMutations();

  useEffect(() => {
    queryClient.prefetchQuery(metadata.get());
  }, []);

  return (
    <Stack gap={2.5}>
      <SettingsSection title="Account Information">
        <SettingsItem label="Email" value={me?.email} slotProps={{ icon: { component: Mail } }} loading={isLoading} />

        <SettingsItem
          label="Phone"
          value={me?.phoneNumber ?? "Not Configured"}
          slotProps={{ icon: { component: Phone } }}
          loading={isLoading}
        />

        <SettingsItem
          label="Timezone"
          value={me?.timezone && me.timezone !== "--None--" ? me.timezone : "Not Configured"}
          slotProps={{ icon: { component: Clock4 } }}
          loading={isLoading}
        />
      </SettingsSection>

      <SettingsSection title="Settings">
        <SettingsItem
          label="Change Password"
          suffix="chevron"
          slotProps={{ icon: { component: Lock } }}
          onClick={() =>
            openUrl({
              url: CHANGE_PASSWORD_URL,
              presentationStyle: "FormSheet",
              dismissButtonStyle: "close",
            })
          }
        />

        <SettingsItem
          label="Update Profile"
          suffix="chevron"
          slotProps={{ icon: { component: User } }}
          onClick={() => navigate("/profile/update")}
        />
      </SettingsSection>

      <SettingsSection title="AI Features">
        <SettingsItem
          label="AI Chat Assistant"
          description="Enable AI-powered chat support"
          slotProps={{ icon: { component: Bot, sx: { color: colors.purple[500] } } }}
          suffix={
            <Switch
              checked={project?.agentEnabled}
              onChange={(event) => edit.mutate({ hasAgent: event.target.checked })}
              disabled={!me?.isAdmin}
            />
          }
        />

        <SettingsItem
          label="Smart Knowledge Base"
          description="Get intelligent article suggestions"
          slotProps={{ icon: { component: BookOpen, sx: { color: colors.blue[500] } } }}
          suffix={
            <Switch
              checked={project?.kbReferencesEnabled}
              onChange={(event) => edit.mutate({ hasKbReferences: event.target.checked })}
              disabled={!me?.isAdmin}
            />
          }
        />
      </SettingsSection>

      {version && (
        <Typography variant="caption" color="text.secondary" textAlign="center" sx={{ py: 1 }}>
          Version {version}
        </Typography>
      )}
    </Stack>
  );
}
