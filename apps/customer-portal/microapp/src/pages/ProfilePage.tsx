import { useEffect } from "react";

import { useNavigate } from "react-router-dom";

import { openUrl } from "@src/bridge";
import { useQueryClient } from "@tanstack/react-query";
import { colors, Stack, Switch, Typography } from "@wso2/oxygen-ui";
import { BookOpen, Bot, Clock4, Lock, Mail, Phone, User } from "@wso2/oxygen-ui-icons-react";

import { CHANGE_PASSWORD_URL } from "@config/endpoints";

import { metadata } from "@features/metadata/api/metadata.queries";
import { SettingsItem } from "@features/profile/components";
import { useAppVersion, useMe, useProfileMutations, useProject } from "@features/profile/hooks";

import { SectionCard } from "@shared/components/common";

export default function ProfilePage() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const version = useAppVersion();
  const { data: project } = useProject();
  const { data: me, isLoading } = useMe();
  const { edit } = useProfileMutations();

  useEffect(() => {
    queryClient.prefetchQuery(metadata.get());
  }, []);

  return (
    <Stack gap={2.5}>
      <SectionCard title="Account Information">
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
      </SectionCard>

      <SectionCard title="Settings">
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
      </SectionCard>

      <SectionCard title="AI Features">
        <SettingsItem
          label="AI Chat Assistant"
          description="Enable AI-powered chat support"
          slotProps={{ icon: { component: Bot, sx: { color: colors.purple[500] } } }}
          suffix={
            <Switch
              checked={project?.agentEnabled}
              onChange={(event) => edit.mutate({ hasAgent: event.target.checked })}
              disabled={!me.isAdmin}
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
              disabled={!me.isAdmin}
            />
          }
        />
      </SectionCard>

      {version && (
        <Typography variant="caption" color="text.secondary" textAlign="center" sx={{ py: 1 }}>
          Version {version}
        </Typography>
      )}
    </Stack>
  );
}
