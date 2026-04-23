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

// TODO: This page currently contains placeholder values and static content.
//       All displayed data (names, avatars, dates, etc.) will be replaced
//       with dynamic values in the future.

import { useLayoutEffect, type ReactNode } from "react";
import { Card, Divider, Stack, Switch, Typography, colors } from "@wso2/oxygen-ui";
import { Bell, BookOpen, Bot, Clock4, Lock, Mail, Phone, User } from "@wso2/oxygen-ui-icons-react";
import { useLayout } from "@context/layout";
import { SettingListItem } from "@components/features/settings";
import { Avatar } from "@components/features/users";

export default function ProfilePage() {
  const layout = useLayout();

  const AppBarSlot = () => (
    <Stack direction="row" alignItems="center" gap={1.5} mt={1}>
      <Avatar>John Smith</Avatar>
      <Stack>
        <Typography variant="h6" fontWeight="medium">
          John Smith
        </Typography>
        <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
          Customer since 2024
        </Typography>
      </Stack>
    </Stack>
  );

  useLayoutEffect(() => {
    layout.setAppBarSlotsOverride(<AppBarSlot />);

    return () => {
      layout.setAppBarSlotsOverride(undefined);
    };
  }, []);

  return (
    <Stack gap={2.5}>
      <SectionCard title="Account Information">
        <SettingListItem name="Email" value="user@example.com" icon={Mail} />
        <SettingListItem name="Phone" value="+1 (555) 123-4567" icon={Phone} />
        <SettingListItem name="Timezone" value="Eastern Time (ET) - UTC-5" icon={Clock4} />
      </SectionCard>

      <SectionCard title="Settings">
        <SettingListItem name="Change Password" suffix="chevron" icon={Lock} />
        <SettingListItem name="Update Profile" suffix="chevron" icon={User} />
      </SectionCard>

      <SectionCard title="Notifications">
        <SettingListItem name="Push Notifications" icon={Bell} suffix={<Switch defaultChecked />} />
      </SectionCard>

      <SectionCard title="AI Features">
        <SettingListItem
          name="AI Chat Assistant"
          description="Enable AI-powered chat support"
          iconColor={colors.purple[500]}
          icon={Bot}
          suffix={<Switch defaultChecked />}
        />
        <SettingListItem
          name="Smart Knowledge Base"
          description="Get intelligent article suggestions"
          iconColor={colors.blue[500]}
          icon={BookOpen}
          suffix={<Switch defaultChecked />}
        />
      </SectionCard>
      <Typography variant="caption" color="text.secondary" textAlign="center" sx={{ py: 1 }}>
        Version 1.0.0
      </Typography>
    </Stack>
  );
}

function SectionCard({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Stack gap={1}>
      <Typography variant="subtitle1" color="text.secondary">
        {title}
      </Typography>
      <Card component={Stack} elevation={0} divider={<Divider />}>
        {children}
      </Card>
    </Stack>
  );
}
