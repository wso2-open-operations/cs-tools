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

import { useEffect, useLayoutEffect, useState, type ReactNode } from "react";
import { Card, Divider, Skeleton, Stack, Switch, Typography, colors } from "@wso2/oxygen-ui";
import { Bell, BookOpen, Bot, Clock4, Lock, Mail, Phone, User } from "@wso2/oxygen-ui-icons-react";
import { useLayout } from "@context/layout";
import { SettingListItem } from "@components/features/settings";
import { Avatar } from "@components/features/users";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { users } from "@src/services/users";
import { useProject } from "../context/project";
import { projects } from "../services/projects";
import { useNotify } from "../context/snackbar";
import { metadata } from "../services/metadata";
import { getVersion, openUrl } from "../components/microapp-bridge";
import { CHANGE_PASSWORD_URL } from "../config/endpoints";
import { useMe } from "../context/me";

export default function ProfilePage() {
  const layout = useLayout();
  const notify = useNotify();
  const queryClient = useQueryClient();
  const { projectId, noveraEnabled, kbReferencesEnabled } = useProject();
  const { isAdmin } = useMe();
  const { data } = useQuery(users.me());
  const name = data ? data?.firstName + " " + data?.lastName : undefined;

  const projectEditMutation = useMutation({
    ...projects.edit(projectId!),
    onError: (_, variables) => {
      notify.error("Failed to update project. Please try again.");
      if (variables.hasAgent) setIsNoveraEnabled(!variables.hasAgent);
      if (variables.hasKbReferences) setIsKbReferencesEnabled(!variables.hasKbReferences);
    },
    onSuccess: () => queryClient.refetchQueries({ queryKey: projects.get(projectId!).queryKey }),
  });

  const AppBarSlot = () => (
    <Stack direction="row" alignItems="center" gap={1.5} mt={-2}>
      {name ? <Avatar>{name}</Avatar> : <Skeleton variant="circular" width={40} height={40} sx={{ flexShrink: 0 }} />}
      <Typography variant="h6" fontWeight="medium" sx={{ flex: 1 }}>
        {name ?? <Skeleton variant="text" width="100%" height={30} />}
      </Typography>
    </Stack>
  );

  const prefetch = () => {
    queryClient.prefetchQuery(metadata.get());
  };

  const [isNoveraEnabled, setIsNoveraEnabled] = useState(noveraEnabled);
  const [isKbReferencesEnabled, setIsKbReferencesEnabled] = useState(kbReferencesEnabled);
  const [version, setVersion] = useState<string | undefined>(undefined);

  useEffect(() => getVersion((version) => setVersion(version)));

  useEffect(prefetch, []);

  useLayoutEffect(() => {
    layout.setAppBarSlotsOverride(<AppBarSlot />);

    return () => {
      layout.setAppBarSlotsOverride(undefined);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data]);

  return (
    <Stack gap={2.5}>
      <SectionCard title="Account Information">
        <SettingListItem
          name="Email"
          value={data?.email ?? <Skeleton variant="text" width="100%" height={25} />}
          icon={Mail}
        />
        <SettingListItem
          name="Phone"
          value={data?.phoneNumber ?? (data ? "Not Configured" : <Skeleton variant="text" width="100%" height={25} />)}
          icon={Phone}
        />
        <SettingListItem
          name="Timezone"
          value={
            data ? (
              data.timezone === "--None--" ? (
                "Not Configured"
              ) : (
                data.timezone
              )
            ) : (
              <Skeleton variant="text" width="100%" height={25} />
            )
          }
          icon={Clock4}
        />
      </SectionCard>

      <SectionCard title="Settings">
        <SettingListItem
          name="Change Password"
          suffix="chevron"
          icon={Lock}
          onClick={() =>
            openUrl({
              url: CHANGE_PASSWORD_URL,
              presentationStyle: "FormSheet",
              dismissButtonStyle: "close",
            })
          }
        />
        <SettingListItem name="Update Profile" suffix="chevron" icon={User} to="/profile/update" />
      </SectionCard>

      <SectionCard title="Notifications">
        <SettingListItem name="Push Notifications" icon={Bell} suffix={<Switch defaultChecked />} />
      </SectionCard>

      {isAdmin && (
        <SectionCard title="AI Features">
          <SettingListItem
            name="AI Chat Assistant"
            description="Enable AI-powered chat support"
            iconColor={colors.purple[500]}
            icon={Bot}
            suffix={
              <Switch
                checked={isNoveraEnabled}
                onChange={(event) => {
                  projectEditMutation.mutate({ hasAgent: event.target.checked });
                  setIsNoveraEnabled(!isNoveraEnabled);
                }}
              />
            }
          />
          <SettingListItem
            name="Smart Knowledge Base"
            description="Get intelligent article suggestions"
            iconColor={colors.blue[500]}
            icon={BookOpen}
            suffix={
              <Switch
                checked={isKbReferencesEnabled}
                onChange={(event) => {
                  projectEditMutation.mutate({ hasKbReferences: event.target.checked });
                  setIsKbReferencesEnabled(!isKbReferencesEnabled);
                }}
              />
            }
          />
        </SectionCard>
      )}
      {version && (
        <Typography variant="caption" color="text.secondary" textAlign="center" sx={{ py: 1 }}>
          Version {version}
        </Typography>
      )}
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
