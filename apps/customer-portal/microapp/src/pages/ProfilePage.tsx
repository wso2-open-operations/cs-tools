// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { useLayoutEffect, type ReactNode } from "react";
import { Box, Button, Card, Divider, Stack, Switch, Typography, colors, pxToRem } from "@wso2/oxygen-ui";
import { Bell, BookOpen, Bot, Clock4, Lock, LogOut, Mail, Phone, User } from "@wso2/oxygen-ui-icons-react";
import { useLayout } from "@context/layout";
import { SettingListItem } from "@components/features/settings";
import { Avatar } from "@components/features/users";

export default function ProfilePage() {
  const layout = useLayout();

  const AppBarSlot = () => (
    <Stack direction="row" alignItems="center" gap={1.5} mt={1}>
      <Avatar>Lithika Damnod</Avatar>
      <Stack>
        <Typography variant="h6" fontWeight="medium">
          Lithika Damnod
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
        <SettingListItem
          name="Email"
          value="user@example.com"
          iconColor={colors.yellow[800]}
          iconBackgroundColor={colors.yellow[100]}
          icon={Mail}
        />
        <SettingListItem
          name="Phone"
          value="+1 (555) 123-4567"
          iconColor={colors.blue[800]}
          iconBackgroundColor={colors.blue[100]}
          icon={Phone}
        />
        <SettingListItem
          name="Timezone"
          value="Eastern Time (ET) - UTC-5"
          iconColor={colors.purple[800]}
          iconBackgroundColor={colors.purple[100]}
          icon={Clock4}
        />
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

      <Button
        sx={{
          direction: "row",
          alignItems: "center",
          bgcolor: "background.paper",
          color: "error.main",
          textTransform: "initial",
          fontWeight: "medium",
          gap: 1.5,
        }}
      >
        <Box color="error.main">
          <LogOut size={pxToRem(18)} />
        </Box>
        Log Out
      </Button>
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
